package term

import (
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/style"
)

// Cell is one terminal cell after ANSI/VT processing.
type Cell struct {
	Ch    rune
	Style style.Style
}

type row struct{ cells []Cell }

// Emulator implements the terminal behavior needed by interactive shells and
// full-screen CLI applications. It keeps a visible grid, bounded scrollback,
// alternate-screen state, cursor state and common DEC/CSI modes.
type Emulator struct {
	mu                              sync.Mutex
	w, h                            int
	main, alt                       []row
	scrollback                      []row
	scrollbackHead, scrollbackCount int
	maxScrollback                   int
	useAlt                          bool
	cursorX, cursorY                int
	savedX, savedY                  int
	savedStyle                      style.Style
	mainSavedX, mainSavedY          int
	mainSavedStyle                  style.Style
	curStyle                        style.Style
	defaultFG, defaultBG            color.Color
	top, bottom                     int
	origin, wrap, cursorVisible     bool
	mouseMode                       int
	mouseSGR                        bool
	bracketedPaste                  bool
	appCursor                       bool
	parser                          parserState
	utf8Pending                     [4]byte
	utf8PendingLen                  int
	utf8PendingNeed                 int
	csi                             [64]byte
	csiLen                          int
	osc                             [4096]byte
	oscLen                          int
	dirty                           bool
	damageAll                       bool
	damageMin, damageMax            int
	tabStops                        []bool
	responses                       []byte
}

type parserState uint8

const (
	psText parserState = iota
	psEsc
	psCSI
	psOSC
	psOSCEsc
)

func NewEmulator(w, h, scrollback int) *Emulator {
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	if scrollback < 0 {
		scrollback = 0
	}
	e := &Emulator{w: w, h: h, maxScrollback: scrollback, defaultFG: color.White, defaultBG: color.Background, wrap: true, cursorVisible: true}
	e.curStyle = style.Style{Fg: e.defaultFG, Bg: e.defaultBG}
	e.resizeLocked(w, h)
	for x := 0; x < w; x++ {
		e.tabStops[x] = x%8 == 0
	}
	e.damageAll = true
	return e
}

func (e *Emulator) Resize(w, h int) {
	if w < 1 || h < 1 {
		return
	}
	e.mu.Lock()
	e.resizeLocked(w, h)
	e.mu.Unlock()
}
func (e *Emulator) resizeLocked(w, h int) {
	e.w, e.h = w, h
	e.main = resizeGrid(e.main, w, h, e.curStyle)
	e.alt = resizeGrid(e.alt, w, h, e.curStyle)
	oldTabs := e.tabStops
	e.tabStops = make([]bool, w)
	copy(e.tabStops, oldTabs)
	for x := len(oldTabs); x < w; x++ {
		e.tabStops[x] = x%8 == 0
	}
	e.top = 0
	e.bottom = h - 1
	if e.cursorX >= w {
		e.cursorX = w - 1
	}
	if e.cursorY >= h {
		e.cursorY = h - 1
	}
	e.dirty = true
	e.damageAll = true
	e.damageMin, e.damageMax = 0, h-1
}
func resizeGrid(g []row, w, h int, st style.Style) []row {
	out := make([]row, h)
	for y := 0; y < h; y++ {
		if y < len(g) {
			out[y].cells = append([]Cell(nil), g[y].cells...)
			if len(out[y].cells) > w {
				out[y].cells = out[y].cells[:w]
			}
		}
		for len(out[y].cells) < w {
			out[y].cells = append(out[y].cells, Cell{' ', st})
		}
	}
	return out
}
func (e *Emulator) grid() []row {
	if e.useAlt {
		return e.alt
	}
	return e.main
}
func (e *Emulator) setGrid(g []row) {
	if e.useAlt {
		e.alt = g
	} else {
		e.main = g
	}
}
func (e *Emulator) currentStyleLocked() style.Style { return e.curStyle }
func (e *Emulator) markRowLocked(y int) {
	if y < 0 || y >= e.h {
		return
	}
	if e.damageMin > e.damageMax {
		e.damageMin, e.damageMax = y, y
	} else {
		if y < e.damageMin {
			e.damageMin = y
		}
		if y > e.damageMax {
			e.damageMax = y
		}
	}
	e.dirty = true
}

func (e *Emulator) markRowsLocked(top, bottom int) {
	if top < 0 {
		top = 0
	}
	if bottom >= e.h {
		bottom = e.h - 1
	}
	if top > bottom || e.h == 0 {
		return
	}
	if e.damageMin > e.damageMax {
		e.damageMin, e.damageMax = top, bottom
	} else {
		if top < e.damageMin {
			e.damageMin = top
		}
		if bottom > e.damageMax {
			e.damageMax = bottom
		}
	}
	e.dirty = true
}

func (e *Emulator) markAllLocked() {
	e.damageAll = true
	e.markRowsLocked(0, e.h-1)
}

