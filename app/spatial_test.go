package app

import (
	"io"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
)

type spatialLeaf struct{}

func (*spatialLeaf) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func TestSpatialCandidates(t *testing.T) {
	p := make([]layout.Placement, 0, 100)
	for i := 0; i < 100; i++ {
		p = append(p, layout.Placement{Widget: &spatialLeaf{}, Area: geometry.Rect{X: (i % 10) * 12, Y: (i / 10) * 4, W: 10, H: 3}})
	}
	var idx spatialIndex
	idx.rebuild(120, 40, p)
	got := idx.candidateIndices(geometry.Rect{X: 0, Y: 0, W: 10, H: 3}, p)
	if len(got) != 1 || p[got[0]].Area.X != 0 || p[got[0]].Area.Y != 0 {
		t.Fatalf("unexpected candidates: %v", got)
	}
}

func TestFastPathSchedulerDefaultsToIdle(t *testing.T) {
	a := New(layout.Wrap(&spatialLeaf{}), nil)
	if a.live.Load() != 0 {
		t.Fatal("new app must be non-live")
	}
	if a.wake == nil {
		t.Fatal("new app must have a wake channel")
	}
}

func TestSpatialCandidatesPreservePaintOrder(t *testing.T) {
	// Both placements overlap the same damage region. The child is placed
	// after the border in the retained placement list and must be returned
	// after it even though the grid stores entries in reverse insertion order.
	border := &spatialLeaf{}
	child := &spatialLeaf{}
	p := []layout.Placement{
		{Widget: border, Area: geometry.Rect{X: 0, Y: 0, W: 20, H: 8}},
		{Widget: child, Area: geometry.Rect{X: 2, Y: 1, W: 16, H: 6}},
	}
	var idx spatialIndex
	idx.rebuild(40, 20, p)
	got := idx.candidateIndices(geometry.Rect{X: 3, Y: 2, W: 4, H: 2}, p)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("spatial query lost paint order: got %v", got)
	}
}

func TestAdaptiveDamagePromotesLargeUnion(t *testing.T) {
	a := New(layout.Wrap(&spatialLeaf{}), nil)
	a.width, a.height = 100, 40
	a.damageMu.Lock()
	a.addDamageLocked(geometry.Rect{X: 0, Y: 0, W: 80, H: 30}, false)
	a.damageMu.Unlock()
	if !a.damageFull {
		t.Fatal("large damage should promote to full repaint")
	}
}

func TestSparseTargetedDamageMarksPlacementDirty(t *testing.T) {
	w := &spatialLeaf{}
	a := New(layout.Wrap(w), nil)
	a.width, a.height = 20, 10
	a.buf = buffer.New(20, 10)
	a.relayout()
	a.requestRender()
	a.drawTo(io.Discard)
	if _, dirty := a.RetainedState(); dirty != 0 {
		t.Fatalf("initial frame left dirty placements: %d", dirty)
	}
	a.InvalidateWidgets(w)
	_, dirty := a.RetainedState()
	if dirty != 1 {
		t.Fatalf("targeted invalidation marked %d placements dirty, want 1", dirty)
	}
}
