package widget

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/clipboard"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/term"
)

// Terminal is a real integrated terminal. It owns a PTY-backed shell session
// and a VT/ANSI emulator; commands are not executed through ordinary pipes.
// The widget remains responsible only for presentation, focus and input while
// term owns process/terminal semantics.
type Terminal struct {
	FocusMixin
	mu            sync.Mutex
	session       *term.PTYSession
	emulator      *term.Emulator
	cwd           string
	shell         string
	scroll        int
	maxScrollback int
	input         []rune        // legacy compatibility; real terminal input goes directly to PTY
	viewport      [][]term.Cell // reusable visible-row snapshot storage
	damageRows    [2]int
	viewDirty     bool
	width, height int
	running       bool
	closed        bool
	generation    uint64
	notifyPending atomic.Bool
	lastStatus    string
	contextMenu   bool
	contextMenuX  int
	contextMenuY  int
	Background    *color.Color
	OnChange      func()
	onExit        func(error)
}

func NewTerminal(cwd string) *Terminal {
	if cwd == "" {
		cwd = "."
	}
	return &Terminal{cwd: cwd, maxScrollback: 10000}
}
func (t *Terminal) OwnsBackground() bool { return true }
func (t *Terminal) SetCWD(dir string) {
	if dir != "" {
		t.mu.Lock()
		t.cwd = dir
		t.mu.Unlock()
	}
}
func (t *Terminal) CWD() string           { t.mu.Lock(); defer t.mu.Unlock(); return t.cwd }
func (t *Terminal) SetShell(shell string) { t.mu.Lock(); t.shell = shell; t.mu.Unlock() }
func (t *Terminal) Shell() string         { t.mu.Lock(); defer t.mu.Unlock(); return t.shell }
func (t *Terminal) SetMaxScrollback(n int) {
	if n < 0 {
		n = 0
	}
	t.mu.Lock()
	t.maxScrollback = n
	if n == 0 {
		t.scroll = 0
	}
	if t.emulator != nil {
		t.emulator.SetMaxScrollback(n)
	}
	t.mu.Unlock()
}
func (t *Terminal) OnExit(fn func(error)) { t.mu.Lock(); t.onExit = fn; t.mu.Unlock() }

// Start creates the shell only once. The first terminal layout supplies the
// actual pane size so the shell starts with the correct number of rows/cols.
func (t *Terminal) Start() error {
	return t.start(false)
}

func (t *Terminal) start(_ bool) error {
	t.mu.Lock()
	if t.session != nil {
		t.mu.Unlock()
		return nil
	}
	if t.closed {
		t.mu.Unlock()
		return fmt.Errorf("terminal is closed")
	}
	// Reserve the generation before doing any blocking PTY work. Stop can then
	// invalidate this start without racing the newly created session into the
	// widget after it has been cancelled.
	t.generation++
	gen := t.generation
	w, h := t.width, t.height
	cwd := t.cwd
	shell := t.shell
	maxScrollback := t.maxScrollback
	t.mu.Unlock()
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	s, err := term.StartPTY(shell, cwd, w, h, nil)
	if err != nil {
		t.mu.Lock()
		valid := t.generation == gen && !t.closed
		t.mu.Unlock()
		if !valid {
			return fmt.Errorf("terminal start cancelled")
		}
		return err
	}
	em := term.NewEmulator(w, h, maxScrollback)
	t.mu.Lock()
	if t.generation != gen || t.closed || t.session != nil {
		t.mu.Unlock()
		_ = s.Close()
		_ = s.Wait()
		return fmt.Errorf("terminal start cancelled")
	}
	t.session = s
	t.emulator = em
	t.running = true
	t.mu.Unlock()
	go t.readSession(s, em, gen)
	t.notify()
	return nil
}

