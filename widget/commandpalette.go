package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// Command is one command-palette action. Execute is called when the selected
// item is accepted. Key is an optional short hint shown at the right edge.
type Command struct {
	Name    string
	Key     string
	Execute func()
}

// CommandPalette is a compact fuzzy-filtered command launcher. Matching is
// subsequence-based and stores only indexes, so typing/searching does not
// allocate on the steady-state path.
type CommandPalette struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color // nil = theme.Panel
	FocusMixin
	Commands []Command
	Query    string
	Selected int
	OnClose  func()
	matches  []int
	lastQ    string
}

func NewCommandPalette(commands []Command) *CommandPalette {
	p := &CommandPalette{Commands: commands, matches: make([]int, 0, len(commands))}
	p.rebuildMatches()
	return p
}

func (p *CommandPalette) OwnsBackground() bool { return p.Background != nil }

func (p *CommandPalette) SetQuery(q string) { p.Query = q; p.rebuildMatches(); p.Selected = 0 }

func (p *CommandPalette) rebuildMatches() {
	p.matches = p.matches[:0]
	for i := range p.Commands {
		if fuzzyContains(p.Commands[i].Name, p.Query) {
			p.matches = append(p.matches, i)
		}
	}
	p.lastQ = p.Query
}

func fuzzyContains(s, q string) bool {
	if q == "" {
		return true
	}
	si, qi := 0, 0
	for qi < len(q) {
		if si >= len(s) {
			return false
		}
		qb := lowerASCII(q[qi])
		for si < len(s) && lowerASCII(s[si]) != qb {
			si++
		}
		if si == len(s) {
			return false
		}
		si++
		qi++
	}
	return true
}
func lowerASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func (p *CommandPalette) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if p.ThemeOverride != nil {
		theme = p.ThemeOverride
	}
	if area.W < 8 || area.H < 4 {
		return
	}
	if p.Query != p.lastQ {
		p.rebuildMatches()
	}
	fill := theme.Panel
	if p.Background != nil {
		fill = fill.WithBg(*p.Background)
	}
	buffer.DrawBorder(buf, area.X, area.Y, area.W, area.H, " COMMAND PALETTE ", theme.BorderFocus, theme.Title, fill, true)
	buf.SetString(area.X+2, area.Y+2, "> ", bgOr(theme.Text, p.Background))
	buf.SetString(area.X+4, area.Y+2, p.Query, bgOr(theme.Text, p.Background))
	buf.Set(area.X+4+len(p.Query), area.Y+2, '_', bgOr(theme.Info, p.Background))
	maxRows := area.H - 4
	for row := 0; row < maxRows; row++ {
		i := row
		y := area.Y + 4 + row
		if i >= len(p.matches) {
			break
		}
		cmd := p.Commands[p.matches[i]]
		st := bgOr(theme.Text, p.Background)
		if i == p.Selected && p.focused {
			st = bgOr(theme.Selected, p.Background)
		}
		buf.FillRect(area.X+1, y, area.W-2, 1, ' ', st)
		buf.SetString(area.X+3, y, cmd.Name, st)
		if cmd.Key != "" {
			x := area.X + area.W - len(cmd.Key) - 3
			if x > area.X+3+len(cmd.Name) {
				buf.SetString(x, y, cmd.Key, st)
			}
		}
	}
}

func (p *CommandPalette) HandleKey(k input.Key) bool {
	if k.Type == input.KeyEsc {
		if p.OnClose != nil {
			p.OnClose()
		}
		return true
	}
	if k.Type == input.KeyUp {
		if p.Selected > 0 {
			p.Selected--
		}
		return true
	}
	if k.Type == input.KeyDown {
		if p.Selected+1 < len(p.matches) {
			p.Selected++
		}
		return true
	}
	if k.Type == input.KeyBackspace {
		if len(p.Query) > 0 {
			p.Query = p.Query[:len(p.Query)-1]
			p.rebuildMatches()
			p.Selected = 0
		}
		return true
	}
	if k.Type == input.KeyEnter {
		if p.Selected < len(p.matches) {
			if fn := p.Commands[p.matches[p.Selected]].Execute; fn != nil {
				fn()
			}
		}
		return true
	}
	if k.Type == input.KeyRune && k.Rune >= 32 && k.Rune < 127 {
		if len(p.Query) < 96 {
			p.Query += string(k.Rune)
			p.rebuildMatches()
			p.Selected = 0
		}
		return true
	}
	return false
}
func (p *CommandPalette) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action != input.MousePress || !area.Contains(ev.X, ev.Y) {
		return false
	}
	row := ev.Y - area.Y - 4
	if row >= 0 && row < len(p.matches) {
		p.Selected = row
		if fn := p.Commands[p.matches[row]].Execute; fn != nil {
			fn()
		}
	}
	return true
}
