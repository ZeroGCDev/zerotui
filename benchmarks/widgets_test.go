package benchmarks

import (
	"io"
	"strconv"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func benchWidgetDraw(b *testing.B, w widget.Widget, area geometry.Rect) {
	buf := buffer.New(160, 60)
	theme := style.TokyoNightTheme()
	w.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Draw(buf, area, theme)
	}
}

func BenchmarkWidgetLabelDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewLabel("ZeroTUI — high-frequency retained UI"), geometry.Rect{X: 1, Y: 1, W: 80, H: 1})
}
func BenchmarkWidgetButtonDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewButton("TRIGGER SIGNAL", nil), geometry.Rect{X: 1, Y: 1, W: 30, H: 1})
}
func BenchmarkWidgetToggleDraw(b *testing.B) {
	var v uint32 = 1
	benchWidgetDraw(b, widget.NewToggle("Simulation", &v), geometry.Rect{X: 1, Y: 1, W: 30, H: 1})
}
func BenchmarkWidgetSliderDraw(b *testing.B) {
	var v uint32 = 25
	benchWidgetDraw(b, widget.NewSlider("Leverage", &v, 1, 50, 1, widget.FormatInt("x")), geometry.Rect{X: 1, Y: 1, W: 50, H: 1})
}
func BenchmarkWidgetTextInputDraw(b *testing.B) {
	w := widget.NewTextInput("Filter symbol")
	w.SetValue("BTC-PERP")
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 40, H: 3})
}
func BenchmarkWidgetGaugeDraw(b *testing.B) {
	w := widget.NewGauge("CPU")
	w.Value = .73
	w.WarnAt = .7
	w.DangerAt = .9
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 60, H: 1})
}
func BenchmarkWidgetTabsDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewTabs([]string{"Overview", "Positions", "Orders", "Trades"}), geometry.Rect{X: 1, Y: 1, W: 70, H: 1})
}
func BenchmarkWidgetListDraw(b *testing.B) {
	items := make([]string, 500)
	for i := range items {
		items[i] = "Position #" + strconv.Itoa(i)
	}
	w := widget.NewList(items)
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 50, H: 24})
}
func BenchmarkWidgetTableDraw(b *testing.B) {
	cols := []widget.Column{{Title: "INDEX", Width: 8}, {Title: "SYMBOL", Width: 12}, {Title: "PRICE", Width: 14, Align: widget.AlignRight}, {Title: "STATUS", Width: 10}}
	rows := make([][]string, 500)
	for i := range rows {
		rows[i] = []string{strconv.Itoa(i), "BTC-PERP", strconv.Itoa(78000 + i), "ACTIVE"}
	}
	w := widget.NewTable(cols)
	w.Rows = rows
	w.Zebra = true
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 60, H: 24})
}
func BenchmarkWidgetVirtualListDraw(b *testing.B) {
	w := widget.NewVirtualList(1_000_000, func(i int) string { return "virtual-row" })
	w.ShowScrollBar = true
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 60, H: 24})
}
func BenchmarkWidgetVirtualTableDraw(b *testing.B) {
	cols := []widget.Column{{Title: "INDEX", Width: 8}, {Title: "SYMBOL", Width: 12}, {Title: "PRICE", Width: 14, Align: widget.AlignRight}, {Title: "STATUS", Width: 10}}
	w := widget.NewVirtualTable(cols, 1_000_000, func(row, col int) string {
		switch col {
		case 0:
			return "000000"
		case 1:
			return "BTC-PERP"
		case 2:
			return "78900.00"
		default:
			return "ACTIVE"
		}
	})
	w.ShowScrollBar = true
	w.Zebra = true
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 60, H: 24})
}
func BenchmarkWidgetPriceTickerDraw(b *testing.B) {
	var price uint64 = 78900250000
	w := widget.NewPriceTicker("BTC-PERP", &price, 9, 2)
	area := geometry.Rect{X: 1, Y: 1, W: 40, H: 1}
	buf := buffer.New(160, 60)
	theme := style.TokyoNightTheme()
	w.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		price++
		w.Draw(buf, area, theme)
	}
}
func BenchmarkWidgetSparklineDraw(b *testing.B) {
	w := widget.NewSparkline(200)
	for i := 0; i < 200; i++ {
		w.Push(float64(i))
	}
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 120, H: 1})
}
func BenchmarkWidgetOrderBookDraw(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(100)
	book.SetLevels(bids, asks)
	benchWidgetDraw(b, book, geometry.Rect{X: 1, Y: 1, W: 120, H: 40})
}
func BenchmarkWidgetFastLogDraw(b *testing.B) {
	w := widget.NewFastLogView(1000)
	for i := 0; i < 1000; i++ {
		w.Append("2026-09-02T18:00:00Z INFO market update BTC-PERP")
	}
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 120, H: 30})
}
func BenchmarkWidgetPanelDraw(b *testing.B) {
	child := widget.NewLabel("Inside retained panel")
	w := widget.NewPanel("MARKET", child)
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 80, H: 10})
}
func BenchmarkWidgetBadgeDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewBadge("LIVE"), geometry.Rect{X: 1, Y: 1, W: 12, H: 1})
}
func BenchmarkWidgetDividerDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewDivider(true), geometry.Rect{X: 1, Y: 1, W: 80, H: 1})
}
func BenchmarkWidgetStatDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewStat("PnL", "+12.48%"), geometry.Rect{X: 1, Y: 1, W: 30, H: 2})
}
func BenchmarkWidgetSpinnerDraw(b *testing.B) {
	w := widget.NewSpinner("Loading")
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 30, H: 1})
}
func BenchmarkWidgetScrollBarDraw(b *testing.B) {
	w := widget.ScrollBar{Total: 100000, Offset: 5000, Viewport: 24}
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 1, H: 24})
}
func BenchmarkWidgetGradientBarDraw(b *testing.B) {
	w := widget.NewGradientBar(geometryColor(20, 200, 120), geometryColor(200, 80, 80), geometryColor(20, 20, 30))
	benchWidgetDraw(b, w, geometry.Rect{X: 1, Y: 1, W: 80, H: 1})
}
func geometryColor(r, g, bb uint8) (c color.Color) { return color.RGB(r, g, bb) }

