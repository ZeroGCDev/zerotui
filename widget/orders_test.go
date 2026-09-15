package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestOrdersDirtyRegionsOnlyChangedRows(t *testing.T) {
	o := NewOrders(8, 2, 2)
	rows := []Order{
		{ID: "1", Symbol: "A", Side: "BUY", Status: "OPEN", Price: 100, Qty: 1},
		{ID: "2", Symbol: "B", Side: "SELL", Status: "OPEN", Price: 101, Qty: 2},
		{ID: "3", Symbol: "C", Side: "BUY", Status: "OPEN", Price: 102, Qty: 3},
	}
	o.SetRows(rows)
	area := geometry.Rect{X: 2, Y: 3, W: 80, H: 8}
	dst := o.DirtyRegions(area, make([]geometry.Rect, 0, 8))
	if len(dst) != 4 {
		t.Fatalf("initial dirty regions=%v", dst)
	}
	if o.SetRows(rows) {
		t.Fatal("identical rows reported changed")
	}
	// Mutate only the middle row.
	rows[1].Status = "FILLED"
	if !o.SetRows(rows) {
		t.Fatal("changed row not reported")
	}
	dst = o.DirtyRegions(area, dst[:0])
	if len(dst) != 1 || dst[0].Y != area.Y+2 {
		t.Fatalf("expected only row 1 dirty, got %v", dst)
	}
}

func TestOrdersShrinkingRowsErasesStaleContent(t *testing.T) {
	o := NewOrders(4, 2, 2)
	o.SetRows([]Order{{Symbol: "OLD1", Side: "BUY"}, {Symbol: "OLD2", Side: "SELL"}})
	buf := buffer.New(40, 6)
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 4}
	o.Draw(buf, area, style.TokyoNightTheme())
	o.SetRows([]Order{{Symbol: "NEW", Side: "BUY"}})
	o.Draw(buf, area, style.TokyoNightTheme())
	for x := 0; x < area.W; x++ {
		if buf.CellAt(x, 2).Ch != ' ' {
			t.Fatalf("stale order row remained at x=%d: %q", x, buf.CellAt(x, 2).Ch)
		}
	}
}
