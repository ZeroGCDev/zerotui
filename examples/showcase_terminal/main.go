package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.DraculaTheme()
	var price uint64 = 102_450_000_000

	ticker := widget.NewPriceTicker("BTC-USD", &price, 8, 2)
	book := widget.NewOrderBook(8, 2, 2)
	spark := widget.NewSparkline(60)
	badge := widget.NewBadge("MARKET OPEN")
	badge.Info = true

	orders := widget.NewVirtualTable(
		[]widget.Column{
			{Title: "ID", Width: 8},
			{Title: "SIDE", Width: 8},
			{Title: "PRICE", Width: 14},
			{Title: "SIZE", Width: 12},
			{Title: "STATE", Width: 12},
		}, 500, func(row, col int) string {
			rows := [][]string{
				{"10482", "BUY", "102,448.20", "0.80", "FILLED"},
				{"10483", "SELL", "102,472.10", "1.20", "OPEN"},
				{"10484", "BUY", "102,440.00", "0.35", "OPEN"},
			}
			return rows[row%len(rows)][col]
		})
	orders.Zebra, orders.ShowScrollBar = true, true

	var logs = widget.NewFastLogView(200)
	logs.FollowTail = true
	logs.Append("08:41:02  connected to market stream")
	logs.Append("08:41:03  order book synchronized")
	logs.Append("08:41:04  strategy: momentum-alpha online")

	quote := layout.BorderedRounded("QUOTE", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(badge), 1),
		layout.Fix(layout.Wrap(ticker), 2),
		layout.Fix(layout.Wrap(spark), 4),
		layout.Flex1(layout.Wrap(book)),
	), nil)

	ordersPanel := layout.BorderedRounded("RECENT ORDERS", layout.Wrap(orders), func() bool { return orders.IsFocused() })
	logPanel := layout.BorderedRounded("EVENT STREAM", layout.Wrap(logs), func() bool { return logs.IsFocused() })

	top := layout.NewSplit(layout.Horizontal, quote, ordersPanel, .32)
	root := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("  AURORA TRADING TERMINAL")), 1),
		layout.Flex1(layout.NewSplit(layout.Vertical, top, logPanel, .72)),
		layout.Fix(layout.Wrap(widget.NewLabel("  [t] theme  [q] quit")), 1),
	)

	a := app.New(root, theme)
	a.QuitKeys = []rune{'q'}
	a.OnKey = func(k input.Key) bool {
		if k.Type == input.KeyRune && k.Rune == 't' {
			a.SetTheme(style.NordTheme())
			return true
		}
		return false
	}

	go func() {
		for i := uint64(0); ; i++ {
			time.Sleep(250 * time.Millisecond)
			atomic.AddUint64(&price, uint64((int64(i%7)-3)*10000))
			spark.Push(float64(price) / 1e8)
			logs.Append(fmt.Sprintf("tick %04d  quote updated", i))
			bids := make([]widget.Level, 6)
			asks := make([]widget.Level, 6)
			mid := atomic.LoadUint64(&price)
			for j := range bids {
				bids[j] = widget.Level{Price: mid - uint64(j+1)*1000000, Size: uint64(50+j*13) * 100}
				asks[j] = widget.Level{Price: mid + uint64(j+1)*1000000, Size: uint64(45+j*17) * 100}
			}
			book.SetLevels(bids, asks)
			a.BeginBatch()
			a.InvalidateWidgets(ticker, spark, book, logs)
			a.EndBatch()
		}
	}()

	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
}