// TakeDamage consumes the emulator's pending screen damage. Rows are expressed
// in emulator coordinates and are intentionally coarse: terminal escape
// sequences such as scrolling can affect many cells at once.
func (e *Emulator) TakeDamage(dst []int) (n int, all bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.dirty {
		return 0, false
	}
	all = e.damageAll
	if all || len(dst) == 0 {
		e.dirty = false
		e.damageAll = false
		e.damageMin, e.damageMax = 1, 0
		return 0, all
	}
	for y := e.damageMin; y <= e.damageMax && n < len(dst); y++ {
		dst[n] = y
		n++
	}
	e.dirty = false
	e.damageAll = false
	e.damageMin, e.damageMax = 1, 0
	return n, false
}

func (e *Emulator) Clear() {
	e.mu.Lock()
	g := e.grid()
	for y := range g {
		for x := range g[y].cells {
			g[y].cells[x] = Cell{' ', e.curStyle}
		}
	}
	e.setGrid(g)
	e.cursorX, e.cursorY = 0, 0
	e.markAllLocked()
	e.mu.Unlock()
}

func (e *Emulator) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	minY, maxY := e.h, -1
	mark := func(y int) {
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	// Fast path for the overwhelmingly common terminal stream: ASCII text and
	// control bytes. Avoid utf8.DecodeRune on every byte while retaining a
	// bounded carry buffer for UTF-8 sequences split across PTY reads.
	for i := 0; i < len(p); {
		if e.parser == psText && e.utf8PendingLen == 0 && p[i] < utf8.RuneSelf {
			oldY := e.cursorY
			e.feedRuneLocked(rune(p[i]))
			mark(oldY)
			mark(e.cursorY)
			i++
			continue
		}

		if e.utf8PendingLen > 0 {
			need := e.utf8PendingNeed
			if need < 2 || need > len(e.utf8Pending) {
				e.utf8PendingLen = 0
				e.utf8PendingNeed = 0
				continue
			}
			// If the bytes already buffered contain a non-continuation byte,
			// the lead byte is invalid now; emit one replacement and replay the
			// remaining bytes through the normal path rather than delaying ASCII.
			invalidPrefix := false
			for j := 1; j < e.utf8PendingLen; j++ {
				if e.utf8Pending[j]&0xC0 != 0x80 {
					invalidPrefix = true
					break
				}
			}
			if invalidPrefix {
				oldY := e.cursorY
				e.feedRuneLocked(utf8.RuneError)
				mark(oldY)
				mark(e.cursorY)
				remaining := e.utf8PendingLen - 1
				copy(e.utf8Pending[:remaining], e.utf8Pending[1:e.utf8PendingLen])
				e.utf8PendingLen = remaining
				if remaining == 0 {
					e.utf8PendingNeed = 0
				} else {
					e.utf8PendingNeed = utf8SequenceLen(e.utf8Pending[0])
					if e.utf8PendingNeed == 1 {
						b := e.utf8Pending[0]
						e.utf8PendingLen = 0
						e.utf8PendingNeed = 0
						oldY = e.cursorY
						e.feedRuneLocked(rune(b))
						mark(oldY)
						mark(e.cursorY)
					}
				}
				continue
			}
			for e.utf8PendingLen < need && i < len(p) {
				e.utf8Pending[e.utf8PendingLen] = p[i]
				e.utf8PendingLen++
				i++
			}
			if e.utf8PendingLen < need {
				break
			}
			r, n := utf8.DecodeRune(e.utf8Pending[:need])
			if n == need {
				oldY := e.cursorY
				e.feedRuneLocked(r)
				mark(oldY)
				mark(e.cursorY)
				e.utf8PendingLen = 0
				e.utf8PendingNeed = 0
				continue
			}
			oldY := e.cursorY
			e.feedRuneLocked(utf8.RuneError)
			mark(oldY)
			mark(e.cursorY)
			copy(e.utf8Pending[:], e.utf8Pending[1:need])
			e.utf8PendingLen = need - 1
			e.utf8PendingNeed = utf8SequenceLen(e.utf8Pending[0])
			if e.utf8PendingNeed == 1 {
				b := e.utf8Pending[0]
				e.utf8PendingLen = 0
				e.utf8PendingNeed = 0
				oldY = e.cursorY
				e.feedRuneLocked(rune(b))
				mark(oldY)
				mark(e.cursorY)
			}
			continue
		}

		if e.parser != psText || p[i] < utf8.RuneSelf {
			oldY := e.cursorY
			e.feedRuneLocked(rune(p[i]))
			mark(oldY)
			mark(e.cursorY)
			i++
			continue
		}
		need := utf8SequenceLen(p[i])
		if need == 1 {
			oldY := e.cursorY
			e.feedRuneLocked(utf8.RuneError)
			mark(oldY)
			mark(e.cursorY)
			i++
			continue
		}
		if len(p)-i < need {
			copy(e.utf8Pending[:], p[i:])
			e.utf8PendingLen = len(p) - i
			e.utf8PendingNeed = need
			break
		}
		r, n := utf8.DecodeRune(p[i : i+need])
		if n != need {
			r = utf8.RuneError
			n = 1
		}
		oldY := e.cursorY
		e.feedRuneLocked(r)
		mark(oldY)
		mark(e.cursorY)
		i += n
	}
	if maxY >= minY {
		e.markRowsLocked(minY, maxY)
	}
	e.dirty = true
	return len(p), nil
}