func (t *Terminal) readSession(s *term.PTYSession, em *term.Emulator, gen uint64) {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.Read(buf)
		if n > 0 {
			_, _ = em.Write(buf[:n])
			var reply [128]byte
			if r := em.DrainResponses(reply[:0]); len(r) > 0 {
				_, _ = s.Write(r)
			}
			t.notify()
		}
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "input/output error") || strings.Contains(err.Error(), "file already closed") {
				err = nil
			}
			_ = s.Close()
			waitErr := s.Wait()
			if err == nil && waitErr != nil {
				// A normal PTY EOF is not itself an error. Preserve a real child
				// exit status for callers that care about process failure.
				err = waitErr
			}
			t.mu.Lock()
			current := t.session == s && t.generation == gen
			if current {
				t.running = false
				t.session = nil
			}
			cb := t.onExit
			t.mu.Unlock()
			if current && cb != nil {
				cb(err)
			}
			if current {
				t.notify()
			}
			return
		}
	}
}

func (t *Terminal) notify() {
	if !t.notifyPending.CompareAndSwap(false, true) {
		return
	}
	t.mu.Lock()
	cb := t.OnChange
	t.mu.Unlock()
	if cb != nil {
		cb()
	}
	t.notifyPending.Store(false)
}

// Append is retained for application compatibility. It feeds text through the
// same emulator used by PTY output, so ANSI sequences are interpreted rather
// than displayed literally.
func (t *Terminal) Append(text string) {
	t.mu.Lock()
	em := t.emulator
	if em == nil {
		w, h := t.width, t.height
		if w < 1 {
			w = 80
		}
		if h < 1 {
			h = 24
		}
		em = term.NewEmulator(w, h, t.maxScrollback)
		t.emulator = em
	}
	t.mu.Unlock()
	_, _ = em.Write([]byte(text))
	t.notify()
}

// RunCommand sends a command to the live shell. It deliberately does not
// start a second process; the shell remains the single interactive session.
func (t *Terminal) RunCommand(command string) error {
	if err := t.Start(); err != nil {
		return err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	return t.write([]byte(command + "\n"))
}

// RunFile runs the active document through the existing shell session.
func (t *Terminal) RunFile(path string) error {
	abs, _ := filepath.Abs(path)
	t.SetCWD(filepath.Dir(abs))
	base := filepath.Base(abs)
	var command string
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".go":
		command = "go run " + shellQuote(abs)
	case ".py":
		command = "python3 " + shellQuote(abs)
	case ".js", ".mjs", ".cjs":
		command = "node " + shellQuote(abs)
	case ".ts":
		command = "npx tsx " + shellQuote(abs)
	case ".rb":
		command = "ruby " + shellQuote(abs)
	case ".sh":
		command = "sh " + shellQuote(abs)
	case ".rs":
		command = "rustc " + shellQuote(abs) + " -o /tmp/zerotui-run && /tmp/zerotui-run"
	default:
		command = "echo 'No runner configured for " + strings.ReplaceAll(base, "'", "'\\''") + "'"
	}
	return t.RunCommand(command)
}
func shellQuote(s string) string {
	// ZeroTUI targets Unix shells for the real terminal backend (Linux/macOS).
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
func (t *Terminal) write(p []byte) error {
	t.mu.Lock()
	s := t.session
	t.mu.Unlock()
	if s == nil {
		return fmt.Errorf("terminal session is not running")
	}
	_, err := s.Write(p)
	return err
}
func (t *Terminal) Stop() {
	t.mu.Lock()
	t.generation++
	s := t.session
	t.session = nil
	t.running = false
	t.mu.Unlock()
	if s != nil {
		_ = s.Close()
	}
}

// Close permanently releases the terminal session. Unlike Stop, a closed
// terminal cannot be started again; this is useful for widget teardown.
func (t *Terminal) Close() {
	t.mu.Lock()
	t.closed = true
	t.generation++
	s := t.session
	t.session = nil
	t.running = false
	t.mu.Unlock()
	if s != nil {
		_ = s.Close()
	}
}

// SetSize changes the emulator and PTY size to match the actual pane.
func (t *Terminal) SetSize(w, h int) {
	if w < 1 || h < 1 {
		return
	}
	t.mu.Lock()
	if w == t.width && h == t.height {
		t.mu.Unlock()
		return
	}
	t.width, t.height = w, h
	s := t.session
	em := t.emulator
	t.mu.Unlock()
	if em != nil {
		em.Resize(w, h)
	}
	if s != nil {
		_ = s.Resize(w, h)
	}
}

func (t *Terminal) contextMenuRect(area geometry.Rect) geometry.Rect {
	w, h := 18, 3
	if w > area.W-2 {
		w = area.W - 2
	}
	if h > area.H-2 {
		h = area.H - 2
	}
	if w < 10 {
		w = area.W
	}
	if h < 3 {
		h = area.H
	}
	x, y := t.contextMenuX, t.contextMenuY
	if x+w > area.X+area.W {
		x = area.X + area.W - w
	}
	if y+h > area.Y+area.H {
		y = area.Y + area.H - h
	}
	if x < area.X {
		x = area.X
	}
	if y < area.Y {
		y = area.Y
	}
	return geometry.Rect{X: x, Y: y, W: w, H: h}
}

func (t *Terminal) handleContextMenuMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if !t.contextMenu {
		return false
	}
	if ev.Action != input.MousePress {
		return true
	}
	menu := t.contextMenuRect(area)
	if !menu.Contains(ev.X, ev.Y) {
		t.contextMenu = false
		return true
	}
	if ev.Y != menu.Y+1 || ev.X <= menu.X || ev.X >= menu.X+menu.W-1 {
		t.contextMenu = false
		return true
	}
	t.contextMenu = false
	if text, ok := clipboard.Paste(); ok {
		_ = t.HandlePaste(text)
		t.notify()
	}
	return true
}

