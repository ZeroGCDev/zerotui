package benchmarks

import (
	"io"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func BenchmarkConcurrencySparklinePush(b *testing.B) {
	s := widget.NewSparkline(200)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Push(float64(i % 200))
			i++
		}
	})
}
func BenchmarkConcurrencySparklinePushDraw(b *testing.B) {
	s := widget.NewSparkline(200)
	for i := 0; i < 200; i++ {
		s.Push(float64(i))
	}
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 120, H: 1}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		buf := buffer.New(120, 2)
		for pb.Next() {
			s.Push(100)
			s.Draw(buf, area, theme)
		}
	})
}
func BenchmarkConcurrencyOrderBookSetLevels(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(50)
	book.SetLevels(bids, asks)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		lb, la := benchmarkLevels(50)
		for pb.Next() {
			lb[0].Size++
			book.SetLevels(lb, la)
		}
	})
}
func BenchmarkConcurrencyOrderBookUpdateDraw(b *testing.B) {
	book := widget.NewOrderBook(2, 3, 2)
	bids, asks := benchmarkLevels(50)
	book.SetLevels(bids, asks)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 120, H: 30}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		buf := buffer.New(120, 30)
		for pb.Next() {
			book.SetLevels(bids, asks)
			book.Draw(buf, area, theme)
			_, _ = buf.RenderRegions(io.Discard, []buffer.Rect{{X: 0, Y: 0, W: 120, H: 30}})
		}
	})
}