func utf8SequenceLen(b byte) int {
	switch {
	case b < 0x80:
		return 1
	case b >= 0xC2 && b <= 0xDF:
		return 2
	case b >= 0xE0 && b <= 0xEF:
		return 3
	case b >= 0xF0 && b <= 0xF4:
		return 4
	default:
		return 1
	}
}

func (e *Emulator) feedRuneLocked(r rune) {
	switch e.parser {
	case psEsc:
		e.parser = psText
		switch r {
		case '[':
			e.parser = psCSI
			e.csiLen = 0
			return
		case ']':
			e.parser = psOSC
			e.oscLen = 0
			return
		case '7':
			e.savedX, e.savedY = e.cursorX, e.cursorY
			e.savedStyle = e.curStyle
			return
		case '8':
			e.cursorX, e.cursorY = e.savedX, e.savedY
			e.curStyle = e.savedStyle
			return
		case 'D':
			e.lineFeedLocked()
			return
		case 'M':
			e.reverseIndexLocked()
			return
		case 'E':
			e.cursorX = 0
			e.lineFeedLocked()
			return
		case 'H':
			if e.cursorX < len(e.tabStops) {
				e.tabStops[e.cursorX] = true
			}
			return
		case 'c':
			e.resetLocked()
			return
		case '(':
			return
		case '#':
			return
		default:
			return
		}
	case psCSI:
		if r >= 0x40 && r <= 0x7e {
			e.handleCSILocked(e.csi[:e.csiLen], r)
			e.parser = psText
			return
		}
		if r == 0x1b {
			e.parser = psEsc
			return
		}
		if e.csiLen < len(e.csi) {
			e.csi[e.csiLen] = byte(r)
			e.csiLen++
		}
		return
	case psOSC:
		if r == 0x07 {
			e.parser = psText
			return
		}
		if r == 0x1b {
			e.parser = psOSCEsc
			return
		}
		if e.oscLen < len(e.osc) {
			e.osc[e.oscLen] = byte(r)
			e.oscLen++
		}
		return
	case psOSCEsc:
		if r == '\\' || r == 0x07 {
			e.parser = psText
		} else {
			e.parser = psOSC
		}
		return
	}
	if r == 0x1b {
		e.parser = psEsc
		return
	}
	switch r {
	case '\n':
		e.lineFeedLocked()
	case '\r':
		e.cursorX = 0
	case '\b':
		if e.cursorX > 0 {
			e.cursorX--
		}
	case '\t':
		for x := e.cursorX + 1; x < e.w; x++ {
			if e.tabStops[x] {
				e.cursorX = x
				break
			}
		}
	case '\a':
	default:
		if r >= 0x20 {
			e.putLocked(r)
		}
	}
}
func (e *Emulator) resetLocked() {
	e.curStyle = style.Style{Fg: e.defaultFG, Bg: e.defaultBG}
	e.cursorX, e.cursorY = 0, 0
	e.top, e.bottom = 0, e.h-1
	e.origin = false
	e.wrap = true
	e.cursorVisible = true
	e.appCursor = false
	e.mouseMode = 0
	e.mouseSGR = false
	e.bracketedPaste = false
	for i := range e.tabStops {
		e.tabStops[i] = i%8 == 0
	}
	e.responses = e.responses[:0]
	e.useAlt = false
	e.parser = psText
	e.utf8PendingLen = 0
	e.utf8PendingNeed = 0
	// RIS (ESC c) is a real terminal reset, not merely a mode reset. Clear
	// both screen buffers and discard scrollback so a reset cannot expose stale
	// application state after returning from an alternate screen.
	for i := range e.main {
		clearRowLocked(&e.main[i], e.w, e.curStyle)
	}
	for i := range e.alt {
		clearRowLocked(&e.alt[i], e.w, e.curStyle)
	}
	e.scrollback = nil
	e.scrollbackHead = 0
	e.scrollbackCount = 0
	e.markAllLocked()
}
func (e *Emulator) putLocked(r rune) {
	w := terminalRuneWidth(r)
	if w == 0 {
		// The cell model has no grapheme-composition storage. Keep combining
		// marks from corrupting the cursor stream; a future grapheme layer can
		// attach them to the preceding cell without changing terminal layout.
		return
	}
	if e.cursorX >= e.w || (w == 2 && e.cursorX == e.w-1) {
		if e.wrap {
			e.cursorX = 0
			e.lineFeedLocked()
		} else {
			e.cursorX = e.w - 1
			if w == 2 {
				return
			}
		}
	}
	g := e.grid()
	// Writing over either half of a previous wide glyph must remove the
	// orphaned half before placing the new glyph.
	if e.cursorX > 0 && g[e.cursorY].cells[e.cursorX].Ch == 0 && terminalRuneWidth(g[e.cursorY].cells[e.cursorX-1].Ch) == 2 {
		g[e.cursorY].cells[e.cursorX-1] = Cell{' ', e.curStyle}
	}
	if terminalRuneWidth(g[e.cursorY].cells[e.cursorX].Ch) == 2 && e.cursorX+1 < e.w {
		g[e.cursorY].cells[e.cursorX+1] = Cell{' ', e.curStyle}
	}
	g[e.cursorY].cells[e.cursorX] = Cell{r, e.curStyle}
	if w == 2 && e.cursorX+1 < e.w {
		g[e.cursorY].cells[e.cursorX+1] = Cell{0, e.curStyle}
	}
	e.setGrid(g)
	e.cursorX += w
	if e.cursorX >= e.w && e.wrap {
		e.cursorX = e.w
	}
}