func (t *Terminal) drawContextMenu(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if !t.contextMenu || area.W <= 0 || area.H <= 0 {
		return
	}
	menu := t.contextMenuRect(area)
	if menu.W < 10 || menu.H < 3 {
		return
	}
	buffer.DrawBorder(buf, menu.X, menu.Y, menu.W, menu.H, "", theme.BorderFocus, theme.Title, theme.Panel, true)
	buf.FillRect(menu.X+1, menu.Y+1, menu.W-2, 1, ' ', theme.Selected)
	buf.SetString(menu.X+3, menu.Y+1, "Paste", theme.Selected.WithAttr(style.Bold))
	buf.SetString(menu.X+menu.W-8, menu.Y+1, "Ctrl+V", theme.TextMuted)
}

func (t *Terminal) HandlePaste(text string) bool {
	if text == "" {
		return false
	}
	t.mu.Lock()
	em := t.emulator
	running := t.running
	t.mu.Unlock()
	if em == nil && !running {
		if err := t.Start(); err != nil {
			t.mu.Lock()
			t.lastStatus = err.Error()
			t.mu.Unlock()
			t.notify()
			return false
		}
		t.mu.Lock()
		em = t.emulator
		t.mu.Unlock()
	}
	if em == nil {
		return false
	}
	_, _, paste, _ := em.Modes()
	if paste {
		return t.write([]byte("\x1b[200~"+text+"\x1b[201~")) == nil
	}
	return t.write([]byte(text)) == nil
}

