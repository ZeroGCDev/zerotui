package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// Divider draws a light structural divider. A dedicated primitive avoids
// forcing users to create a one-off label containing box-drawing characters.
type Divider struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color
	Horizontal    bool
}

func (d *Divider) OwnsBackground() bool { return d.Background != nil }

func NewDivider(horizontal bool) *Divider { return &Divider{Horizontal: horizontal} }

func (d *Divider) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if d.ThemeOverride != nil {
		theme = d.ThemeOverride
	}
	st := bgOr(theme.Border, d.Background)
	if d.Horizontal {
		buf.FillRect(area.X, area.Y, area.W, 1, '─', st)
		return
	}
	buf.FillRect(area.X, area.Y, 1, area.H, '│', st)
}
