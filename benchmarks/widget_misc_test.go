package benchmarks

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/widget"
)

func BenchmarkWidgetCloseButtonDraw(b *testing.B) {
	benchWidgetDraw(b, widget.NewCloseButton(nil), geometry.Rect{X: 1, Y: 1, W: 3, H: 1})
}
func BenchmarkWidgetResizeHandleDraw(b *testing.B) {
	ratio := .5
	benchWidgetDraw(b, widget.NewResizeHandle(widget.ResizeHorizontal, &ratio), geometry.Rect{X: 1, Y: 1, W: 3, H: 1})
}
func BenchmarkWidgetCommandPaletteDraw(b *testing.B) {
	cmds := make([]widget.Command, 100)
	for i := range cmds {
		cmds[i] = widget.Command{Name: "command-" + itoa(i)}
	}
	p := widget.NewCommandPalette(cmds)
	benchWidgetDraw(b, p, geometry.Rect{X: 1, Y: 1, W: 70, H: 20})
}
func BenchmarkWidgetCommandPaletteQuery(b *testing.B) {
	cmds := make([]widget.Command, 100)
	for i := range cmds {
		cmds[i] = widget.Command{Name: "command-" + itoa(i)}
	}
	p := widget.NewCommandPalette(cmds)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SetQuery("command-" + itoa(i%100))
	}
}
func itoa(i int) string {
	// Kept separate from strconv in benchmark names so command-palette
	// filtering cost remains visible without obscuring the benchmark body.
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var a [12]byte
	n := len(a)
	for i > 0 {
		n--
		a[n] = digits[i%10]
		i /= 10
	}
	return string(a[n:])
}