func BenchmarkWidgetKeyHandling(b *testing.B) {
	var v uint32 = 1
	w := widget.NewToggle("Sim", &v)
	w.Focus(true)
	k := input.Key{Type: input.KeySpace}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetSliderKeyHandling(b *testing.B) {
	var v uint32 = 25
	w := widget.NewSlider("L", &v, 1, 50, 1, widget.FormatInt("x"))
	w.Focus(true)
	k := input.Key{Type: input.KeyRight}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetTextInputKeyHandling(b *testing.B) {
	w := widget.NewTextInput("symbol")
	w.Focus(true)
	k := input.Key{Type: input.KeyRune, Rune: 'X'}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetMouseHandling(b *testing.B) {
	var v uint32
	w := widget.NewToggle("Sim", &v)
	w.Focus(true)
	ev := input.MouseEvent{X: 5, Y: 1, Button: input.MouseLeft, Action: input.MousePress}
	area := geometry.Rect{X: 0, Y: 0, W: 30, H: 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleMouse(ev, area)
	}
}

func BenchmarkWidgetListKeyHandling(b *testing.B) {
	items := make([]string, 1000)
	w := widget.NewList(items)
	w.Focus(true)
	k := input.Key{Type: input.KeyDown}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetVirtualListKeyHandling(b *testing.B) {
	w := widget.NewVirtualList(1_000_000, func(int) string { return "row" })
	w.Focus(true)
	k := input.Key{Type: input.KeyDown}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetTableKeyHandling(b *testing.B) {
	w := widget.NewTable([]widget.Column{{Title: "A", Width: 10}})
	w.Rows = make([][]string, 1000)
	w.Focus(true)
	k := input.Key{Type: input.KeyDown}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetVirtualTableKeyHandling(b *testing.B) {
	w := widget.NewVirtualTable([]widget.Column{{Title: "A", Width: 10}}, 1_000_000, func(int, int) string { return "row" })
	w.Focus(true)
	k := input.Key{Type: input.KeyDown}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleKey(k)
	}
}
func BenchmarkWidgetVirtualListMouseScroll(b *testing.B) {
	w := widget.NewVirtualList(1_000_000, func(int) string { return "row" })
	w.ShowScrollBar = true
	ev := input.MouseEvent{X: 59, Y: 12, Button: input.MouseNone, Action: input.MouseWheelDown}
	area := geometry.Rect{X: 0, Y: 0, W: 60, H: 24}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleMouse(ev, area)
	}
}
func BenchmarkWidgetVirtualTableMouseScroll(b *testing.B) {
	w := widget.NewVirtualTable([]widget.Column{{Title: "A", Width: 10}}, 1_000_000, func(int, int) string { return "row" })
	w.ShowScrollBar = true
	ev := input.MouseEvent{X: 59, Y: 12, Button: input.MouseNone, Action: input.MouseWheelDown}
	area := geometry.Rect{X: 0, Y: 0, W: 60, H: 24}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleMouse(ev, area)
	}
}
func BenchmarkWidgetResizeHandleMouse(b *testing.B) {
	r := .5
	w := widget.NewResizeHandle(widget.ResizeVertical, &r)
	area := geometry.Rect{X: 20, Y: 0, W: 1, H: 24}
	ev := input.MouseEvent{X: 20, Y: 10, Button: input.MouseLeft, Action: input.MouseDrag}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = w.HandleMouse(ev, area)
	}
}
