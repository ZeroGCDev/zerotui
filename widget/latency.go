package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
	"sync/atomic"
)

// LatencyStats is an atomic nanosecond summary; feed/gateway code can update it directly.
type LatencyStats struct{ Min, Max, Last, Count atomic.Uint64 }

// LatencyMonitor renders atomic feed/gateway latency statistics.
type LatencyMonitor struct {
	ThemeOverride                  *style.Theme
	Background                     *color.Color
	Stats                          *LatencyStats
	ThresholdWarn, ThresholdDanger uint64
	scratch                        [32]byte
}

func NewLatencyMonitor() *LatencyMonitor       { return &LatencyMonitor{Stats: &LatencyStats{}} }
func (l *LatencyMonitor) OwnsBackground() bool { return l.Background != nil }
func (l *LatencyMonitor) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if l.ThemeOverride != nil {
		th = l.ThemeOverride
	}
	if a.W < 10 || a.H < 1 {
		return
	}
	if l.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *l.Background})
	}
	last := l.Stats.Last.Load()
	st := bgOr(th.Text, l.Background)
	if l.ThresholdDanger > 0 && last >= l.ThresholdDanger {
		st = bgOr(th.Negative, l.Background)
	} else if l.ThresholdWarn > 0 && last >= l.ThresholdWarn {
		st = bgOr(th.Warning, l.Background)
	}
	b.SetString(a.X, a.Y, "LAT", bgOr(th.Title, l.Background))
	out := l.scratch[:0]
	out = numfmt.AppendUint(out, last)
	b.SetBytes(a.X+4, a.Y, out, st)
	if a.H > 1 {
		b.SetString(a.X, a.Y+1, "MIN", bgOr(th.TextMuted, l.Background))
		out = l.scratch[:0]
		out = numfmt.AppendUint(out, l.Stats.Min.Load())
		b.SetBytes(a.X+4, a.Y+1, out, st)
		b.SetString(a.X+a.W/2, a.Y+1, "MAX", bgOr(th.TextMuted, l.Background))
		out = l.scratch[:0]
		out = numfmt.AppendUint(out, l.Stats.Max.Load())
		b.SetBytes(a.X+a.W/2+4, a.Y+1, out, st)
	}
}
