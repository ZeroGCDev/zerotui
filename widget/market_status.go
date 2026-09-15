package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"sync/atomic"
)

// MarketState is a connection/session state, published atomically.
type MarketState uint32

const (
	MarketDisconnected MarketState = iota
	MarketConnecting
	MarketLive
	MarketHalted
)

// MarketStatus renders connection and market-session state.
type MarketStatus struct {
	ThemeOverride *style.Theme
	Background    *color.Color
	State         atomic.Uint32
	Venue, Symbol string
}

func NewMarketStatus(venue, symbol string) *MarketStatus {
	return &MarketStatus{Venue: venue, Symbol: symbol}
}
func (m *MarketStatus) OwnsBackground() bool   { return m.Background != nil }
func (m *MarketStatus) SetState(s MarketState) { m.State.Store(uint32(s)) }
func (m *MarketStatus) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if m.ThemeOverride != nil {
		th = m.ThemeOverride
	}
	if a.W < 8 || a.H < 1 {
		return
	}
	if m.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *m.Background})
	}
	st := bgOr(th.TextMuted, m.Background)
	label := "DOWN"
	switch MarketState(m.State.Load()) {
	case MarketConnecting:
		label = "CONNECT"
		st = bgOr(th.Warning, m.Background)
	case MarketLive:
		label = "LIVE"
		st = bgOr(th.Positive, m.Background)
	case MarketHalted:
		label = "HALT"
		st = bgOr(th.Negative, m.Background)
	}
	b.SetString(a.X, a.Y, label, st)
	if a.W > len(label)+1 {
		b.SetString(a.X+len(label)+1, a.Y, m.Venue, bgOr(th.Text, m.Background))
	}
	if a.H > 1 && m.Symbol != "" {
		b.SetString(a.X, a.Y+1, m.Symbol, bgOr(th.Text, m.Background))
	}
}
