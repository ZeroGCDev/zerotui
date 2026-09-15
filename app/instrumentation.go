package app

import "sync/atomic"

// Metrics is a compact, lock-free snapshot of renderer activity. Counters are
// updated only at frame boundaries or from the render path; reading them never
// allocates and never stops the renderer.
type Metrics struct {
	Frames           uint64
	FullFrames       uint64
	PartialFrames    uint64
	PaintedWidgets   uint64
	CandidateWidgets uint64
	SkippedWidgets   uint64
	DamageRegions    uint64
	LayoutPasses     uint64
	LayoutCacheHits  uint64
	BytesWritten     uint64
	LastFrameNS      uint64
	LastLayoutNS     uint64
}

type metricsState struct {
	frames           atomic.Uint64
	fullFrames       atomic.Uint64
	partialFrames    atomic.Uint64
	paintedWidgets   atomic.Uint64
	candidateWidgets atomic.Uint64
	skippedWidgets   atomic.Uint64
	damageRegions    atomic.Uint64
	layoutPasses     atomic.Uint64
	layoutCacheHits  atomic.Uint64
	bytesWritten     atomic.Uint64
	lastFrameNS      atomic.Uint64
	lastLayoutNS     atomic.Uint64
}

func (m *metricsState) snapshot() Metrics {
	return Metrics{
		Frames:           m.frames.Load(),
		FullFrames:       m.fullFrames.Load(),
		PartialFrames:    m.partialFrames.Load(),
		PaintedWidgets:   m.paintedWidgets.Load(),
		CandidateWidgets: m.candidateWidgets.Load(),
		SkippedWidgets:   m.skippedWidgets.Load(),
		DamageRegions:    m.damageRegions.Load(),
		LayoutPasses:     m.layoutPasses.Load(),
		LayoutCacheHits:  m.layoutCacheHits.Load(),
		BytesWritten:     m.bytesWritten.Load(),
		LastFrameNS:      m.lastFrameNS.Load(),
		LastLayoutNS:     m.lastLayoutNS.Load(),
	}
}

// Metrics returns a point-in-time renderer snapshot. It is safe to call from a
// dashboard/status goroutine and has no heap allocation.
// EnableMetrics toggles renderer instrumentation. It is disabled by default so
// production applications pay no per-frame counter/timestamp cost. Stress
// labs and benchmarks enable it explicitly.
func (a *App) EnableMetrics(enabled bool) { a.metricsEnabled.Store(enabled) }

// ResetMetrics clears all counters. Intended for benchmark phases and demos.
func (a *App) ResetMetrics() {
	a.metrics.frames.Store(0)
	a.metrics.fullFrames.Store(0)
	a.metrics.partialFrames.Store(0)
	a.metrics.paintedWidgets.Store(0)
	a.metrics.candidateWidgets.Store(0)
	a.metrics.skippedWidgets.Store(0)
	a.metrics.damageRegions.Store(0)
	a.metrics.layoutPasses.Store(0)
	a.metrics.layoutCacheHits.Store(0)
	a.metrics.bytesWritten.Store(0)
	a.metrics.lastFrameNS.Store(0)
	a.metrics.lastLayoutNS.Store(0)
}

func (a *App) Metrics() Metrics { return a.metrics.snapshot() }
