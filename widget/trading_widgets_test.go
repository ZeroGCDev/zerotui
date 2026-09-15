package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

func tradingTestBuffer() *buffer.Buffer { return buffer.New(120, 40) }
func TestTradingWidgetsDraw(t *testing.T) {
	th := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 120, H: 20}
	b := tradingTestBuffer()
	ob := NewOrderBook(2, 2, 2)
	ob.SetLevels([]Level{{Price: 12345, Size: 10}, {Price: 12340, Size: 8}}, []Level{{Price: 12350, Size: 9}})
	ob.Draw(b, area, th)
	ts := NewTimeAndSales(64, 2, 2, 2)
	ts.AddTrade(Trade{Price: 12345, Size: 4, Buy: true})
	ts.Draw(b, area, th)
	ps := NewPositionList()
	var tpOn, slOn, tpValue, slValue uint32
	panel := NewPositionPanel(&tpOn, &slOn, &tpValue, &slValue)
	panel.SetData(PositionPanelData{Symbol: "NIFTY", Side: "LONG", Quantity: "10", Entry: "10000", Mark: "10010", ROI: "+1.00%", Leverage: "10x", PnL: "+100"})
	ps.SetItems([]PositionListItem{{Key: "nifty", Panel: panel}})
	ps.Draw(b, area, th)
	os := NewOrders(16, 2, 2)
	os.SetRows([]Order{{ID: "1", Symbol: "NIFTY", Side: "BUY", Price: 10000, Qty: 10, Status: "OPEN"}})
	os.Draw(b, area, th)
	pnl := NewPnL(2, 2)
	pnl.Realized.Store(100)
	pnl.Unrealized.Store(50)
	pnl.Fees.Store(5)
	pnl.Draw(b, area, th)
	lm := NewLatencyMonitor()
	lm.Stats.Last.Store(150)
	lm.Draw(b, area, th)
	rm := NewRiskMonitor(2, 2)
	rm.State.Utilization.Store(125)
	rm.Draw(b, area, th)
	ms := NewMarketStatus("NSE", "NIFTY")
	ms.SetState(MarketLive)
	ms.Draw(b, area, th)
	oe := NewOrderEntry()
	oe.Symbol = "NIFTY"
	oe.Price = "10000"
	oe.Qty = "1"
	oe.Focus(true)
	oe.Draw(b, area, th)
	if !oe.HandleKey(inputKeyEnter()) {
		t.Fatal("order entry did not consume enter")
	}
}
func inputKeyEnter() (k input.Key) { k.Type = input.KeyEnter; return }

func BenchmarkTradingWidgetsDraw(b *testing.B) {
	th := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 120, H: 40}
	buf := tradingTestBuffer()
	ts := NewTimeAndSales(256, 2, 2, 2)
	for i := 0; i < 256; i++ {
		ts.AddTrade(Trade{Price: uint64(10000 + i), Size: uint64(i + 1), Buy: i&1 == 0})
	}
	ps := NewPositionList()
	items := make([]PositionListItem, 32)
	for i := range items {
		var tpOn, slOn, tpValue, slValue uint32
		p := NewPositionPanel(&tpOn, &slOn, &tpValue, &slValue)
		p.SetData(PositionPanelData{Symbol: "SYM", Quantity: "1", Mark: "10000", PnL: "+1"})
		items[i] = PositionListItem{Key: string(rune('A' + i)), Panel: p}
	}
	ps.SetItems(items)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.Draw(buf, area, th)
		ps.Draw(buf, area, th)
	}
}

func TestOrdersSetRowsReportsChange(t *testing.T) {
	o := NewOrders(4, 2, 2)
	row := Order{ID: "1", Symbol: "B-BTC_USDT", Side: "SELL", Status: "OPEN", Price: 10350, Qty: 100}
	if !o.SetRows([]Order{row}) {
		t.Fatal("first row publication should report changed")
	}
	if o.SetRows([]Order{row}) {
		t.Fatal("same rows should not report changed")
	}
}
