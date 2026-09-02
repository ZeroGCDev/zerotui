package layout

import (
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/widget"
)

// Split is a two-pane responsive container with an optional mouse-draggable
// divider. Ratio is the fraction given to the first pane. Geometry is
// recomputed only on resize or divider drag, never every render frame.
type Split struct {
	Dir       Direction
	First     Node
	Second    Node
	Ratio     float64
	Gap       int
	MinFirst  int
	MinSecond int
	Handle    *widget.ResizeHandle
	area      geometry.Rect
}

func NewSplit(dir Direction, first, second Node, ratio float64) *Split {
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	s := &Split{
		Dir: dir, First: first, Second: second, Ratio: ratio,
		Gap: 1, MinFirst: 10, MinSecond: 10,
	}
	rd := widget.ResizeVertical
	if dir == Vertical {
		rd = widget.ResizeHorizontal
	}
	s.Handle = widget.NewResizeHandle(rd, &s.Ratio)
	s.Handle.OnResize = func(r float64) { s.Ratio = r }
	s.Handle.OnDrag = func(ev input.MouseEvent, _ geometry.Rect) { s.SetRatioFromPoint(s.area, ev) }
	return s
}

func (s *Split) Visible() bool {
	return s != nil && isVisible(s.First) || s != nil && isVisible(s.Second)
}

func (s *Split) Compute(area geometry.Rect) []Placement { return s.ComputeInto(area, nil) }

func (s *Split) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	s.area = area

	firstVisible := s.First != nil && isVisible(s.First)
	secondVisible := s.Second != nil && isVisible(s.Second)

	// A closed pane collapses completely and the other pane takes the whole
	// rectangle. This also removes the divider, so there is no dead gap.
	if !firstVisible && !secondVisible {
		return out
	}
	if !firstVisible {
		if rnode, ok := s.Second.(ReusableNode); ok {
			return rnode.ComputeInto(area, out)
		}
		return append(out, s.Second.Compute(area)...)
	}
	if !secondVisible {
		if rnode, ok := s.First.(ReusableNode); ok {
			return rnode.ComputeInto(area, out)
		}
		return append(out, s.First.Compute(area)...)
	}

	main := area.W
	if s.Dir == Vertical {
		main = area.H
	}
	gap := s.Gap
	if gap < 0 {
		gap = 0
	}
	avail := main - gap
	if avail < 1 {
		avail = 1
	}
	ratio := s.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	first := int(float64(avail) * ratio)
	if first < s.MinFirst {
		first = s.MinFirst
	}
	if avail-first < s.MinSecond {
		first = avail - s.MinSecond
	}
	if first < 0 {
		first = 0
	}
	second := avail - first
	s.Ratio = float64(first) / float64(avail)

	if s.Dir == Horizontal {
		a := geometry.Rect{X: area.X, Y: area.Y, W: first, H: area.H}
		h := geometry.Rect{X: area.X + first, Y: area.Y, W: gap, H: area.H}
		if rnode, ok := s.First.(ReusableNode); ok {
			out = rnode.ComputeInto(a, out)
		} else {
			out = append(out, s.First.Compute(a)...)
		}
		if gap > 0 {
			out = append(out, Placement{Widget: s.Handle, Area: h})
		}
		secondArea := geometry.Rect{X: area.X + first + gap, Y: area.Y, W: second, H: area.H}
		if rnode, ok := s.Second.(ReusableNode); ok {
			out = rnode.ComputeInto(secondArea, out)
		} else {
			out = append(out, s.Second.Compute(secondArea)...)
		}
	} else {
		a := geometry.Rect{X: area.X, Y: area.Y, W: area.W, H: first}
		h := geometry.Rect{X: area.X, Y: area.Y + first, W: area.W, H: gap}
		if rnode, ok := s.First.(ReusableNode); ok {
			out = rnode.ComputeInto(a, out)
		} else {
			out = append(out, s.First.Compute(a)...)
		}
		if gap > 0 {
			out = append(out, Placement{Widget: s.Handle, Area: h})
		}
		secondArea := geometry.Rect{X: area.X, Y: area.Y + first + gap, W: area.W, H: second}
		if rnode, ok := s.Second.(ReusableNode); ok {
			out = rnode.ComputeInto(secondArea, out)
		} else {
			out = append(out, s.Second.Compute(secondArea)...)
		}
	}
	return out
}

// HandleMouse computes a new ratio from a terminal-cell position. App invokes
// this on the resize handle and then relayouts immediately.
func (s *Split) SetRatioFromPoint(area geometry.Rect, ev input.MouseEvent) {
	main := area.W
	pos := ev.X - area.X
	if s.Dir == Vertical {
		main = area.H
		pos = ev.Y - area.Y
	}
	if main <= 1 {
		return
	}
	ratio := float64(pos) / float64(main)
	if ratio < s.Handle.MinRatio {
		ratio = s.Handle.MinRatio
	}
	if ratio > s.Handle.MaxRatio {
		ratio = s.Handle.MaxRatio
	}
	s.Ratio = ratio
}
