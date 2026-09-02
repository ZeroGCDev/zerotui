package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// Panel draws a titled border box around a child widget, inset by one cell. It's the standard grouping container for a dashboard section ("RISK CONTROLS", "ORDER BOOK", "OPEN POSITIONS", ...).
type Panel struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Title         string
	Child         Widget
	Focused       bool // draws BorderFocus instead of Border when true
	Rounded       bool
	Background    *color.Color // nil = theme.Panel (default); overrides the interior fill
}

func NewPanel(title string, child Widget) *Panel {
	return &Panel{Title: title, Child: child}
}

// OwnsBackground marks the panel interior as an opaque compositing layer.
func (p *Panel) OwnsBackground() bool { return true }

func (p *Panel) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if p.ThemeOverride != nil {
		theme = p.ThemeOverride
	}
	st := theme.Border
	if p.Focused {
		st = theme.BorderFocus
	}
	fill := bgOr(theme.Panel, p.Background)
	buffer.DrawBorder(buf, area.X, area.Y, area.W, area.H, p.Title, st, theme.Title, fill, p.Rounded)
	if p.Child != nil {
		p.Child.Draw(buf, area.Inset(1), theme)
	}
}
