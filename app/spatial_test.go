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
	a.addDamageLocked(geometry.Rect{X: 0, Y: 0, W: 80, H: 30})
	a.damageMu.Unlock()
	if !a.damageFull {
		t.Fatal("large damage should promote to full repaint")
	}
}

func TestAdaptiveDamagePromotesDisjointCoverage(t *testing.T) {
	a := New(layout.Wrap(&spatialLeaf{}), nil)
	a.width, a.height = 100, 40
	a.damageMu.Lock()
	a.addDamageLocked(geometry.Rect{X: 0, Y: 0, W: 30, H: 20})
	a.addDamageLocked(geometry.Rect{X: 70, Y: 0, W: 30, H: 20})
	a.addDamageLocked(geometry.Rect{X: 0, Y: 20, W: 30, H: 20})
	a.addDamageLocked(geometry.Rect{X: 70, Y: 20, W: 30, H: 20})
	full := a.damageFull
	a.damageMu.Unlock()
	if !full {
		t.Fatal("disjoint damage covering more than half the screen should promote to full repaint")
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

func TestInvalidateWidgetsHandlesRepeatedPlacementWithoutAllocatingIndexSlices(t *testing.T) {
	w := &resizeTestWidget{}
	root := layout.NewFlex(layout.Horizontal, layout.Fix(layout.Wrap(w), 10), layout.Flex1(layout.Wrap(w)))
	a := New(root, nil)
	a.width, a.height = 80, 24
	a.buf = buffer.New(80, 24)
	a.relayout()

	refs, ok := a.widgetIndex[w]
	if !ok || refs.first != 0 || len(refs.extra) != 1 || refs.extra[0] != 1 {
		t.Fatalf("repeated widget index=%#v", refs)
	}
	a.damageMu.Lock()
	a.damageCount = 0
	a.damageFull = false
	a.damageMu.Unlock()
	a.InvalidateWidgets(w)
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	if a.damageCount == 0 && !a.damageFull {
		t.Fatal("repeated widget was not invalidated")
	}
}

func TestDamageCoalescingIsTransitive(t *testing.T) {
	a := New(layout.Wrap(&spatialLeaf{}), nil)
	a.width, a.height = 100, 20
	a.damageMu.Lock()
	// A and C do not touch. B bridges them. A correct coalescer must end with
	// one region rather than leaving a stale split after the bridge is merged.
	a.addDamageLocked(geometry.Rect{X: 0, Y: 0, W: 10, H: 10})
	a.addDamageLocked(geometry.Rect{X: 30, Y: 0, W: 10, H: 10})
	a.addDamageLocked(geometry.Rect{X: 9, Y: 0, W: 22, H: 10})
	if a.damageFull || a.damageCount != 1 {
		a.damageMu.Unlock()
		t.Fatalf("transitive merge count=%d full=%v", a.damageCount, a.damageFull)
	}
	want := geometry.Rect{X: 0, Y: 0, W: 40, H: 10}
	if a.damage[0] != want {
		a.damageMu.Unlock()
		t.Fatalf("merged damage=%v want=%v", a.damage[0], want)
	}
	a.damageMu.Unlock()
}

func TestInvalidateRectClipsToTerminal(t *testing.T) {
	a := New(layout.Wrap(&spatialLeaf{}), nil)
	a.width, a.height = 20, 10
	a.InvalidateRect(geometry.Rect{X: -5, Y: -3, W: 10, H: 8})
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	if a.damageFull || a.damageCount != 1 {
		t.Fatalf("unexpected clipped damage count=%d full=%v", a.damageCount, a.damageFull)
	}
	want := geometry.Rect{X: 0, Y: 0, W: 5, H: 5}
	if a.damage[0] != want {
		t.Fatalf("clipped damage=%v want=%v", a.damage[0], want)
	}
}

func TestInvalidateRectOutsideTerminalIsIgnored(t *testing.T) {
	a := New(layout.Wrap(&spatialLeaf{}), nil)
	a.width, a.height = 20, 10
	a.InvalidateRect(geometry.Rect{X: 25, Y: 2, W: 5, H: 5})
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	if a.damageFull || a.damageCount != 0 {
		t.Fatalf("outside damage was retained: count=%d full=%v", a.damageCount, a.damageFull)
	}
}

type drawCountingWidget struct {
	count int
}

func (w *drawCountingWidget) Draw(buf *buffer.Buffer, area geometry.Rect, _ *style.Theme) {
	w.count++
	if area.W > 0 && area.H > 0 {
		buf.Set(area.X, area.Y, 'X', style.Style{})
	}
}

func TestIdleFrameDoesNotRedraw(t *testing.T) {
	w := &drawCountingWidget{}
	a := New(layout.Wrap(w), nil)
	a.width, a.height = 20, 10
	a.buf = buffer.New(20, 10)
	a.relayout()
	a.dirty.Store(true)
	if err := a.drawTo(io.Discard); err != nil {
		t.Fatal(err)
	}
	if w.count != 1 {
		t.Fatalf("initial draw count=%d want 1", w.count)
	}
	if err := a.drawTo(io.Discard); err != nil {
		t.Fatal(err)
	}
	if w.count != 1 {
		t.Fatalf("idle draw count=%d want unchanged 1", w.count)
	}
}
