package benchmarks

import (
	"io"
	"strconv"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

var sinkInt int
var sinkBool bool
var sinkRect geometry.Rect
var sinkRects []geometry.Rect

func BenchmarkGeometryContains(b *testing.B) {
	r := geometry.Rect{X: 10, Y: 8, W: 120, H: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = r.Contains(20+i%130, 10+i%50)
	}
}
func BenchmarkGeometryInset(b *testing.B) {
	r := geometry.Rect{X: 10, Y: 8, W: 120, H: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkRect = r.Inset(i & 7)
	}
}
func BenchmarkGeometryInsetXY(b *testing.B) {
	r := geometry.Rect{X: 10, Y: 8, W: 120, H: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkRect = r.InsetXY(i&3, i&5)
	}
}
func BenchmarkGeometryRow(b *testing.B) {
	r := geometry.Rect{X: 10, Y: 8, W: 120, H: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkRect = r.Row(i & 39)
	}
}
func BenchmarkGeometrySplitH(b *testing.B) {
	r := geometry.Rect{X: 10, Y: 8, W: 120, H: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, sinkRect = r.SplitH(i % 121)
	}
}
func BenchmarkGeometrySplitV(b *testing.B) {
	r := geometry.Rect{X: 10, Y: 8, W: 120, H: 40}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, sinkRect = r.SplitV(i % 41)
	}
}

func BenchmarkStyleComposition(b *testing.B) {
	s := style.New(color.White, color.Black)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s = s.WithFg(color.RGB(uint8(i), 10, 20)).WithBg(color.RGB(20, 30, uint8(i)))
	}
	sinkInt = int(s.Attr)
}
func BenchmarkStyleAttributes(b *testing.B) {
	s := style.Style{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = s.WithAttr(style.Bold).Attr.Has(style.Bold)
	}
}
func BenchmarkThemeConstruction(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i & 3 {
		case 0:
			_ = style.TokyoNightTheme()
		case 1:
			_ = style.NordTheme()
		case 2:
			_ = style.CatppuccinMochaTheme()
		default:
			_ = style.RosePineTheme()
		}
	}
}

func BenchmarkBufferSetString(b *testing.B) {
	buf := buffer.New(160, 48)
	st := style.Style{Fg: color.White, Bg: color.Black}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetString(10, 10, "BTC-PERP 78900.25", st)
	}
}
func BenchmarkBufferFillRect(b *testing.B) {
	buf := buffer.New(160, 48)
	st := style.Style{Fg: color.White, Bg: color.Black}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.FillRect(0, 0, 80, 20, ' ', st)
	}
}
func BenchmarkBufferRenderRegions(b *testing.B) {
	buf := buffer.New(160, 48)
	st := style.Style{Fg: color.White, Bg: color.Black}
	buf.SetString(20, 20, "A", st)
	_, _ = buf.Render(io.Discard)
	regions := []buffer.Rect{{X: 20, Y: 20, W: 1, H: 1}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetString(20, 20, string(rune('A'+i%2)), st)
		_, _ = buf.RenderRegions(io.Discard, regions)
	}
}
func BenchmarkBufferRenderFull(b *testing.B) {
	buf := buffer.New(160, 48)
	st := style.Style{Fg: color.White, Bg: color.Black}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear(st)
		_, _ = buf.Render(io.Discard)
	}
}
func BenchmarkBufferResize(b *testing.B) {
	buf := buffer.New(160, 48)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i&1 == 0 {
			buf.Resize(80, 24)
		} else {
			buf.Resize(160, 48)
		}
	}
}

func BenchmarkBufferRenderSynchronized(b *testing.B) {
	buf := buffer.New(160, 48)
	st := style.Style{Fg: color.White, Bg: color.Black}
	buf.SetString(1, 1, "X", st)
	_, _ = buf.RenderSynchronized(io.Discard)
	regions := []buffer.Rect{{X: 1, Y: 1, W: 1, H: 1}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetString(1, 1, string(rune('X'+i%2)), st)
		_, _ = buf.RenderRegionsSynchronized(io.Discard, regions)
	}
}

func BenchmarkGeometryBatch(b *testing.B) {
	rs := make([]geometry.Rect, 16)
	r := geometry.Rect{X: 5, Y: 5, W: 100, H: 30}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range rs {
			rs[j] = r.Row((i + j) % 30).Inset(j & 1)
		}
	}
	sinkRects = rs
}

func BenchmarkBufferSetPaddedString(b *testing.B) {
	buf := buffer.New(160, 48)
	st := style.Style{Fg: color.White, Bg: color.Black}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetPaddedString(5, 5, "BTC-PERP", 16, i&1 == 0, st)
	}
}

func BenchmarkGeometryContainsBatch(b *testing.B) {
	r := geometry.Rect{X: 0, Y: 0, W: 160, H: 48}
	hits := 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 128; j++ {
			if r.Contains(j%170, j%60) {
				hits++
			}
		}
	}
	sinkInt = hits
}

var _ = strconv.IntSize
