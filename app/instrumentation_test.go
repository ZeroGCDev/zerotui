package app

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
)

type metricLeaf struct{}

func (*metricLeaf) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func TestMetricsStartEmpty(t *testing.T) {
	a := New(layout.Wrap(&metricLeaf{}), nil)
	m := a.Metrics()
	if m.Frames != 0 || m.BytesWritten != 0 || m.LayoutPasses != 0 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestRetainedLayoutCacheHit(t *testing.T) {
	a := New(layout.NewRetained(layout.Wrap(&metricLeaf{})), nil)
	a.EnableMetrics(true)
	a.width, a.height = 80, 24
	a.buf = buffer.New(80, 24)
	a.relayout()
	before := a.Metrics().LayoutPasses
	a.relayout()
	after := a.Metrics()
	if after.LayoutPasses != before {
		t.Fatalf("unexpected second layout pass: before=%d after=%d", before, after.LayoutPasses)
	}
	if after.LayoutCacheHits == 0 {
		t.Fatal("expected retained layout cache hit")
	}
}

func TestBatchCoalescesWakeupsAndThemeSwapInvalidates(t *testing.T) {
	a := New(layout.Wrap(&metricLeaf{}), style.TokyoNightTheme())
	a.BeginBatch()
	a.Invalidate()
	a.Invalidate()
	if a.batchDepth.Load() != 1 {
		t.Fatalf("batch depth=%d want 1", a.batchDepth.Load())
	}
	a.EndBatch()
	if !a.dirty.Load() {
		t.Fatal("batch should leave app dirty")
	}
	custom := style.NordTheme()
	a.dirty.Store(false)
	a.SetTheme(custom)
	if a.Theme != custom || !a.dirty.Load() {
		t.Fatal("SetTheme did not swap theme and invalidate")
	}
}
