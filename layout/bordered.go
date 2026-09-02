package layout

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// borderPainter is the widget.Widget that actually draws the box; it's unexported because Bordered is the only supported way to create one.
type borderPainter struct {
	title     string
	focusedFn func() bool
	rounded   bool
}

// OwnsBackground marks the panel fill as an opaque compositing layer.
func (bp *borderPainter) OwnsBackground() bool { return true }

func (bp *borderPainter) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	st := theme.Border
	if bp.focusedFn != nil && bp.focusedFn() {
		st = theme.BorderFocus
	}
	buffer.DrawBorder(buf, area.X, area.Y, area.W, area.H, bp.title, st, theme.Title, theme.Panel, bp.rounded)
}

// bordered is the Node that pairs a borderPainter with an inset child.
type bordered struct {
	painter *borderPainter
	child   Node
}

// Bordered draws a titled box around `child`'s entire computed layout, inset by one cell - the multi-widget analogue of widget.Panel. Because it is a layout.Node like any other, every focusable widget inside `child` still flattens into the app's single top-level placement list, so Tab-navigation and mouse routing work exactly as if the box weren't there. focusedFn is optional; pass a func that reports whether any child widget currently has focus to auto-highlight the border.
func Bordered(title string, child Node, focusedFn func() bool) Node {
	return &bordered{painter: &borderPainter{title: title, focusedFn: focusedFn}, child: child}
}

// BorderedRounded is Bordered with rounded corners.
func BorderedRounded(title string, child Node, focusedFn func() bool) Node {
	return &bordered{painter: &borderPainter{title: title, focusedFn: focusedFn, rounded: true}, child: child}
}

func (b *bordered) Compute(area geometry.Rect) []Placement { return b.ComputeInto(area, nil) }

func (b *bordered) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	out = append(out, Placement{Widget: b.painter, Area: area})
	if b.child != nil {
		inner := area.Inset(1)
		if rnode, ok := b.child.(ReusableNode); ok {
			out = rnode.ComputeInto(inner, out)
		} else {
			out = append(out, b.child.Compute(inner)...)
		}
	}
	return out
}
