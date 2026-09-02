package benchmarks

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func benchmarkApp() (*app.App, []widget.Widget) {
	var widgets []widget.Widget
	for i := 0; i < 32; i++ {
		widgets = append(widgets, widget.NewLabel("label"))
	}
	items := make([]layout.Item, 0, len(widgets))
	for _, w := range widgets {
		items = append(items, layout.Flex1(layout.Wrap(w)))
	}
	root := layout.NewFlex(layout.Vertical, items...)
	a := app.New(root, style.TokyoNightTheme())
	a.Relayout()
	return a, widgets
}
func BenchmarkAppRelayout(b *testing.B) {
	a, _ := benchmarkApp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Relayout()
	}
}
func BenchmarkAppInvalidate(b *testing.B) {
	a, _ := benchmarkApp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Invalidate()
	}
}
func BenchmarkAppInvalidateRect(b *testing.B) {
	a, _ := benchmarkApp()
	r := geometry.Rect{X: 10, Y: 10, W: 20, H: 4}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.InvalidateRect(r)
	}
}
func BenchmarkAppInvalidateWidgets(b *testing.B) {
	a, ws := benchmarkApp()
	targets := []widget.Widget{ws[0], ws[7], ws[15], ws[23]}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.InvalidateWidgets(targets...)
	}
}
func BenchmarkAppFocus(b *testing.B) {
	var v uint32 = 1
	f := widget.NewToggle("focus", &v)
	a := app.New(layout.Wrap(f), style.TokyoNightTheme())
	a.Relayout()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = a.Focus(f)
	}
}
func BenchmarkAppLiveRect(b *testing.B) {
	a, _ := benchmarkApp()
	r := geometry.Rect{X: 20, Y: 20, W: 10, H: 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok := a.RequestLiveRect(r)
		if ok {
			a.DropLiveRect(r)
		}
		sinkBool = ok
	}
}
func BenchmarkAppLiveReference(b *testing.B) {
	a, _ := benchmarkApp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.RequestLive()
		a.DropLive()
	}
}
func BenchmarkAppInteractiveToggle(b *testing.B) {
	a, _ := benchmarkApp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.SetInteractive(i&1 == 0)
	}
}
func BenchmarkAppRetainedState(b *testing.B) {
	a, _ := benchmarkApp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total, dirty := a.RetainedState()
		sinkInt = total + dirty
	}
}