func ctrlByte(r rune) byte {
	if r >= 'a' && r <= 'z' {
		return byte(r - 'a' + 1)
	}
	if r >= 'A' && r <= 'Z' {
		return byte(r - 'A' + 1)
	}
	switch r {
	case '[':
		return 27
	case '\\':
		return 28
	case ']':
		return 29
	case '^':
		return 30
	case '_':
		return 31
	}
	return 0
}
func keyBytes(k input.Key, appCursor bool) []byte {
	if k.Mods&input.ModCtrl != 0 && k.Type == input.KeyRune {
		if c := ctrlByte(k.Rune); c != 0 {
			return []byte{c}
		}
	}
	prefix := []byte{}
	if k.Mods&input.ModAlt != 0 {
		prefix = []byte{27}
	}
	switch k.Type {
	case input.KeyRune, input.KeySpace:
		return append(prefix, []byte(string(k.Rune))...)
	case input.KeyEnter:
		return append(prefix, '\r')
	case input.KeyEsc:
		return append(prefix, 27)
	case input.KeyTab:
		return append(prefix, '\t')
	case input.KeyShiftTab:
		return append(prefix, []byte("\x1b[Z")...)
	case input.KeyBackspace:
		return append(prefix, 127)
	case input.KeyDelete:
		return append(prefix, []byte("\x1b[3~")...)
	case input.KeyHome:
		if appCursor {
			return append(prefix, []byte("\x1bOH")...)
		}
		return append(prefix, []byte("\x1b[H")...)
	case input.KeyEnd:
		if appCursor {
			return append(prefix, []byte("\x1bOF")...)
		}
		return append(prefix, []byte("\x1b[F")...)
	case input.KeyUp:
		if appCursor {
			return append(prefix, []byte("\x1bOA")...)
		}
		return append(prefix, []byte("\x1b[A")...)
	case input.KeyDown:
		if appCursor {
			return append(prefix, []byte("\x1bOB")...)
		}
		return append(prefix, []byte("\x1b[B")...)
	case input.KeyRight:
		if appCursor {
			return append(prefix, []byte("\x1bOC")...)
		}
		return append(prefix, []byte("\x1b[C")...)
	case input.KeyLeft:
		if appCursor {
			return append(prefix, []byte("\x1bOD")...)
		}
		return append(prefix, []byte("\x1b[D")...)
	case input.KeyPageUp:
		return append(prefix, []byte("\x1b[5~")...)
	case input.KeyPageDown:
		return append(prefix, []byte("\x1b[6~")...)
	case input.KeyF2:
		return append(prefix, []byte("\x1bOQ")...)
	case input.KeyF5:
		return append(prefix, []byte("\x1b[15~")...)
	case input.KeyF6:
		return append(prefix, []byte("\x1b[17~")...)
	case input.KeyCtrlC:
		return []byte{3}
	case input.KeyCtrlP:
		return []byte{16}
	case input.KeyCtrlW:
		return []byte{23}
	}
	return nil
}
func (t *Terminal) HandleKey(k input.Key) bool {
	// In an integrated IDE terminal Ctrl+V is an explicit paste command. The
	// child PTY therefore never receives the control byte for this shortcut.
	if k.Type == input.KeyRune && k.Rune == 'v' && k.Mods&input.ModCtrl != 0 {
		if text, ok := clipboard.Paste(); ok {
			return t.HandlePaste(text)
		}
		return true
	}
	t.mu.Lock()
	em := t.emulator
	running := t.running
	t.mu.Unlock()
	if em == nil && !running {
		if err := t.Start(); err != nil {
			t.mu.Lock()
			t.lastStatus = err.Error()
			t.mu.Unlock()
			t.notify()
			return true
		}
		t.mu.Lock()
		em = t.emulator
		t.mu.Unlock()
	}
	_, _, _, appCursor := em.Modes()
	b := keyBytes(k, appCursor)
	if len(b) == 0 {
		return false
	}
	// Scrolling is a UI concern, but PageUp/PageDown should only become shell
	// input when the application is in an alternate screen.
	if k.Type == input.KeyPageUp || k.Type == input.KeyPageDown {
		_, _, _, _ = em.Modes()
	}
	if err := t.write(b); err != nil {
		t.mu.Lock()
		t.lastStatus = err.Error()
		t.mu.Unlock()
	}
	return true
}

