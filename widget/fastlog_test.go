package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

func BenchmarkFastLogViewport10000(b *testing.B) {
	logv := NewFastLogView(10000)
	for i := 0; i < 10000; i++ {
		logv.Append("INFO websocket price tick 78900.25")
	}
	buf := buffer.New(120, 40)
	th := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 120, H: 40}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logv.Draw(buf, area, th)
	}
}
