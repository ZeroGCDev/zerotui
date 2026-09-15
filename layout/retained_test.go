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

type fixedSizeTestNode struct{}

func (fixedSizeTestNode) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}
func (n fixedSizeTestNode) Compute(area geometry.Rect) []Placement {
	return []Placement{{Widget: n, Area: area}}
}

func TestFixedSizeConstrainsChild(t *testing.T) {
	l := fixedSizeTestNode{}
	n := FixedSize(Wrap(l), 8, 3)
	pl := n.Compute(geometry.Rect{X: 0, Y: 0, W: 20, H: 10})
	if len(pl) != 1 {
		t.Fatalf("placements = %d, want 1", len(pl))
	}
	if got := pl[0].Area; got.W != 8 || got.H != 3 || got.X != 6 || got.Y != 3 {
		t.Fatalf("area = %+v, want centered 8x3 at (6,3)", got)
	}
}

func TestSizeBoundsAndPadding(t *testing.T) {
	leaf := fixedSizeTestNode{}
	n := Padding(SizeBounds(Wrap(leaf), 8, 12, 3, 5), 2, 1, 2, 1)
	pl := n.Compute(geometry.Rect{X: 0, Y: 0, W: 30, H: 15})
	if len(pl) != 1 {
		t.Fatalf("placements=%d want 1", len(pl))
	}
	if got := pl[0].Area; got.W != 12 || got.H != 5 || got.X != 9 || got.Y != 5 {
		t.Fatalf("area=%+v want centered 12x5 at (9,5)", got)
	}
}
