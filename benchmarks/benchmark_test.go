package benchmarks

import (
	"io"
	"strconv"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func benchmarkLevels(n int) ([]widget.Level, []widget.Level) {
	bids := make([]widget.Level, n)
	asks := make([]widget.Level, n)
	for i := 0; i < n; i++ {
		bids[i] = widget.Level{Price: uint64(100_000_000_000 - i*100), Size: uint64(1000 + i*7)}
		asks[i] = widget.Level{Price: uint64(100_000_000_100 + i*100), Size: uint64(900 + i*9)}
	}
	return bids, asks
}

func benchmarkBook(b *testing.B, levels, changedRows int) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(levels)
	book.SetLevels(bids, asks)
	buf := buffer.New(120, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 120, H: 60}
	book.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := i % levels
		bids[row].Size++
		book.SetLevels(bids, asks)
		buf.SetClip(buffer.Rect{X: 0, Y: 1, W: 120, H: changedRows})
		book.Draw(buf, area, theme)
		buf.ClearClip()
		_, _ = buf.RenderRegions(io.Discard, []buffer.Rect{{X: 0, Y: 1, W: 120, H: changedRows}})
	}
}

func BenchmarkOrderBookTick(b *testing.B) {
	for _, levels := range []int{10, 25, 50, 100} {
		b.Run(strconv.Itoa(levels), func(b *testing.B) { benchmarkBook(b, levels, 1) })
	}
}

func BenchmarkOrderBookTenLevelUpdate(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(100)
	book.SetLevels(bids, asks)
	buf := buffer.New(120, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 120, H: 60}
	book.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			bids[j].Size += uint64(i%13 + 1)
		}
		book.SetLevels(bids, asks)
		buf.SetClip(buffer.Rect{X: 0, Y: 1, W: 120, H: 10})
		book.Draw(buf, area, theme)
		buf.ClearClip()
		_, _ = buf.RenderRegions(io.Discard, []buffer.Rect{{X: 0, Y: 1, W: 120, H: 10}})
	}
}

func BenchmarkOrderBookBestBidAsk(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(50)
	book.SetLevels(bids, asks)
	buf := buffer.New(120, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 120, H: 60}
	book.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bids[0].Price++
		asks[0].Price++
		book.SetLevels(bids, asks)
		buf.SetClip(buffer.Rect{X: 0, Y: 1, W: 120, H: 1})
		book.Draw(buf, area, theme)
		buf.ClearClip()
		_, _ = buf.RenderRegions(io.Discard, []buffer.Rect{{X: 0, Y: 1, W: 120, H: 1}})
	}
}

func BenchmarkOrderBookFullRefresh(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(100)
	book.SetLevels(bids, asks)
	buf := buffer.New(120, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 120, H: 60}
	book.Draw(buf, area, theme)
	_, _ = buf.Render(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range bids {
			bids[j].Price++
			asks[j].Price++
			bids[j].Size += uint64(i & 3)
		}
		book.SetLevels(bids, asks)
		book.Draw(buf, area, theme)
		_, _ = buf.Render(io.Discard)
	}
}

func BenchmarkOrderBookResize(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(50)
	book.SetLevels(bids, asks)
	buf := buffer.New(140, 70)
	theme := style.TokyoNightTheme()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i&1 == 0 {
			buf.Resize(120, 60)
			book.Draw(buf, geometry.Rect{W: 120, H: 60}, theme)
		} else {
			buf.Resize(140, 70)
			book.Draw(buf, geometry.Rect{W: 140, H: 70}, theme)
		}
		_, _ = buf.Render(io.Discard)
	}
}

func BenchmarkOrderBookThemeChange(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(50)
	book.SetLevels(bids, asks)
	buf := buffer.New(120, 60)
	themes := []*style.Theme{style.TokyoNightTheme(), style.CatppuccinMochaTheme()}
	area := geometry.Rect{W: 120, H: 60}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		book.Draw(buf, area, themes[i&1])
		_, _ = buf.Render(io.Discard)
	}
}

func BenchmarkOrderBookExplicitBackground(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bg := color.RGB(10, 10, 20)
	book.Background = &bg
	bids, asks := benchmarkLevels(50)
	book.SetLevels(bids, asks)
	buf := buffer.New(120, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 120, H: 60}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		book.Draw(buf, area, theme)
	}
}

func BenchmarkRendererSparseCell(b *testing.B) {
	buf := buffer.New(120, 40)
	st := style.Style{Fg: color.White, Bg: color.Black}
	buf.SetString(20, 20, "A", st)
	_, _ = buf.Render(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i&1 == 0 {
			buf.SetString(20, 20, "A", st)
		} else {
			buf.SetString(20, 20, "B", st)
		}
		_, _ = buf.RenderRegions(io.Discard, []buffer.Rect{{X: 20, Y: 20, W: 1, H: 1}})
	}
}

func BenchmarkRendererFullFrame(b *testing.B) {
	buf := buffer.New(120, 40)
	st := style.Style{Fg: color.White, Bg: color.Black}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear(st)
		_, _ = buf.Render(io.Discard)
	}
}

func BenchmarkRendererSynchronizedSparse(b *testing.B) {
	buf := buffer.New(120, 40)
	st := style.Style{Fg: color.White, Bg: color.Black}
	buf.SetString(20, 20, "A", st)
	_, _ = buf.RenderSynchronized(io.Discard)
	region := []buffer.Rect{{X: 20, Y: 20, W: 1, H: 1}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i&1 == 0 {
			buf.SetString(20, 20, "A", st)
		} else {
			buf.SetString(20, 20, "B", st)
		}
		_, _ = buf.RenderRegionsSynchronized(io.Discard, region)
	}
}

func BenchmarkResponsiveTradingFrame(b *testing.B) {
	price := uint64(78_900_000)
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(32)
	book.SetLevels(bids, asks)
	ticker := widget.NewPriceTicker("BTC-PERP", &price, 2, 2)
	root := layout.NewFlex(layout.Horizontal,
		layout.FlexN(layout.Wrap(ticker), 1),
		layout.FlexN(layout.Wrap(book), 2),
	)
	placements := make([]layout.Placement, 0, 8)
	buf := buffer.New(160, 48)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 160, H: 48}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		price += uint64(i & 3)
		bids[i%len(bids)].Size++
		book.SetLevels(bids, asks)
		placements = layout.ComputeInto(root, area, placements)
		for _, p := range placements {
			p.Widget.Draw(buf, p.Area, theme)
		}
		_, _ = buf.Render(io.Discard)
	}
}

func BenchmarkResponsiveReflowReuse(b *testing.B) {
	label := widget.NewLabel("ZeroTUI")
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(32)
	book.SetLevels(bids, asks)
	root := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(label), 1),
		layout.Flex1(layout.Wrap(book)),
	)
	placements := make([]layout.Placement, 0, 8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		placements = layout.ComputeInto(root, geometry.Rect{W: 80 + i%81, H: 24 + i%25}, placements)
	}
}