func terminalRuneWidth(r rune) int {
	if r < 0x80 {
		if r < 0x20 || r == 0x7f {
			return 0
		}
		return 1
	}
	if unicode.IsControl(r) {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		r >= 0x2e80 && r <= 0x303e || r >= 0x3040 && r <= 0xa4cf ||
		r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff ||
		r >= 0xfe10 && r <= 0xfe19 || r >= 0xfe30 && r <= 0xfe6f ||
		r >= 0xff00 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 ||
		r >= 0x1f000 && r <= 0x1faff || r >= 0x20000 && r <= 0x3fffd) {
		return 2
	}
	return 1
}
func (e *Emulator) lineFeedLocked() {
	if e.cursorY < e.bottom {
		e.cursorY++
		return
	}
	e.scrollUpLocked(1)
}
func (e *Emulator) reverseIndexLocked() {
	if e.cursorY > e.top {
		e.cursorY--
		return
	}
	e.scrollDownLocked(1)
}
func (e *Emulator) pushScrollbackLocked(r row) {
	if e.maxScrollback <= 0 || len(r.cells) == 0 {
		return
	}
	if len(e.scrollback) < e.maxScrollback {
		cells := make([]Cell, len(r.cells))
		copy(cells, r.cells)
		e.scrollback = append(e.scrollback, row{cells: cells})
		e.scrollbackCount = len(e.scrollback)
		return
	}
	// The scrollback ring owns its storage. Never alias a live screen row here:
	// screen rows are deliberately recycled after a scroll to keep the PTY hot
	// path allocation-free once the ring has reached capacity.
	dst := &e.scrollback[e.scrollbackHead]
	if cap(dst.cells) < len(r.cells) {
		dst.cells = make([]Cell, len(r.cells))
	} else {
		dst.cells = dst.cells[:len(r.cells)]
	}
	copy(dst.cells, r.cells)
	e.scrollbackHead++
	if e.scrollbackHead == len(e.scrollback) {
		e.scrollbackHead = 0
	}
	e.scrollbackCount = len(e.scrollback)
}

func clearRowLocked(r *row, w int, st style.Style) {
	if cap(r.cells) < w {
		r.cells = make([]Cell, w)
	} else {
		r.cells = r.cells[:w]
	}
	fillCellsLocked(r.cells, Cell{' ', st})
}

func fillCellsLocked(dst []Cell, c Cell) {
	if len(dst) == 0 {
		return
	}
	dst[0] = c
	n := 1
	for n < len(dst) {
		copyN := n
		if copyN > len(dst)-n {
			copyN = len(dst) - n
		}
		copy(dst[n:n+copyN], dst[:copyN])
		n += copyN
	}
}

func (e *Emulator) scrollUpLocked(n int) {
	if n < 1 {
		return
	}
	g := e.grid()
	if !e.useAlt && e.top == 0 {
		for i := 0; i < n && len(g) > 0; i++ {
			e.pushScrollbackLocked(g[e.top])
			first := g[e.top]
			copy(g[e.top:e.bottom], g[e.top+1:e.bottom+1])
			g[e.bottom] = first
			clearRowLocked(&g[e.bottom], e.w, e.curStyle)
		}
	} else {
		for i := 0; i < n; i++ {
			first := g[e.top]
			copy(g[e.top:e.bottom], g[e.top+1:e.bottom+1])
			g[e.bottom] = first
			clearRowLocked(&g[e.bottom], e.w, e.curStyle)
		}
	}
	e.setGrid(g)
	e.markRowsLocked(e.top, e.bottom)
}

func (e *Emulator) scrollDownLocked(n int) {
	g := e.grid()
	for i := 0; i < n; i++ {
		last := g[e.bottom]
		copy(g[e.top+1:e.bottom+1], g[e.top:e.bottom])
		g[e.top] = last
		clearRowLocked(&g[e.top], e.w, e.curStyle)
	}
	e.setGrid(g)
	e.markRowsLocked(e.top, e.bottom)
}