func (t *Terminal) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if t.contextMenu && t.handleContextMenuMouse(ev, area) {
		return true
	}
	if ev.Button == input.MouseRight && ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		t.contextMenu = true
		t.contextMenuX, t.contextMenuY = ev.X, ev.Y
		t.notify()
		return true
	}
	t.mu.Lock()
	em := t.emulator
	t.mu.Unlock()
	if em == nil {
		return area.Contains(ev.X, ev.Y)
	}
	mouse, sgr, _, _ := em.Modes()
	if ev.Action == input.MouseWheelUp || ev.Action == input.MouseWheelDown {
		if mouse == 0 {
			t.mu.Lock()
			if ev.Action == input.MouseWheelUp {
				t.scroll += 3
				if max := em.ScrollbackLen(); t.scroll > max {
					t.scroll = max
				}
			} else {
				t.scroll -= 3
				if t.scroll < 0 {
					t.scroll = 0
				}
			}
			t.viewDirty = true
			t.mu.Unlock()
			t.notify()
			return true
		}
	}
	if mouse == 0 {
		return area.Contains(ev.X, ev.Y)
	}

	x := ev.X - area.X
	y := ev.Y - area.Y - 1 // terminal content starts below the widget header
	if x < 0 || y < 0 || x >= area.W || y >= area.H-1 {
		return true
	}

	button := int(ev.Button)
	if ev.Button == input.MouseNone {
		button = 3
	}
	if ev.Action == input.MouseWheelUp || ev.Action == input.MouseWheelDown {
		button = 64
		if ev.Action == input.MouseWheelDown {
			button = 65
		}
	}
	if ev.Action == input.MouseDrag {
		button |= 32
	}
	if ev.Mods&input.ModShift != 0 {
		button |= 4
	}
	if ev.Mods&input.ModAlt != 0 {
		button |= 8
	}
	if ev.Mods&input.ModCtrl != 0 {
		button |= 16
	}

	var seq [64]byte
	n := 0
	seq[n] = 0x1b
	n++
	seq[n] = '['
	n++
	if sgr {
		seq[n] = '<'
		n++
		n = appendDecimal(seq[:], n, button)
		seq[n] = ';'
		n++
		n = appendDecimal(seq[:], n, x+1)
		seq[n] = ';'
		n++
		n = appendDecimal(seq[:], n, y+1)
		if ev.Action == input.MouseRelease {
			seq[n] = 'm'
		} else {
			seq[n] = 'M'
		}
		n++
	} else {
		// X10/normal mouse reports are binary after CSI M: Cb, Cx, Cy.
		// Coordinates are limited to the legacy 223-cell range.
		if x > 222 {
			x = 222
		}
		if y > 222 {
			y = 222
		}
		if ev.Action == input.MouseRelease {
			button = 3 | (button & (4 | 8 | 16))
		}
		seq[n] = 'M'
		n++
		seq[n] = byte(32 + button)
		n++
		seq[n] = byte(33 + x)
		n++
		seq[n] = byte(33 + y)
		n++
	}
	return t.write(seq[:n]) == nil
}

func appendDecimal(dst []byte, n, v int) int {
	var tmp [20]byte
	if v == 0 {
		dst[n] = '0'
		return n + 1
	}
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	copy(dst[n:], tmp[i:])
	return n + len(tmp) - i
}

// DirtyRegions integrates terminal state with ZeroTUI's retained damage model.
// PTY output becomes emulator row damage; the widget maps that damage into its
// screen-space content area. Scrollback is a viewport transform, so changing it
// conservatively repaints the visible content band.
func (t *Terminal) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	if area.W <= 0 || area.H <= 1 || len(dst) == 0 {
		return nil
	}
	t.mu.Lock()
	em := t.emulator
	scroll := t.scroll
	viewDirty := t.viewDirty
	t.viewDirty = false
	t.mu.Unlock()
	if em == nil {
		return nil
	}
	contentH := area.H - 1
	if viewDirty || scroll > 0 {
		dst[0] = geometry.Rect{X: area.X, Y: area.Y + 1, W: area.W, H: contentH}
		return dst[:1]
	}
	var rows [2]int
	n, all := em.TakeDamage(rows[:])
	if all {
		dst[0] = geometry.Rect{X: area.X, Y: area.Y + 1, W: area.W, H: contentH}
		return dst[:1]
	}
	if n == 0 {
		return nil
	}
	top, bottom := rows[0], rows[n-1]
	if top < 0 {
		top = 0
	}
	if bottom >= contentH {
		bottom = contentH - 1
	}
	if top > bottom {
		return nil
	}
	dst[0] = geometry.Rect{X: area.X, Y: area.Y + 1 + top, W: area.W, H: bottom - top + 1}
	return dst[:1]
}

