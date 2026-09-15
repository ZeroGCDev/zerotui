package app

import (
	"io"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
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

type glyphPaint struct {
	ch rune
}

func (g *glyphPaint) Draw(buf *buffer.Buffer, area geometry.Rect, _ *style.Theme) {
	if area.W > 0 && area.H > 0 {
		x, y := area.X+area.W/2, area.Y+area.H/2
		st := buf.CellAt(x, y).Style
		buf.Set(x, y, g.ch, st)
	}
}

func TestTargetedDamageRecomposesNonOpaqueOverlayBackdrop(t *testing.T) {
	base := &solidPaint{st: style.Style{Bg: color.RGB(10, 20, 30)}, ch: 'B'}
	top := &glyphPaint{ch: 'T'}
	visible := true
	overlay := layout.NewOverlay(func() bool { return visible }, layout.Wrap(top))
	root := layout.NewStack(layout.Wrap(base), overlay)
	a := &App{Root: root, Theme: style.TokyoNightTheme(), width: 40, height: 20, placements: make([]layout.Placement, 0, 4)}
	a.buf = buffer.New(40, 20)
	a.relayout()
	a.dirty.Store(true)
	a.drawTo(io.Discard)

	// The backdrop dims the already-painted base. A targeted child repaint must
	// replay that backdrop after ClearRegion rather than exposing Theme.Background.
	before := a.buf.CellAt(20, 10)
	if before.Ch != 'T' {
		t.Fatalf("overlay child not painted: %+v", before)
	}
	baseCell := a.buf.CellAt(2, 2)
	if baseCell.Ch != 'B' || baseCell.Style.Attr&style.Dim == 0 {
		t.Fatalf("initial dim backdrop missing: %+v", baseCell)
	}

	top.ch = 'X'
	a.InvalidateWidgets(top)
	a.drawTo(io.Discard)
	after := a.buf.CellAt(20, 10)
	if after.Ch != 'X' {
		t.Fatalf("targeted overlay repaint failed: %+v", after)
	}
	if after.Style.Attr&style.Dim == 0 {
		t.Fatalf("targeted overlay repaint lost backdrop dim: %+v", after)
	}
}
