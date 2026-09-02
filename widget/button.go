package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// Button is a clickable/enter-activated action control.
type Button struct {
	FocusMixin
	Label      string
	OnPress    func()
	Danger     bool         // renders in the Negative theme role (e.g. STOP / KILL)
	Background *color.Color // nil = inherit whatever's behind it (default)
}

func NewButton(label string, onPress func()) *Button {
	return &Button{Label: label, OnPress: onPress}
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (b *Button) OwnsBackground() bool { return b.Background != nil }

func (b *Button) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.H <= 0 {
		return
	}
	st := theme.Text
	if b.Danger {
		st = theme.Negative
	} else {
		st = theme.Positive
	}
	if b.focused {
		st = theme.Selected
	}
	if b.Background != nil {
		st = st.WithBg(*b.Background)
	}
	buf.FillRect(area.X, area.Y, area.W, 1, ' ', st)
	buf.SetString(area.X, area.Y, "[ ", st)
	buf.SetString(area.X+2, area.Y, b.Label, st)
	buf.SetString(area.X+2+len(b.Label), area.Y, " ]", st)
}

func (b *Button) HandleKey(k input.Key) bool {
	if k.Type == input.KeyEnter || k.Type == input.KeySpace {
		if b.OnPress != nil {
			b.OnPress()
		}
		return true
	}
	return false
}

func (b *Button) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		if b.OnPress != nil {
			b.OnPress()
		}
		return true
	}
	return false
}
