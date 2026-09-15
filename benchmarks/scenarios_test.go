package benchmarks

import (
	"io"
	"strconv"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func tradingDashboardRoot() layout.Node {
	var price uint64 = 78_900_000
	ticker := widget.NewPriceTicker("BTC-PERP", &price, 6, 2)
	spark := widget.NewSparkline(200)
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(32)
	book.SetLevels(bids, asks)
	cols := []widget.Column{{Title: "INDEX", Width: 8}, {Title: "SYMBOL", Width: 12}, {Title: "PRICE", Width: 14, Align: widget.AlignRight}, {Title: "STATUS", Width: 10}}
	table := widget.NewVirtualTable(cols, 100000, func(row, col int) string {
		switch col {
		case 0:
			return strconv.Itoa(row)
		case 1:
			return "BTC-PERP"
		case 2:
			return strconv.Itoa(78000 + row%1000)
		default:
			return "ACTIVE"
		}
	})
	table.ShowScrollBar = true
	market := layout.BorderedRounded("MARKET", layout.NewFlex(layout.Vertical, layout.Fix(layout.Wrap(ticker), 1), layout.Fix(layout.Wrap(spark), 1), layout.Flex1(layout.Wrap(book))), nil)
	data := layout.BorderedRounded("DATA", layout.Wrap(table), nil)
	return layout.NewFlex(layout.Vertical, layout.FlexN(market, 2), layout.FlexN(data, 1))
}

func BenchmarkScenarioTradingDashboardLayoutDraw(b *testing.B) {
	root := tradingDashboardRoot()
	placements := make([]layout.Placement, 0, 64)
	buf := buffer.New(160, 48)
	theme := style.TokyoNightTheme()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		placements = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, placements)
		for _, p := range placements {
			p.Widget.Draw(buf, p.Area, theme)
		}
		_, _ = buf.Render(io.Discard)
	}
	sinkInt = len(placements)
}
func BenchmarkScenarioTradingDashboardResize(b *testing.B) {
	root := tradingDashboardRoot()
	placements := make([]layout.Placement, 0, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		placements = layout.ComputeInto(root, geometry.Rect{W: 100 + i%121, H: 30 + i%31}, placements)
	}
	sinkInt = len(placements)
}
func BenchmarkScenarioLargeVirtualTable(b *testing.B) {
	cols := []widget.Column{{Title: "INDEX", Width: 8}, {Title: "SYMBOL", Width: 12}, {Title: "PRICE", Width: 14, Align: widget.AlignRight}, {Title: "VOLUME", Width: 12, Align: widget.AlignRight}, {Title: "STATUS", Width: 10}}
	t := widget.NewVirtualTable(cols, 1_000_000, func(row, col int) string {
		switch col {
		case 0:
			return strconv.Itoa(row)
		case 1:
			return "BTC-PERP"
		case 2:
			return strconv.Itoa(78000 + row%1000)
		case 3:
			return strconv.Itoa((row * 89) % 15000)
		default:
			return "ACTIVE"
		}
	})
	t.ShowScrollBar = true
	t.Zebra = true
	benchWidgetDraw(b, t, geometry.Rect{X: 0, Y: 0, W: 100, H: 40})
}
func BenchmarkScenarioNestedLayout(b *testing.B) {
	leaf := layout.Wrap(widget.NewLabel("node"))
	root := layout.Bordered("root", layout.NewStack(
		layout.NewFlex(layout.Horizontal, layout.Flex1(layout.NewGrid(4, 4, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf, leaf)), layout.Flex1(layout.Center(layout.Wrap(widget.NewLabel("center")), .7, .7))),
		layout.NewFlex(layout.Vertical, layout.Flex1(leaf), layout.Flex1(leaf)),
	), nil)
	dst := make([]layout.Placement, 0, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkScenarioPartialWidgetUpdate(b *testing.B) {
	var price uint64 = 78900000000
	ticker := widget.NewPriceTicker("BTC-PERP", &price, 9, 2)
	buf := buffer.New(160, 48)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 30, H: 1}
	ticker.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)
	regions := []buffer.Rect{{X: 0, Y: 0, W: 30, H: 1}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		price++
		ticker.Draw(buf, area, theme)
		_, _ = buf.RenderRegions(io.Discard, regions)
	}
}
func BenchmarkScenarioFullDashboardRedraw(b *testing.B) {
	root := tradingDashboardRoot()
	placements := make([]layout.Placement, 0, 64)
	buf := buffer.New(160, 48)
	theme := style.TokyoNightTheme()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear(theme.Background)
		placements = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, placements)
		for _, p := range placements {
			p.Widget.Draw(buf, p.Area, theme)
		}
		_, _ = buf.Render(io.Discard)
	}
	sinkInt = len(placements)
}
func BenchmarkScenarioHighFrequencyMarketUpdate(b *testing.B) {
	var price uint64 = 78900000000
	ticker := widget.NewPriceTicker("BTC-PERP", &price, 9, 2)
	spark := widget.NewSparkline(200)
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(32)
	book.SetLevels(bids, asks)
	buf := buffer.New(160, 48)
	theme := style.TokyoNightTheme()
	ticker.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 30, H: 1}, theme)
	spark.Draw(buf, geometry.Rect{X: 0, Y: 1, W: 120, H: 1}, theme)
	book.Draw(buf, geometry.Rect{X: 0, Y: 2, W: 120, H: 30}, theme)
	_, _ = buf.Render(io.Discard)
	regions := []buffer.Rect{{X: 0, Y: 0, W: 120, H: 32}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		price++
		bids[i%32].Size++
		asks[(i+7)%32].Size++
		spark.Push(float64(price))
		book.SetLevels(bids, asks)
		ticker.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 30, H: 1}, theme)
		spark.Draw(buf, geometry.Rect{X: 0, Y: 1, W: 120, H: 1}, theme)
		book.Draw(buf, geometry.Rect{X: 0, Y: 2, W: 120, H: 30}, theme)
		_, _ = buf.RenderRegions(io.Discard, regions)
	}
}
