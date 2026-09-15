package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
	"sync/atomic"
)

// RiskState is an atomic risk snapshot. Utilization is hundredths of a percent.
type RiskState struct {
	Utilization, DailyLoss, Drawdown atomic.Int64
	Breached                         atomic.Bool
}

// RiskMonitor renders atomic risk utilization and loss statistics.
type RiskMonitor struct {
	ThemeOverride          *style.Theme
	Background             *color.Color
	State                  *RiskState
	Decimals, ShowDecimals int
	scratch                [32]byte
}

func NewRiskMonitor(decimals, show int) *RiskMonitor {
	return &RiskMonitor{State: &RiskState{}, Decimals: decimals, ShowDecimals: show}
}
func (r *RiskMonitor) OwnsBackground() bool { return r.Background != nil }
func (r *RiskMonitor) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if r.ThemeOverride != nil {
		th = r.ThemeOverride
	}
	if a.W < 12 || a.H < 1 {
		return
	}
	if r.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *r.Background})
	}
	st := bgOr(th.Text, r.Background)
	if r.State.Breached.Load() {
		st = bgOr(th.Negative, r.Background)
	}
	b.SetString(a.X, a.Y, "RISK", bgOr(th.Title, r.Background))
	out := r.scratch[:0]
	out = numfmt.AppendSignedFixedPrec(out, r.State.Utilization.Load(), 2, 2)
	b.SetBytes(a.X+5, a.Y, out, st)
	if a.H > 1 {
		b.SetString(a.X, a.Y+1, "LOSS", bgOr(th.TextMuted, r.Background))
		out = r.scratch[:0]
		out = numfmt.AppendSignedFixedPrec(out, r.State.DailyLoss.Load(), r.Decimals, r.ShowDecimals)
		b.SetBytes(a.X+5, a.Y+1, out, st)
		b.SetString(a.X+a.W/2, a.Y+1, "DD", bgOr(th.TextMuted, r.Background))
		out = r.scratch[:0]
		out = numfmt.AppendSignedFixedPrec(out, r.State.Drawdown.Load(), r.Decimals, r.ShowDecimals)
		b.SetBytes(a.X+a.W/2+3, a.Y+1, out, st)
	}
}
