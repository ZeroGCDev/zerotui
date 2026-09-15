package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.TokyoNightTheme()

	trades := widget.NewTimeAndSales(64, 2, 2, 2)
	for i := 0; i < 12; i++ {
		trades.AddTrade(widget.Trade{Price: uint64(7890000 + i*25), Size: uint64(10 + i), Buy: i%2 == 0})
	}

	var tpBTC, slBTC, tpETH, slETH, tpSOL, slSOL uint32 = 1, 0, 1, 1, 0, 1
	var tpBTCValue, slBTCValue, tpETHValue, slETHValue, tpSOLValue, slSOLValue uint32 = 50, 25, 75, 50, 100, 25

	btcPanel := widget.NewPositionPanel(&tpBTC, &slBTC, &tpBTCValue, &slBTCValue)
	btcPanel.Title = "BTC-PERP"
	btcPanel.SetData(widget.PositionPanelData{
		Symbol: "BTC-PERP", Side: "LONG", Quantity: "2", Entry: "78,100", Mark: "78,900", ROI: "+1.02%", Leverage: "10x", PnL: "+1,600", TPPrice: "79,400", TPOrder: "READY",
	})

	ethPanel := widget.NewPositionPanel(&tpETH, &slETH, &tpETHValue, &slETHValue)
	ethPanel.Title = "ETH-PERP"
	ethPanel.SetData(widget.PositionPanelData{
		Symbol: "ETH-PERP", Side: "SHORT", Quantity: "4", Entry: "3,250", Mark: "3,210", ROI: "+1.23%", Leverage: "8x", PnL: "+160", TPPrice: "3,150", TPOrder: "READY",
	})

	solPanel := widget.NewPositionPanel(&tpSOL, &slSOL, &tpSOLValue, &slSOLValue)
	solPanel.Title = "SOL-PERP"
	solPanel.SetData(widget.PositionPanelData{
		Symbol: "SOL-PERP", Side: "LONG", Quantity: "12", Entry: "148.00", Mark: "151.00", ROI: "+2.03%", Leverage: "5x", PnL: "+36", TPPrice: "154.00", TPOrder: "READY",
	})

	positions := widget.NewPositionList()
	positions.PanelHeight = 6
	positions.Gap = 1
	positions.SetItems([]widget.PositionListItem{
		{Key: "BTC-PERP", Panel: btcPanel},
		{Key: "ETH-PERP", Panel: ethPanel},
		{Key: "SOL-PERP", Panel: solPanel},
	})

	orders := widget.NewOrders(32, 2, 2)
	orders.SetRows([]widget.Order{
		{ID: "A102", Symbol: "BTC-PERP", Side: "BUY", Status: "OPEN", Price: 7885000, Qty: 3, Filled: 1},
		{ID: "A103", Symbol: "ETH-PERP", Side: "SELL", Status: "PARTIAL", Price: 322500, Qty: 8, Filled: 3},
	})

	pnl := widget.NewPnL(2, 2)
	pnl.Realized.Store(125000)
	pnl.Unrealized.Store(32000)
	pnl.Fees.Store(1800)

	latency := widget.NewLatencyMonitor()
	latency.Stats.Last.Store(420)
	latency.Stats.Min.Store(180)
	latency.Stats.Max.Store(910)
	latency.Stats.Count.Store(250000)
	latency.ThresholdWarn = 500
	latency.ThresholdDanger = 1000

	risk := widget.NewRiskMonitor(2, 2)
	risk.State.Utilization.Store(37)
	risk.State.DailyLoss.Store(1250)
	risk.State.Drawdown.Store(2400)

	status := widget.NewMarketStatus("Example Exchange", "BTC-PERP")
	status.SetState(widget.MarketLive)

	entry := widget.NewOrderEntry()
	entry.Symbol, entry.Side, entry.Price, entry.Qty = "BTC-PERP", "BUY", "78900.00", "1"
	entry.OnSubmit = func(symbol, side, price, qty string) {
		fmt.Printf("order: %s %s %s @ %s\n", side, qty, symbol, price)
	}
	entry.Focus(true)

	card := func(title string, w widget.Widget) layout.Node {
		return layout.BorderedRounded(title, layout.Wrap(w), nil)
	}

	grid := layout.NewGrid(2, 4,
		card("TIME & SALES", trades),
		card("POSITIONS", positions),
		card("ORDERS", orders),
		card("P&L", pnl),
		card("LATENCY", latency),
		card("RISK", risk),
		card("MARKET STATUS", status),
		card("ORDER ENTRY", entry),
	)

	footer := widget.NewShortcutHelpBar(
		widget.Shortcut{Key: "Tab", Label: "focus"},
		widget.Shortcut{Key: "Enter", Label: "submit"},
		widget.Shortcut{Key: "q", Label: "quit"},
	)

	root := layout.Center(layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("  TRADING WIDGETS / LIVE DATA SURFACES")), 1),
		layout.Flex1(grid),
		layout.Fix(layout.Wrap(footer), 2),
	), .96, .92)

	// Live simulator: continuously refresh every trading surface with a small
	// random market walk. This keeps the showcase feeling like a real trading
	// terminal while remaining deterministic enough to be easy to understand.
	a := app.New(root, theme)
	a.QuitKeys = []rune{'q'}

	go func() {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		mid := 7890000
		for tick := 0; ; tick++ {
			time.Sleep(350 * time.Millisecond)
			mid += rng.Intn(81) - 40

			trade := widget.Trade{
				Price: uint64(mid + rng.Intn(120) - 60),
				Size:  uint64(1 + rng.Intn(25)),
				Buy:   rng.Intn(2) == 0,
			}
			trades.AddTrade(trade)

			btcMark := mid
			btcPanel.SetData(widget.PositionPanelData{
				Symbol: "BTC-PERP", Side: "LONG", Quantity: "2", Entry: "78,100", Mark: fmt.Sprintf("%.2f", float64(btcMark)/100.0), ROI: "+1.02%", Leverage: "10x", PnL: fmt.Sprintf("%+.2f", float64(int64(btcMark)-7810000)*2/100.0), TPPrice: "79,400", TPOrder: "READY",
			})
			ethMark := 321000 + rng.Intn(8000)
			ethPanel.SetData(widget.PositionPanelData{
				Symbol: "ETH-PERP", Side: "SHORT", Quantity: "4", Entry: "3,250", Mark: fmt.Sprintf("%.2f", float64(ethMark)/100.0), ROI: "+1.23%", Leverage: "8x", PnL: fmt.Sprintf("%+.2f", float64(rng.Intn(24000)-9000)/100.0), TPPrice: "3,150", TPOrder: "READY",
			})
			solMark := 14800 + rng.Intn(700)
			solPanel.SetData(widget.PositionPanelData{
				Symbol: "SOL-PERP", Side: "LONG", Quantity: "12", Entry: "148.00", Mark: fmt.Sprintf("%.2f", float64(solMark)/100.0), ROI: "+2.03%", Leverage: "5x", PnL: fmt.Sprintf("%+.2f", float64(rng.Intn(9000)-2500)/100.0), TPPrice: "154.00", TPOrder: "READY",
			})

			orders.SetRows([]widget.Order{
				{ID: fmt.Sprintf("A%03d", 102+tick%8), Symbol: "BTC-PERP", Side: "BUY", Status: "OPEN", Price: uint64(mid - rng.Intn(600)), Qty: uint64(1 + rng.Intn(5)), Filled: uint64(rng.Intn(3))},
				{ID: fmt.Sprintf("A%03d", 110+tick%8), Symbol: "ETH-PERP", Side: "SELL", Status: "PARTIAL", Price: uint64(322500 + rng.Intn(5000)), Qty: uint64(4 + rng.Intn(8)), Filled: uint64(rng.Intn(4))},
			})

			pnl.Realized.Store(int64(125000 + rng.Intn(8000)))
			pnl.Unrealized.Store(int64(rng.Intn(90000) - 20000))
			pnl.Fees.Store(int64(1800 + rng.Intn(400)))
			latency.Stats.Last.Store(uint64(180 + rng.Intn(620)))
			latency.Stats.Min.Store(uint64(120 + rng.Intn(120)))
			latency.Stats.Max.Store(uint64(700 + rng.Intn(800)))
			latency.Stats.Count.Store(uint64(250000 + tick))
			risk.State.Utilization.Store(int64(25 + rng.Intn(55)))
			risk.State.DailyLoss.Store(int64(rng.Intn(5000)))
			risk.State.Drawdown.Store(int64(rng.Intn(8000)))

			if rng.Intn(10) == 0 {
				status.SetState(widget.MarketHalted)
			} else {
				status.SetState(widget.MarketLive)
			}

			a.BeginBatch()
			a.InvalidateWidgets(trades, positions, orders, pnl, latency, risk, status)
			a.EndBatch()
		}
	}()

	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
}