func (t *Terminal) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W <= 0 || area.H <= 0 {
		return
	}
	t.SetSize(area.W, maxInt(1, area.H-1))
	clip := buf.Clip()
	bg := theme.Panel
	if t.Background != nil {
		bg = style.Style{Bg: *t.Background, Fg: theme.Text.Fg}
	}

	fill := rectIntersectBuffer(clip, area)
	if fill.W > 0 && fill.H > 0 {
		buf.FillRect(fill.X, fill.Y, fill.W, fill.H, ' ', bg)
	}
	if area.H >= 1 && containsBuffer(clip, area.X+1, area.Y) {
		buf.SetString(area.X+1, area.Y, "TERMINAL", theme.Title.WithAttr(style.Bold))
	}
	if area.W >= 3 && containsBuffer(clip, area.X+area.W-2, area.Y) {
		buf.SetString(area.X+area.W-2, area.Y, "×", theme.TextMuted)
	}
	contentH := area.H - 1
	if contentH <= 0 {
		return
	}

	t.mu.Lock()
	em := t.emulator
	scroll := t.scroll
	if cap(t.viewport) < contentH {
		t.viewport = make([][]term.Cell, contentH)
	} else {
		t.viewport = t.viewport[:contentH]
	}
	rows := t.viewport
	t.mu.Unlock()
	if em == nil {
		return
	}
	content := geometry.Rect{X: area.X, Y: area.Y + 1, W: area.W, H: contentH}
	contentClip := rectIntersectBuffer(clip, content)
	if contentClip.W <= 0 || contentClip.H <= 0 {
		return
	}
	y0 := contentClip.Y - content.Y
	y1 := y0 + contentClip.H
	cx, cy, cur := em.CopyViewportRange(rows, contentH, scroll, y0, y1-y0)
	x0 := contentClip.X - area.X
	x1 := x0 + contentClip.W
	if x0 < 0 {
		x0 = 0
	}
	if x1 > area.W {
		x1 = area.W
	}
	for y := y0; y < y1; y++ {
		row := rows[y]
		if x1 > len(row) {
			x1 = len(row)
		}
		for x := x0; x < x1; x++ {
			c := row[x]
			st := c.Style
			if st.Fg == color.Default {
				st.Fg = theme.Text.Fg
			}
			if st.Bg == color.Default {
				st.Bg = bg.Bg
			}
			ch := c.Ch
			if ch == 0 {
				ch = ' '
			}
			buf.SetCell(area.X+x, area.Y+1+y, ch, st)
		}
	}
	if cur && CursorBlinkVisible() && cy >= 0 && cy < contentH && cx >= 0 && cx < area.W && containsGeometry(contentClip, area.X+cx, area.Y+1+cy) {
		c := buf.CellAt(area.X+cx, area.Y+1+cy)
		buf.Set(area.X+cx, area.Y+1+cy, c.Ch, c.Style.WithAttr(style.Reverse))
	}
	t.drawContextMenu(buf, area, theme)
}

func rectIntersect(a, b geometry.Rect) geometry.Rect {
	x1, y1 := a.X, a.Y
	if b.X > x1 {
		x1 = b.X
	}
	if b.Y > y1 {
		y1 = b.Y
	}
	x2, y2 := a.X+a.W, a.Y+a.H
	if b.X+b.W < x2 {
		x2 = b.X + b.W
	}
	if b.Y+b.H < y2 {
		y2 = b.Y + b.H
	}
	if x2 <= x1 || y2 <= y1 {
		return geometry.Rect{}
	}
	return geometry.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}
func rectIntersectBuffer(a buffer.Rect, b geometry.Rect) geometry.Rect {
	return rectIntersect(geometry.Rect{X: a.X, Y: a.Y, W: a.W, H: a.H}, b)
}
func containsBuffer(r buffer.Rect, x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}
func containsGeometry(r geometry.Rect, x, y int) bool { return r.Contains(x, y) }

var _ Focusable = (*Terminal)(nil)
var _ DirtyRegionProvider = (*Terminal)(nil)
