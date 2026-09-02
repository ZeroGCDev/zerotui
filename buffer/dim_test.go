package buffer

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestDimRectPreservesGlyphAndMatchesBlend(t *testing.T) {
	buf := New(2, 1)
	st := style.Style{Fg: color.RGB(200, 100, 50), Bg: color.RGB(20, 30, 40), Attr: style.Bold}
	buf.Set(0, 0, 'X', st)
	buf.DimRect(0, 0, 1, 1, 100)
	got := buf.CellAt(0, 0)
	want := st
	want.Fg = color.Lerp(st.Fg, st.Bg, 100)
	want.Attr |= style.Dim
	if got.Ch != 'X' || got.Style != want {
		t.Fatalf("DimRect mismatch: got=%+v want=%+v", got, Cell{Ch: 'X', Style: want})
	}
}

func TestDimRectAttrOnlyAddsDim(t *testing.T) {
	buf := New(1, 1)
	st := style.Style{Fg: color.RGB(200, 100, 50), Bg: color.RGB(20, 30, 40), Attr: style.Bold}
	buf.Set(0, 0, 'X', st)
	buf.DimRectAttr(0, 0, 1, 1)
	got := buf.CellAt(0, 0)
	want := st
	want.Attr |= style.Dim
	if got != (Cell{Ch: 'X', Style: want}) {
		t.Fatalf("DimRectAttr mismatch: got=%+v want=%+v", got, Cell{Ch: 'X', Style: want})
	}
}
