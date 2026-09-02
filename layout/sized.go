package layout

import "github.com/ZeroGCDev/zerotui/geometry"

// FixedSize constrains a child to an explicit width and/or height inside the
// rectangle supplied by its parent. A non-positive dimension means "use the
// available size". The child is centered in the available rectangle.
//
// This is a layout operation, not a render-time operation: it participates in
// the existing cached/reusable layout path and allocates nothing during Draw.
func FixedSize(child Node, width, height int) Node {
	return &fixedSize{child: child, width: width, height: height}
}

type fixedSize struct {
	child  Node
	width  int
	height int
}

func (s *fixedSize) Compute(area geometry.Rect) []Placement {
	return s.ComputeInto(area, nil)
}

func (s *fixedSize) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if s == nil || s.child == nil {
		return out
	}
	w, h := area.W, area.H
	if s.width > 0 && s.width < w {
		w = s.width
	}
	if s.height > 0 && s.height < h {
		h = s.height
	}
	if w > area.W {
		w = area.W
	}
	if h > area.H {
		h = area.H
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	childArea := geometry.Rect{
		X: area.X + (area.W-w)/2,
		Y: area.Y + (area.H-h)/2,
		W: w, H: h,
	}
	if r, ok := s.child.(ReusableNode); ok {
		return r.ComputeInto(childArea, out)
	}
	return append(out, s.child.Compute(childArea)...)
}

// SizeBounds constrains a child to a minimum/maximum width and height. A zero
// maximum means unlimited. The result is centered in the available rectangle.
// All work happens during layout, so Draw remains untouched and allocation-free.
func SizeBounds(child Node, minWidth, maxWidth, minHeight, maxHeight int) Node {
	return &sizeBounds{child: child, minW: minWidth, maxW: maxWidth, minH: minHeight, maxH: maxHeight}
}

type sizeBounds struct {
	child      Node
	minW, maxW int
	minH, maxH int
}

func (s *sizeBounds) Compute(area geometry.Rect) []Placement { return s.ComputeInto(area, nil) }

func (s *sizeBounds) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if s == nil || s.child == nil || area.W <= 0 || area.H <= 0 {
		return out
	}
	w, h := area.W, area.H
	if s.maxW > 0 && w > s.maxW {
		w = s.maxW
	}
	if s.maxH > 0 && h > s.maxH {
		h = s.maxH
	}
	if s.minW > 0 && w < s.minW {
		w = s.minW
	}
	if s.minH > 0 && h < s.minH {
		h = s.minH
	}
	if w > area.W {
		w = area.W
	}
	if h > area.H {
		h = area.H
	}
	childArea := geometry.Rect{X: area.X + (area.W-w)/2, Y: area.Y + (area.H-h)/2, W: w, H: h}
	if r, ok := s.child.(ReusableNode); ok {
		return r.ComputeInto(childArea, out)
	}
	return append(out, s.child.Compute(childArea)...)
}

// Padding reserves a fixed inset around a child. It is performed by the
// layout tree, making padded components cheap during rendering and easy to
// combine with FixedSize/SizeBounds.
func Padding(child Node, left, top, right, bottom int) Node {
	return &padding{child: child, left: left, top: top, right: right, bottom: bottom}
}

type padding struct {
	child                    Node
	left, top, right, bottom int
}

func (p *padding) Compute(area geometry.Rect) []Placement { return p.ComputeInto(area, nil) }
func (p *padding) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if p == nil || p.child == nil {
		return out
	}
	x, y := area.X+p.left, area.Y+p.top
	w, h := area.W-p.left-p.right, area.H-p.top-p.bottom
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	childArea := geometry.Rect{X: x, Y: y, W: w, H: h}
	if r, ok := p.child.(ReusableNode); ok {
		return r.ComputeInto(childArea, out)
	}
	return append(out, p.child.Compute(childArea)...)
}
