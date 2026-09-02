package widget

import (
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
)

/*
PriceTicker reads a scaled fixed-point price from an atomic uint64 (the same representation the reference terminal's market engine writes) and renders it with numfmt directly into the cell buffer - no Sprintf, no string concatenation, no allocation, every frame, matching the 0 B/op hot path of the original prototype's writePrice.

It also flashes Positive/Negative styling on up/down ticks, which is pure render-thread state (not atomic; only the render goroutine touches it) so it costs nothing extra.
*/
type PriceTicker struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Label         string
	Price         *uint64      // atomic, scaled by 10^Decimals
	Decimals      int          // full internal precision of *Price
	Show          int          // visible decimal digits (<=Decimals)
	Background    *color.Color // nil = inherit whatever's behind it (default)
	prevSeen      uint64
	haveSeen      bool
	scratch       [32]byte
}

func NewPriceTicker(label string, price *uint64, decimals, show int) *PriceTicker {
	return &PriceTicker{Label: label, Price: price, Decimals: decimals, Show: show}
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (p *PriceTicker) OwnsBackground() bool { return p.Background != nil }

func (p *PriceTicker) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if p.ThemeOverride != nil {
		theme = p.ThemeOverride
	}
	if area.H <= 0 {
		return
	}
	cur := atomic.LoadUint64(p.Price)

	st := bgOr(theme.Text, p.Background)
	if p.haveSeen {
		switch {
		case cur > p.prevSeen:
			st = bgOr(theme.Positive, p.Background)
		case cur < p.prevSeen:
			st = bgOr(theme.Negative, p.Background)
		}
	}
	p.prevSeen = cur
	p.haveSeen = true

	if p.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', style.Style{Bg: *p.Background})
	}

	x := area.X
	if p.Label != "" {
		buf.SetString(x, area.Y, p.Label, bgOr(theme.TextMuted, p.Background))
		x += len(p.Label) + 1
	}
	out := p.scratch[:0]
	out = numfmt.AppendFixedPrec(out, cur, p.Decimals, p.Show)
	buf.SetBytes(x, area.Y, out, st)
}
