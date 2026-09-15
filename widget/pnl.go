package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
	"sync/atomic"
)

// PnL uses atomics so the risk/strategy side can publish without taking the UI lock.
// PnL renders realized, unrealized, and fee-adjusted P&L.
type PnL struct {
	ThemeOverride              *style.Theme
	Background                 *color.Color
	Realized, Unrealized, Fees atomic.Int64
	Decimals, ShowDecimals     int
	scratch                    [48]byte
}

func NewPnL(decimals, show int) *PnL { return &PnL{Decimals: decimals, ShowDecimals: show} }
func (p *PnL) OwnsBackground() bool  { return p.Background != nil }
func (p *PnL) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if p.ThemeOverride != nil {
		th = p.ThemeOverride
	}
	if a.W < 8 || a.H < 1 {
		return
	}
	if p.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *p.Background})
	}
	r, u, f := p.Realized.Load(), p.Unrealized.Load(), p.Fees.Load()
	total := r + u - f
	st := bgOr(th.Text, p.Background)
	if total > 0 {
		st = bgOr(th.Positive, p.Background)
	} else if total < 0 {
		st = bgOr(th.Negative, p.Background)
	}
	b.SetString(a.X, a.Y, "P&L", bgOr(th.Title, p.Background))
	out := p.scratch[:0]
	out = numfmt.AppendSignedFixedPrec(out, total, p.Decimals, p.ShowDecimals)
	b.SetBytes(a.X+4, a.Y, out, st)
	if a.H > 1 {
		b.SetString(a.X, a.Y+1, "R", bgOr(th.TextMuted, p.Background))
		out = p.scratch[:0]
		out = numfmt.AppendSignedFixedPrec(out, r, p.Decimals, p.ShowDecimals)
		b.SetBytes(a.X+2, a.Y+1, out, st)
		b.SetString(a.X+a.W/2, a.Y+1, "U", bgOr(th.TextMuted, p.Background))
		out = p.scratch[:0]
		out = numfmt.AppendSignedFixedPrec(out, u, p.Decimals, p.ShowDecimals)
		b.SetBytes(a.X+a.W/2+2, a.Y+1, out, st)
	}
}