func paramsBytes(s []byte, out *[16]int) (priv bool, n int) {
	if len(s) > 0 && s[0] == '?' {
		priv = true
		s = s[1:]
	}
	if len(s) == 0 {
		out[0] = 0
		return priv, 1
	}
	value := 0
	have := false
	for _, c := range s {
		if c == ';' {
			if n < len(out) {
				out[n] = value
				n++
			}
			value, have = 0, false
			continue
		}
		if c >= '0' && c <= '9' {
			value = value*10 + int(c-'0')
			have = true
		}
	}
	if n < len(out) {
		if !have {
			value = 0
		}
		out[n] = value
		n++
	}
	return priv, n
}
func first(p []int, d int) int {
	if len(p) == 0 || p[0] == 0 {
		return d
	}
	return p[0]
}
func (e *Emulator) handleCSILocked(s []byte, final rune) {
	var paramsBuf [16]int
	priv, pn := paramsBytes(s, &paramsBuf)
	p := paramsBuf[:pn]
	n := first(p, 1)
	switch final {
	case 'A':
		minY, _ := e.cursorBoundsLocked()
		e.cursorY -= n
		if e.cursorY < minY {
			e.cursorY = minY
		}
	case 'B', 'e':
		_, maxY := e.cursorBoundsLocked()
		e.cursorY += n
		if e.cursorY > maxY {
			e.cursorY = maxY
		}
	case 'C', 'a':
		e.cursorX += n
		if e.cursorX >= e.w {
			e.cursorX = e.w - 1
		}
	case 'D':
		e.cursorX -= n
		if e.cursorX < 0 {
			e.cursorX = 0
		}
	case 'E':
		_, maxY := e.cursorBoundsLocked()
		e.cursorY += n
		if e.cursorY > maxY {
			e.cursorY = maxY
		}
		e.cursorX = 0
	case 'F':
		minY, _ := e.cursorBoundsLocked()
		e.cursorY -= n
		if e.cursorY < minY {
			e.cursorY = minY
		}
		e.cursorX = 0
	case 'G', '`':
		e.cursorX = first(p, 1) - 1
		if e.cursorX < 0 {
			e.cursorX = 0
		}
		if e.cursorX >= e.w {
			e.cursorX = e.w - 1
		}
	case 'd':
		baseY, maxY := e.cursorBoundsLocked()
		e.cursorY = baseY + first(p, 1) - 1
		if e.cursorY > maxY {
			e.cursorY = maxY
		}
	case 'H', 'f':
		y := first(p, 1) - 1
		x := 1
		if len(p) > 1 {
			x = p[1]
		}
		baseY := 0
		limitY := e.h - 1
		if e.origin {
			baseY = e.top
			limitY = e.bottom
		}
		e.cursorY = baseY + y
		if e.cursorY > limitY {
			e.cursorY = limitY
		}
		e.cursorX = x - 1
		if e.cursorX < 0 {
			e.cursorX = 0
		}
		if e.cursorX >= e.w {
			e.cursorX = e.w - 1
		}
	case 'J':
		e.eraseDisplayLocked(first(p, 0))
	case 'K':
		e.eraseLineLocked(first(p, 0))
	case 'P':
		e.deleteCharsLocked(n)
	case '@':
		e.insertCharsLocked(n)
	case 'X':
		e.eraseCharsLocked(n)
	case 'L':
		e.insertLinesLocked(n)
	case 'M':
		e.deleteLinesLocked(n)
	case 'S':
		e.scrollUpLocked(n)
	case 'T':
		e.scrollDownLocked(n)
	case 'm':
		e.sgrLocked(p)
	case 'r':
		e.setScrollRegionLocked(p)
	case 'h', 'l':
		e.setModeLocked(priv, final, p)
	case 's':
		e.savedX, e.savedY = e.cursorX, e.cursorY
	case 'u':
		e.cursorX, e.cursorY = e.savedX, e.savedY
	case 'g':
		mode := first(p, 0)
		if mode == 3 {
			for i := range e.tabStops {
				e.tabStops[i] = false
			}
		} else if mode == 0 && e.cursorX < len(e.tabStops) {
			e.tabStops[e.cursorX] = false
		}
	case 'c':
		e.responses = append(e.responses, []byte("\x1b[?62;1;2;6;9;15;18;22c")...)
	case 'n':
		if first(p, 0) == 5 {
			e.responses = append(e.responses, []byte("\x1b[0n")...)
		} else if first(p, 0) == 6 {
			e.responses = append(e.responses, []byte("\x1b[")...)
			var b [24]byte
			n1 := appendDecimalLocal(b[:], e.cursorY+1)
			e.responses = append(e.responses, b[:n1]...)
			e.responses = append(e.responses, ';')
			n1 = appendDecimalLocal(b[:], e.cursorX+1)
			e.responses = append(e.responses, b[:n1]...)
			e.responses = append(e.responses, 'R')
		}
	}
}
func appendDecimalLocal(dst []byte, v int) int {
	if v == 0 {
		dst[0] = '0'
		return 1
	}
	i := len(dst)
	for v > 0 {
		i--
		dst[i] = byte('0' + v%10)
		v /= 10
	}
	n := len(dst) - i
	copy(dst, dst[i:])
	return n
}
func (e *Emulator) DrainResponses(dst []byte) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.responses) == 0 {
		return dst
	}
	dst = append(dst, e.responses...)
	e.responses = e.responses[:0]
	return dst
}
func (e *Emulator) eraseDisplayLocked(mode int) {
	g := e.grid()
	blank := Cell{' ', e.curStyle}
	switch mode {
	case 2, 3:
		for y := range g {
			for x := range g[y].cells {
				g[y].cells[x] = blank
			}
		}
	case 1:
		for y := 0; y <= e.cursorY; y++ {
			end := e.w
			if y == e.cursorY {
				end = e.cursorX + 1
			}
			for x := 0; x < end; x++ {
				g[y].cells[x] = blank
			}
		}
	default:
		for y := e.cursorY; y < e.h; y++ {
			start := 0
			if y == e.cursorY {
				start = e.cursorX
			}
			for x := start; x < e.w; x++ {
				g[y].cells[x] = blank
			}
		}
	}
	e.setGrid(g)
	e.markAllLocked()
}
func (e *Emulator) eraseLineLocked(mode int) {
	g := e.grid()
	start, end := 0, e.w
	switch mode {
	case 1:
		end = e.cursorX + 1
	case 0:
		start = e.cursorX
	}
	for x := start; x < end; x++ {
		g[e.cursorY].cells[x] = Cell{' ', e.curStyle}
	}
	e.setGrid(g)
}
func (e *Emulator) eraseCharsLocked(n int) {
	g := e.grid()
	for x := e.cursorX; x < e.cursorX+n && x < e.w; x++ {
		g[e.cursorY].cells[x] = Cell{' ', e.curStyle}
	}
	e.setGrid(g)
}
func (e *Emulator) deleteCharsLocked(n int) {
	g := e.grid()
	if n > e.w-e.cursorX {
		n = e.w - e.cursorX
	}
	copy(g[e.cursorY].cells[e.cursorX:], g[e.cursorY].cells[e.cursorX+n:])
	for x := e.w - n; x < e.w; x++ {
		g[e.cursorY].cells[x] = Cell{' ', e.curStyle}
	}
	e.setGrid(g)
}
func (e *Emulator) insertCharsLocked(n int) {
	g := e.grid()
	if n > e.w-e.cursorX {
		n = e.w - e.cursorX
	}
	copy(g[e.cursorY].cells[e.cursorX+n:], g[e.cursorY].cells[e.cursorX:e.w-n])
	for x := e.cursorX; x < e.cursorX+n; x++ {
		g[e.cursorY].cells[x] = Cell{' ', e.curStyle}
	}
	e.setGrid(g)
}
func (e *Emulator) insertLinesLocked(n int) {
	if e.cursorY < e.top || e.cursorY > e.bottom {
		return
	}
	g := e.grid()
	if n > e.bottom-e.cursorY+1 {
		n = e.bottom - e.cursorY + 1
	}
	copy(g[e.cursorY+n:e.bottom+1], g[e.cursorY:e.bottom-n+1])
	for y := e.bottom - n + 1; y <= e.bottom; y++ {
		clearRowLocked(&g[y], e.w, e.curStyle)
	}
	e.setGrid(g)
	e.markRowsLocked(e.cursorY, e.bottom)
}
func (e *Emulator) deleteLinesLocked(n int) {
	if e.cursorY < e.top || e.cursorY > e.bottom {
		return
	}
	g := e.grid()
	if n > e.bottom-e.cursorY+1 {
		n = e.bottom - e.cursorY + 1
	}
	copy(g[e.cursorY:e.bottom-n+1], g[e.cursorY+n:e.bottom+1])
	for y := e.bottom - n + 1; y <= e.bottom; y++ {
		clearRowLocked(&g[y], e.w, e.curStyle)
	}
	e.setGrid(g)
	e.markRowsLocked(e.cursorY, e.bottom)
}
func (e *Emulator) setScrollRegionLocked(p []int) {
	top := 1
	bot := e.h
	if len(p) > 0 && p[0] > 0 {
		top = p[0]
	}
	if len(p) > 1 && p[1] > 0 {
		bot = p[1]
	}
	if top < 1 {
		top = 1
	}
	if bot > e.h {
		bot = e.h
	}
	if top < bot {
		e.top, e.bottom = top-1, bot-1
		e.cursorX = 0
		if e.origin {
			e.cursorY = e.top
		} else {
			e.cursorY = 0
		}
	}
}

