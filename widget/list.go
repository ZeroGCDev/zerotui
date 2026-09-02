package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// List is a single-column selectable menu (order-type picker, symbol watchlist, instrument selector).
type List struct {
	FocusMixin
	Items      []string
	Selected   int
	OnSelect   func(index int)
	Background *color.Color // nil = inherit whatever's behind it (default); fills unselected rows
	scroll     int
}

func NewList(items []string) *List { return &List{Items: items} }

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (l *List) OwnsBackground() bool { return l.Background != nil }

func (l *List) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.H < 1 {
		return
	}
	if l.Selected < l.scroll {
		l.scroll = l.Selected
	}
	if l.Selected >= l.scroll+area.H {
		l.scroll = l.Selected - area.H + 1
	}
	for row := 0; row < area.H; row++ {
		idx := l.scroll + row
		if idx >= len(l.Items) {
			break
		}
		st := bgOr(theme.Text, l.Background)
		prefix := "  "
		if idx == l.Selected {
			prefix = "> "
			if l.focused {
				st = theme.Selected // intentionally ignores Background: selection must stay legible
			} else {
				st = bgOr(theme.Info, l.Background)
			}
		}
		buf.FillRect(area.X, area.Y+row, area.W, 1, ' ', st)
		buf.SetString(area.X, area.Y+row, prefix, st)
		buf.SetString(area.X+len(prefix), area.Y+row, l.Items[idx], st)
	}
}

func (l *List) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyUp:
		l.move(-1)
		return true
	case input.KeyDown:
		l.move(1)
		return true
	case input.KeyEnter:
		if l.OnSelect != nil {
			l.OnSelect(l.Selected)
		}
		return true
	case input.KeyRune:
		switch k.Rune {
		case 'k':
			l.move(-1)
			return true
		case 'j':
			l.move(1)
			return true
		}
	}
	return false
}

func (l *List) move(d int) {
	l.Selected += d
	if l.Selected < 0 {
		l.Selected = 0
	}
	if l.Selected >= len(l.Items) {
		l.Selected = len(l.Items) - 1
	}
}

func (l *List) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		idx := ev.Y - area.Y + l.scroll
		if idx >= 0 && idx < len(l.Items) {
			l.Selected = idx
			if l.OnSelect != nil {
				l.OnSelect(idx)
			}
		}
		return true
	}
	return false
}
