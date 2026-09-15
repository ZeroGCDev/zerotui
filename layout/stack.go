package layout

import "github.com/ZeroGCDev/zerotui/geometry"

// Stack composes children in z-order. Later children are drawn and hit-tested
// above earlier children, making modal/popup/command-palette overlays possible
// without introducing a separate rendering engine.
type Stack struct {
	Children []Node
}

func NewStack(children ...Node) *Stack { return &Stack{Children: children} }

func (s *Stack) Compute(area geometry.Rect) []Placement { return s.ComputeInto(area, nil) }

func (s *Stack) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	for _, child := range s.Children {
		if child == nil {
			continue
		}
		if r, ok := child.(ReusableNode); ok {
			out = r.ComputeInto(area, out)
		} else {
			out = append(out, child.Compute(area)...)
		}
	}
	return out
}