func (e *Emulator) cursorBoundsLocked() (minY, maxY int) {
	if e.origin {
		return e.top, e.bottom
	}
	return 0, e.h - 1
}
func (e *Emulator) prepareAltLocked() {
	if len(e.alt) != e.h {
		e.alt = resizeGrid(e.alt, e.w, e.h, e.curStyle)
		return
	}
	for y := range e.alt {
		clearRowLocked(&e.alt[y], e.w, e.curStyle)
	}
}

func (e *Emulator) setModeLocked(priv bool, final rune, p []int) {
	on := final == 'h'
	for _, m := range p {
		if priv {
			switch m {
			case 1:
				e.appCursor = on
			case 6:
				e.origin = on
				e.cursorX = 0
				if on {
					e.cursorY = e.top
				} else {
					e.cursorY = 0
				}
			case 7:
				e.wrap = on
			case 25:
				e.cursorVisible = on
			case 47:
				if on && !e.useAlt {
					e.mainSavedX, e.mainSavedY = e.cursorX, e.cursorY
					e.mainSavedStyle = e.curStyle
					e.useAlt = true
					e.cursorX, e.cursorY = 0, 0
				} else if !on && e.useAlt {
					e.useAlt = false
					e.cursorX, e.cursorY = e.mainSavedX, e.mainSavedY
					e.curStyle = e.mainSavedStyle
				}
			case 1047:
				if on && !e.useAlt {
					e.mainSavedX, e.mainSavedY = e.cursorX, e.cursorY
					e.mainSavedStyle = e.curStyle
					e.prepareAltLocked()
					e.useAlt = true
					e.cursorX, e.cursorY = 0, 0
				} else if !on && e.useAlt {
					e.useAlt = false
					e.cursorX, e.cursorY = e.mainSavedX, e.mainSavedY
					e.curStyle = e.mainSavedStyle
				}
			case 1048:
				if on {
					e.savedX, e.savedY = e.cursorX, e.cursorY
					e.savedStyle = e.curStyle
				} else {
					e.cursorX, e.cursorY = e.savedX, e.savedY
					e.curStyle = e.savedStyle
				}
			case 1049:
				if on {
					e.savedX, e.savedY = e.cursorX, e.cursorY
					e.savedStyle = e.curStyle
					e.prepareAltLocked()
					e.useAlt = true
					e.cursorX, e.cursorY = 0, 0
				} else {
					e.useAlt = false
					e.cursorX, e.cursorY = e.savedX, e.savedY
					e.curStyle = e.savedStyle
				}
			case 1000, 1002, 1003:
				if on {
					e.mouseMode = m
				} else if e.mouseMode == m {
					e.mouseMode = 0
				}
			case 1006:
				e.mouseSGR = on
			case 2004:
				e.bracketedPaste = on
			}
		} else if m == 4 {
		}
	}
	e.markAllLocked()
}
func (e *Emulator) sgrLocked(p []int) {
	if len(p) == 0 {
		p = []int{0}
	}
	for i := 0; i < len(p); i++ {
		v := p[i]
		switch {
		case v == 0:
			e.curStyle = style.Style{Fg: e.defaultFG, Bg: e.defaultBG}
		case v == 1:
			e.curStyle.Attr |= style.Bold
		case v == 2:
			e.curStyle.Attr |= style.Dim
		case v == 3:
			e.curStyle.Attr |= style.Italic
		case v == 4:
			e.curStyle.Attr |= style.Underline
		case v == 5:
			e.curStyle.Attr |= style.Blink
		case v == 7:
			e.curStyle.Attr |= style.Reverse
		case v == 22:
			e.curStyle.Attr &^= style.Bold | style.Dim
		case v == 23:
			e.curStyle.Attr &^= style.Italic
		case v == 24:
			e.curStyle.Attr &^= style.Underline
		case v == 25:
			e.curStyle.Attr &^= style.Blink
		case v == 27:
			e.curStyle.Attr &^= style.Reverse
		case v >= 30 && v <= 37:
			e.curStyle.Fg = ansiColor(v - 30)
		case v >= 90 && v <= 97:
			e.curStyle.Fg = ansiBright(v - 90)
		case v == 39:
			e.curStyle.Fg = e.defaultFG
		case v >= 40 && v <= 47:
			e.curStyle.Bg = ansiColor(v - 40)
		case v >= 100 && v <= 107:
			e.curStyle.Bg = ansiBright(v - 100)
		case v == 49:
			e.curStyle.Bg = e.defaultBG
		case v == 38 || v == 48:
			if i+1 < len(p) && p[i+1] == 5 && i+2 < len(p) {
				c := ansi256(p[i+2])
				if v == 38 {
					e.curStyle.Fg = c
				} else {
					e.curStyle.Bg = c
				}
				i += 2
			} else if i+1 < len(p) && p[i+1] == 2 && i+4 < len(p) {
				c := color.RGB(uint8(p[i+2]), uint8(p[i+3]), uint8(p[i+4]))
				if v == 38 {
					e.curStyle.Fg = c
				} else {
					e.curStyle.Bg = c
				}
				i += 4
			}
		}
	}
}

