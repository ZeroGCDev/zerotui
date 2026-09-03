package app

import (
	"io"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func TestTargetedChildDamageDoesNotEraseSiblingWidgets(t *testing.T) {
	var price uint64 = 78900000000000
	ticker := widget.NewPriceTicker("BTC-PERP", &price, 9, 2)
	spark := widget.NewSparkline(40)
	spark.Push(78900)
	spark.Push(78901)
	book := widget.NewOrderBook(9, 3, 2)
	book.SetLevels(
		[]widget.Level{{Price: 78899, Size: 1000}},
		[]widget.Level{{Price: 78901, Size: 2000}},
	)

	market := layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(ticker), 2),
		layout.Fix(layout.Wrap(spark), 3),
		layout.Flex1(layout.Wrap(book)),
	)
	root := layout.BorderedRounded("LIVE MARKET", market, nil)

	a := &App{
		Root:       root,
		Theme:      style.NordTheme(),
		width:      80,
		height:     24,
		placements: make([]layout.Placement, 0, 8),
	}
	a.buf = buffer.New(80, 24)
	a.relayout()
	a.dirty.Store(true)
	a.drawTo(io.Discard)

	if got := a.buf.CellAt(1, 1).Ch; got != 'B' {
		t.Fatalf("initial ticker missing: got %q", got)
	}
	if !hasNonSpace(a.buf, geometry.Rect{X: 1, Y: 4, W: 78, H: 1}) {
		t.Fatal("initial sparkline did not render")
	}

	book.SetLevels(
		[]widget.Level{{Price: 78900, Size: 3000}},
		[]widget.Level{{Price: 78902, Size: 2500}},
	)
	a.InvalidateWidgets(book)
	a.drawTo(io.Discard)

	if got := a.buf.CellAt(1, 1).Ch; got != 'B' {
		t.Fatalf("order book partial redraw erased ticker: got %q", got)
	}
	if !hasNonSpace(a.buf, geometry.Rect{X: 1, Y: 4, W: 78, H: 1}) {
		t.Fatal("order book partial redraw erased sparkline")
	}
}

func hasNonSpace(buf *buffer.Buffer, area geometry.Rect) bool {
	for y := area.Y; y < area.Y+area.H; y++ {
		for x := area.X; x < area.X+area.W; x++ {
			if ch := buf.CellAt(x, y).Ch; ch != ' ' && ch != 0 {
				return true
			}
		}
	}
	return false
}

func TestIntersectRect(t *testing.T) {
	got := intersectRect(geometry.Rect{X: 2, Y: 3, W: 10, H: 8}, geometry.Rect{X: 7, Y: 1, W: 8, H: 5})
	want := geometry.Rect{X: 7, Y: 3, W: 5, H: 3}
	if got != want {
		t.Fatalf("intersection=%v want=%v", got, want)
	}
}
