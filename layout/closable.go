package layout

import (
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/widget"
)

// VisibilityNode lets containers collapse a child without allocating a new
// layout tree. Closed panels simply disappear from layout and the remaining
// siblings reclaim the space.
type VisibilityNode interface {
	Node
	Visible() bool
}

// ClosablePanel is a retained panel with an inline [x] title-bar control.
// Visibility is a plain bool because it is changed by terminal input, not a
// render-hot-path data producer. No goroutine, ticker, channel, or per-frame
// allocation is introduced.
type ClosablePanel struct {
	painter *borderPainter
	child   Node
	close   *widget.CloseButton
	visible bool
}

func ClosableRounded(title string, child Node, focusedFn func() bool, onClose func()) *ClosablePanel {
	p := &ClosablePanel{
		painter: &borderPainter{title: title, focusedFn: focusedFn, rounded: true},
		child:   child,
		visible: true,
	}
	p.close = widget.NewCloseButton(func() {
		p.visible = false
		if onClose != nil {
			onClose()
		}
	})
	return p
}

func (p *ClosablePanel) Visible() bool { return p != nil && p.visible }

func (p *ClosablePanel) Show() {
	if p != nil {
		p.visible = true
	}
}

func (p *ClosablePanel) Close() {
	if p != nil {
		p.visible = false
	}
}

func (p *ClosablePanel) Compute(area geometry.Rect) []Placement {
	return p.ComputeInto(area, nil)
}

func (p *ClosablePanel) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if !p.Visible() || area.W <= 0 || area.H <= 0 {
		return out
	}
	out = append(out, Placement{Widget: p.painter, Area: area})

	// The close control lives in the title bar. Keep three cells for "[x]"
	// when possible, without changing the panel's content inset.
	if area.W >= 5 {
		out = append(out, Placement{
			Widget: p.close,
			Area:   geometry.Rect{X: area.X + area.W - 4, Y: area.Y, W: 3, H: 1},
		})
	}

	if p.child != nil && area.W > 2 && area.H > 2 {
		inner := area.Inset(1)
		if rnode, ok := p.child.(ReusableNode); ok {
			out = rnode.ComputeInto(inner, out)
		} else {
			out = append(out, p.child.Compute(inner)...)
		}
	}
	return out
}
