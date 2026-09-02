package benchmarks

import (
	"strconv"
	"testing"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/numfmt"
)

func BenchmarkNumfmtAppendUint(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		dst = numfmt.AppendUint(dst, uint64(789000000+i))
		sinkInt += len(dst)
	}
}
func BenchmarkNumfmtAppendInt(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		dst = numfmt.AppendInt(dst, int64(i)-500000)
		sinkInt += len(dst)
	}
}
func BenchmarkNumfmtAppendFixed(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		dst = numfmt.AppendFixed(dst, 7890012345+uint64(i), 9)
		sinkInt += len(dst)
	}
}
func BenchmarkNumfmtAppendFixedPrec(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		dst = numfmt.AppendFixedPrec(dst, 7890012345+uint64(i), 9, 2)
		sinkInt += len(dst)
	}
}
func BenchmarkNumfmtPadLeft(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		dst = numfmt.AppendUint(dst, uint64(i))
		dst = numfmt.PadLeft(dst, 0, 16, ' ')
		sinkInt += len(dst)
	}
}
func BenchmarkNumfmtStrconvReference(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		dst = strconv.AppendUint(dst, uint64(789000000+i), 10)
		sinkInt += len(dst)
	}
}
func BenchmarkColorRGB(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	var c color.Color
	for i := 0; i < b.N; i++ {
		c = color.RGB(uint8(i), uint8(i>>8), uint8(i>>16))
		sinkInt = int(c)
	}
}
func BenchmarkColorComponents(b *testing.B) {
	c := color.RGB(100, 150, 200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, g, bb := c.Components()
		sinkInt = int(r) + int(g) + int(bb)
	}
}
func BenchmarkColorLerp(b *testing.B) {
	a := color.RGB(10, 20, 30)
	z := color.RGB(220, 180, 140)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInt = int(color.Lerp(a, z, uint8(i)))
	}
}