var ansiColors = [...]color.Color{color.Black, color.Red, color.Green, color.Yellow, color.Blue, color.Magenta, color.Cyan, color.White}
var ansiBrightColors = [...]color.Color{color.Gray, color.Red, color.Green, color.Yellow, color.Blue, color.Magenta, color.Cyan, color.White}

func ansiColor(i int) color.Color  { return ansiColors[i&7] }
func ansiBright(i int) color.Color { return ansiBrightColors[i&7] }
func ansi256(n int) color.Color {
	if n < 16 {
		if n < 8 {
			return ansiColor(n)
		}
		return ansiBright(n - 8)
	}
	if n >= 232 {
		v := uint8(8 + (n-232)*10)
		return color.RGB(v, v, v)
	}
	n -= 16
	r := n / 36
	g := (n / 6) % 6
	b := n % 6
	cv := func(x int) uint8 {
		if x == 0 {
			return 0
		}
		return uint8(55 + x*40)
	}
	return color.RGB(cv(r), cv(g), cv(b))
}

// Snapshot returns a copy of the visible grid and cursor state for rendering.
func (e *Emulator) Snapshot() ([][]Cell, int, int, bool, bool, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	g := e.grid()
	out := make([][]Cell, len(g))
	for y := range g {
		out[y] = append([]Cell(nil), g[y].cells...)
	}
	return out, e.cursorX, e.cursorY, e.cursorVisible, e.useAlt, e.mouseMode
}

