package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// CloseButton is a tiny pointer-only control intended for panel title bars.
// It is deliberately not focusable, so adding close controls does not expand
// the keyboard focus ring or create per-frame work.
type CloseButton struct {
	OnClose func()
}

func NewCloseButton(onClose func()) *CloseButton {
	return &CloseButton{OnClose: onClose}
}

func (b *CloseButton) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 1 || area.H < 1 {
		return
	}
	st := theme.Border
	if area.W >= 3 {
		buf.SetString(area.X, area.Y, "[x]", st)
	} else {
		buf.Set(area.X, area.Y, 'x', st)
	}
}

func (b *CloseButton) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		if b.OnClose != nil {
			b.OnClose()
		}
		return true
	}
	return false
}
