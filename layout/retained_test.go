package layout

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

type retainedCountingNode struct{ calls int }

func (n *retainedCountingNode) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}
func (n *retainedCountingNode) Compute(area geometry.Rect) []Placement {
	n.calls++
	return []Placement{{Widget: n, Area: area}}
}
func (n *retainedCountingNode) ComputeInto(area geometry.Rect, dst []Placement) []Placement {
	n.calls++
	return append(dst, Placement{Widget: n, Area: area})
}

func TestRetainedSkipsCleanSubtreeAndRecomputesWhenInvalidated(t *testing.T) {
	child := &retainedCountingNode{}
	r := NewRetained(child)
	area := geometry.Rect{W: 80, H: 24}
	out := make([]Placement, 0, 1)
	out = ComputeInto(r, area, out)
	out = ComputeInto(r, area, out[:0])
	if child.calls != 1 {
		t.Fatalf("clean retained subtree recomputed %d times; want 1", child.calls)
	}
	r.Invalidate()
	out = ComputeInto(r, area, out[:0])
	if child.calls != 2 {
		t.Fatalf("invalidated retained subtree computed %d times; want 2", child.calls)
	}
	if len(out) != 1 {
		t.Fatalf("placements=%d want 1", len(out))
	}
}
