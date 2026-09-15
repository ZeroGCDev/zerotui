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

func TestTargetedOrderBookDamagePreservesPanelBackground(t *testing.T) {
	book := widget.NewOrderBook(2, 2, 2)
	book.SetLevels([]widget.Level{{Price: 100, Size: 10}}, []widget.Level{{Price: 110, Size: 20}})
	root := layout.Bordered("ORDER BOOK", layout.Wrap(book), nil)
	a := &App{Root: root, Theme: style.TokyoNightTheme(), width: 40, height: 10, placements: make([]layout.Placement, 0, 4)}
	a.buf = buffer.New(40, 10)
	a.relayout()
	a.dirty.Store(true)
	a.drawTo(io.Discard)

	custom := color.RGB(1, 2, 3)
	// Panel remains theme.Panel; the book inherits it.
	if got := a.buf.CellAt(4, 2).Style.Bg; got != a.Theme.Panel.Bg {
		t.Fatalf("initial panel background=%v want=%v", got, a.Theme.Panel.Bg)
	}
	book.SetLevels([]widget.Level{{Price: 101, Size: 10}}, []widget.Level{{Price: 110, Size: 20}})
	a.InvalidateWidgets(book)
	a.drawTo(io.Discard)
	if got := a.buf.CellAt(4, 2).Style.Bg; got != a.Theme.Panel.Bg {
		t.Fatalf("targeted redraw leaked background=%v want=%v", got, a.Theme.Panel.Bg)
	}
	book.Background = &custom
	book.SetLevels([]widget.Level{{Price: 102, Size: 11}}, []widget.Level{{Price: 110, Size: 20}})
	a.InvalidateWidgets(book)
	a.drawTo(io.Discard)
	if got := a.buf.CellAt(4, 2).Style.Bg; got != custom {
		t.Fatalf("explicit book background=%v want=%v", got, custom)
	}
}

type solidPaint struct {
	st style.Style
	ch rune
}

func (s *solidPaint) Draw(buf *buffer.Buffer, area geometry.Rect, _ *style.Theme) {
	buf.FillRect(area.X, area.Y, area.W, area.H, s.ch, s.st)
}

func (s *solidPaint) OwnsBackground() bool { return true }

func TestTargetedDamageRecomposesOverlappingWidgets(t *testing.T) {
	base := &solidPaint{st: style.Style{Bg: color.RGB(10, 20, 30)}, ch: 'B'}
	top := &solidPaint{st: style.Style{Bg: color.RGB(40, 50, 60)}, ch: 'T'}
	root := layout.NewStack(
		layout.Wrap(base),
		layout.Center(layout.Wrap(top), 0.5, 0.5),
	)
	a := &App{Root: root, Theme: style.TokyoNightTheme(), width: 40, height: 20, placements: make([]layout.Placement, 0, 4)}
	a.buf = buffer.New(40, 20)
	a.relayout()
	a.dirty.Store(true)
	a.drawTo(io.Discard)
	before := a.buf.CellAt(20, 10)
	if before.Ch != 'T' {
		t.Fatalf("top widget not painted: %+v", before)
	}
	a.InvalidateWidgets(top)
	a.drawTo(io.Discard)
	after := a.buf.CellAt(20, 10)
	if after.Ch != 'T' || after.Style.Bg != top.st.Bg {
		t.Fatalf("overlap recomposition failed: %+v", after)
	}
}
