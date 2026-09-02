package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// CloseButton is a tiny pointer-only control intended for panel title bars.
// It is deliberately not focusable, so adding close controls does not expand
// the keyboard focus ring or create per-frame work.
type CloseButton struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color
	OnClose       func()
}

func NewCloseButton(onClose func()) *CloseButton {
	return &CloseButton{OnClose: onClose}
}

func (b *CloseButton) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if b.ThemeOverride != nil {
		theme = b.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	st := bgOr(theme.Title, b.Background).WithBg(theme.Panel.Bg).WithAttr(style.Bold)
	if area.W >= 3 {
		buf.FillRect(area.X, area.Y, 3, 1, ' ', st)
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
