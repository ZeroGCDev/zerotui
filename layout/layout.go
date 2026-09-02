/*
Package layout arranges widgets into a Rect tree. Layout.Compute is called once at startup and again on terminal resize - not per frame - so the render loop iterates a cached []Placement with zero allocation.
*/
package layout

import (
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/widget"
)

type Placement struct {
	Widget widget.Widget
	Area   geometry.Rect
}

// Node is anything that can lay itself and its children out inside a Rect, flattening the result into placements.
type Node interface {
	Compute(area geometry.Rect) []Placement
}

// ReusableNode is the optional zero-allocation layout path used by App during
// interactive resize/drag. Implementations append placements into dst instead
// of allocating a new result slice on every reflow. Custom nodes may continue
// implementing Node only; App will fall back to Compute for them.
type ReusableNode interface {
	ComputeInto(area geometry.Rect, dst []Placement) []Placement
}

// ComputeInto reuses dst when root implements ReusableNode.
func ComputeInto(root Node, area geometry.Rect, dst []Placement) []Placement {
	if root == nil {
		return dst[:0]
	}
	if n, ok := root.(ReusableNode); ok {
		return n.ComputeInto(area, dst[:0])
	}
	return append(dst[:0], root.Compute(area)...)
}

// Leaf wraps a single widget as a Node occupying its entire given area.
type Leaf struct{ Widget widget.Widget }

func Wrap(w widget.Widget) Leaf { return Leaf{Widget: w} }

func (l Leaf) Compute(area geometry.Rect) []Placement {
	return []Placement{{Widget: l.Widget, Area: area}}
}

func (l Leaf) ComputeInto(area geometry.Rect, dst []Placement) []Placement {
	return append(dst, Placement{Widget: l.Widget, Area: area})
}

// Fixed pins a child to an explicit size within its parent's flow, overriding the parent's weighted distribution for that one slot.
type Fixed struct {
	Node
	Size int
}