// CopyViewport copies only the rows visible in the requested viewport into dst.
// The caller owns dst and may reuse it between frames. This avoids building a
// temporary concatenated scrollback+screen slice and allocating one cell slice
// per visible row on every terminal redraw.
func (e *Emulator) CopyViewport(dst [][]Cell, height, scroll int) (int, int, bool) {
	return e.CopyViewportRange(dst, height, scroll, 0, height)
}

// CopyViewportRange copies only a contiguous range of visible viewport rows
// into dst. The destination is indexed by viewport row, so callers can reuse a
// stable row cache while repainting a small damage band. It deliberately shares
// the same coordinate and scrollback semantics as CopyViewport.
func (e *Emulator) CopyViewportRange(dst [][]Cell, height, scroll, firstRow, rowCount int) (int, int, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if height < 1 || e.w < 1 || rowCount <= 0 {
		return 0, 0, false
	}
	if firstRow < 0 {
		rowCount += firstRow
		firstRow = 0
	}
	if firstRow >= height || rowCount <= 0 {
		return 0, 0, false
	}
	if rowCount > height-firstRow {
		rowCount = height - firstRow
	}
	if len(dst) < height {
		// This is an API boundary rather than the normal widget path. The terminal
		// widget always provisions its reusable row cache before calling us.
		dst = append(dst, make([][]Cell, height-len(dst))...)
	}

	screen := e.grid()
	scrollbackLen := 0
	if !e.useAlt {
		scrollbackLen = e.scrollbackCount
	}
	total := scrollbackLen + len(screen)
	maxScroll := total - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	start := total - height - scroll + firstRow
	for i := 0; i < rowCount; i++ {
		rowIndex := firstRow + i
		idx := start + i
		if cap(dst[rowIndex]) < e.w {
			dst[rowIndex] = make([]Cell, e.w)
		} else {
			dst[rowIndex] = dst[rowIndex][:e.w]
		}
		var src []Cell
		if idx >= 0 && idx < total {
			if idx < scrollbackLen {
				physical := e.scrollbackHead + idx
				if physical >= len(e.scrollback) {
					physical -= len(e.scrollback)
				}
				src = e.scrollback[physical].cells
			} else {
				src = screen[idx-scrollbackLen].cells
			}
		}
		if len(src) >= e.w {
			copy(dst[rowIndex], src[:e.w])
		} else {
			copy(dst[rowIndex], src)
			fillCellsLocked(dst[rowIndex][len(src):], Cell{' ', e.curStyle})
		}
	}
	cx, cy := e.cursorX, e.cursorY
	visibleCursor := e.cursorVisible && scroll == 0
	return cx, cy, visibleCursor
}

// Viewport is retained for callers that want an owned snapshot. Render paths
// should prefer CopyViewport so the backing storage can be reused.
func (e *Emulator) Viewport(height, scroll int) ([][]Cell, int, int, bool) {
	if height < 1 {
		return nil, 0, 0, false
	}
	out := make([][]Cell, height)
	cx, cy, cur := e.CopyViewport(out, height, scroll)
	return out, cx, cy, cur
}

func (e *Emulator) Modes() (mouse int, sgr, paste, appCursor bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.mouseMode, e.mouseSGR, e.bracketedPaste, e.appCursor
}

// SetMaxScrollback changes the bounded scrollback capacity. This is a
// configuration-time operation; the hot output path only mutates the existing
// ring and does not resize it.
func (e *Emulator) SetMaxScrollback(n int) {
	if n < 0 {
		n = 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if n == e.maxScrollback {
		return
	}
	if n == 0 {
		e.scrollback = nil
		e.scrollbackHead = 0
		e.scrollbackCount = 0
		e.maxScrollback = 0
		e.markAllLocked()
		return
	}
	count := e.scrollbackCount
	if count > n {
		count = n
	}
	out := make([]row, count)
	for i := 0; i < count; i++ {
		physical := e.scrollbackHead + e.scrollbackCount - count + i
		physical %= e.scrollbackCount
		out[i].cells = append(out[i].cells, e.scrollback[physical].cells...)
	}
	e.scrollback = out
	e.scrollbackHead = 0
	e.scrollbackCount = count
	e.maxScrollback = n
	e.markAllLocked()
}

func (e *Emulator) ScrollbackLen() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.scrollbackCount
}
func (e *Emulator) ScrollbackRow(i int) []Cell {
	e.mu.Lock()
	defer e.mu.Unlock()
	if i < 0 || i >= e.scrollbackCount {
		return nil
	}
	physical := e.scrollbackHead + i
	if physical >= len(e.scrollback) {
		physical -= len(e.scrollback)
	}
	return append([]Cell(nil), e.scrollback[physical].cells...)
}
