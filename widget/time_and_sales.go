package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
	"sync"
)

type Trade struct {
	Price, Size uint64
	Buy         bool
}

// TimeAndSales renders a compact time-and-sales stream.
type TimeAndSales struct {
	ThemeOverride                        *style.Theme
	Background                           *color.Color
	mu                                   sync.RWMutex
	trades                               []Trade
	next, count                          int
	Decimals, SizeDecimals, ShowDecimals int
	scratch                              [32]byte
}

func NewTimeAndSales(capacity, decimals, sizeDecimals, showDecimals int) *TimeAndSales {
	if capacity < 1 {
		capacity = 1
	}
	return &TimeAndSales{trades: make([]Trade, capacity), Decimals: decimals, SizeDecimals: sizeDecimals, ShowDecimals: showDecimals}
}
func (t *TimeAndSales) OwnsBackground() bool { return t.Background != nil }
func (t *TimeAndSales) AddTrade(v Trade) {
	t.mu.Lock()
	t.trades[t.next] = v
	t.next = (t.next + 1) % len(t.trades)
	if t.count < len(t.trades) {
		t.count++
	}
	t.mu.Unlock()
}
func (t *TimeAndSales) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if t.ThemeOverride != nil {
		th = t.ThemeOverride
	}
	if a.W < 8 || a.H < 1 {
		return
	}
	if t.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *t.Background})
	}
	b.SetString(a.X, a.Y, "PRICE", bgOr(th.TextMuted, t.Background))
	b.SetString(a.X+a.W/2, a.Y, "SIZE", bgOr(th.TextMuted, t.Background))
	rows := a.H - 1
	if rows <= 0 {
		return
	}
	t.mu.RLock()
	n, next := t.count, t.next
	start := n - rows
	if start < 0 {
		start = 0
	}
	for r := 0; r < rows && start+r < n; r++ {
		i := (next - n + start + r) % len(t.trades)
		if i < 0 {
			i += len(t.trades)
		}
		v := t.trades[i]
		st := bgOr(th.Text, t.Background)
		if v.Buy {
			st = bgOr(th.Positive, t.Background)
		} else {
			st = bgOr(th.Negative, t.Background)
		}
		y := a.Y + 1 + r
		b.FillRect(a.X, y, a.W, 1, ' ', bgOr(th.Panel, t.Background))
		out := t.scratch[:0]
		out = numfmt.AppendFixedPrec(out, v.Price, t.Decimals, t.ShowDecimals)
		b.SetBytes(a.X, y, out, st)
		out = t.scratch[:0]
		out = numfmt.AppendFixedPrec(out, v.Size, t.SizeDecimals, t.ShowDecimals)
		b.SetBytes(a.X+a.W/2, y, out, st)
	}
	t.mu.RUnlock()
}
