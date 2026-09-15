package layout

import "github.com/ZeroGCDev/zerotui/geometry"

// Breakpoint selects one of two already-built layouts based on terminal width.
// Like OpenTUI's flex-wrap/responsive behavior, the same renderables remain
// alive; only their geometry is changed at a resize boundary.
type Breakpoint struct {
	MinWidth int
	Compact  Node
	Expanded Node
}

func Responsive(minWidth int, compact, expanded Node) Node {
	return &Breakpoint{MinWidth: minWidth, Compact: compact, Expanded: expanded}
}

func (b *Breakpoint) Compute(area geometry.Rect) []Placement { return b.ComputeInto(area, nil) }

func (b *Breakpoint) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	child := b.Expanded
	if area.W < b.MinWidth {
		child = b.Compact
	}
	if child == nil {
		return out
	}
	if rnode, ok := child.(ReusableNode); ok {
		return rnode.ComputeInto(area, out)
	}
	return append(out, child.Compute(area)...)
}
