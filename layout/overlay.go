package layout

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

// Overlay conditionally places a floating child above the normal tree. Later
// placements are hit-tested first by App, so the overlay naturally captures
// pointer interaction without a separate renderer.
type Overlay struct {
	Visible        func() bool
	Backdrop       widget.Widget
	Child          Node
	DismissOnClick bool
	OnDismiss      func()
}

func NewOverlay(visible func() bool, child Node) *Overlay {
	o := &Overlay{Visible: visible, Child: child}
	o.Backdrop = &dimBackdrop{owner: o}
	return o
}

// NewModal is a centered, dismissible overlay with a dimmed backdrop. It uses
// the same placement/compositor path as ordinary widgets, so there is no second
// renderer or retained terminal surface.
func NewModal(visible func() bool, child Node, width, height float64) *Overlay {
	o := NewOverlay(visible, Center(child, width, height))
	o.DismissOnClick = true
	o.OnDismiss = func() {}
	o.Backdrop = &dimBackdrop{owner: o}
	return o
}

func (o *Overlay) Compute(area geometry.Rect) []Placement { return o.ComputeInto(area, nil) }

func (o *Overlay) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if o.Visible != nil && !o.Visible() {
		return out
	}
	if o.Backdrop != nil {
		out = append(out, Placement{Widget: o.Backdrop, Area: area})
	}
	if o.Child != nil {
		if r, ok := o.Child.(ReusableNode); ok {
			return r.ComputeInto(area, out)
		}
		out = append(out, o.Child.Compute(area)...)
	}
	return out
}

// dimBackdrop preserves the underlying glyphs and dims the completed scene.
type dimBackdrop struct{ owner *Overlay }

func (d *dimBackdrop) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	_ = theme
	buf.DimRectAttr(area.X, area.Y, area.W, area.H)
}

func (d *dimBackdrop) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if d.owner == nil || !d.owner.DismissOnClick || ev.Action != input.MousePress || !area.Contains(ev.X, ev.Y) {
		return false
	}
	if d.owner.OnDismiss != nil {
		d.owner.OnDismiss()
	}
	return true
}
