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

// PreferredHeight is an optional intrinsic-height hint used by FitHeight.
// It is evaluated during layout only; rendering remains allocation-free.
type PreferredHeight interface {
	PreferredHeight() int
}

// FitHeight keeps a child at its preferred height when space permits and
// clamps it to the available height when the terminal is smaller. It is an
// explicit layout constraint: a widget merely implementing PreferredHeight
// does not alter Flex sizing unless it is wrapped with FitHeight.
func FitHeight(child Node) Node { return &fitHeight{child: child} }

// preferredHeightConstraint is deliberately private. This keeps intrinsic
// sizing an explicit FitHeight operation instead of making PreferredHeight a
// hidden change to the long-standing Flex1/FlexN behavior.
type preferredHeightConstraint interface {
	layoutPreferredHeight() int
}

type fitHeight struct{ child Node }

func (f *fitHeight) layoutPreferredHeight() int {
	if f == nil || f.child == nil {
		return 0
	}
	if ph, ok := f.child.(PreferredHeight); ok {
		return ph.PreferredHeight()
	}
	return 0
}

func (f *fitHeight) Compute(area geometry.Rect) []Placement { return f.ComputeInto(area, nil) }
func (f *fitHeight) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if f == nil || f.child == nil || area.H <= 0 {
		return out
	}
	h := area.H
	if ph, ok := f.child.(PreferredHeight); ok {
		if preferred := ph.PreferredHeight(); preferred > 0 && preferred < h {
			h = preferred
		}
	}
	childArea := geometry.Rect{X: area.X, Y: area.Y, W: area.W, H: h}
	if r, ok := f.child.(ReusableNode); ok {
		return r.ComputeInto(childArea, out)
	}
	return append(out, f.child.Compute(childArea)...)
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

// PreferredHeight forwards an optional intrinsic-height hint from a wrapped
// widget. This keeps layout.Wrap transparent to content-sized vertical flows;
// without it, a Bordered or Flex parent cannot see a widget's preferred size.
func (l Leaf) PreferredHeight() int {
	if l.Widget == nil {
		return 0
	}
	if ph, ok := l.Widget.(interface{ PreferredHeight() int }); ok {
		return ph.PreferredHeight()
	}
	return 0
}

// Fixed pins a child to an explicit size within its parent's flow, overriding the parent's weighted distribution for that one slot.
type Fixed struct {
	Node
	Size int
}
