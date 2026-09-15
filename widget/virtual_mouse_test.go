package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
)

func TestVirtualListScrollbarMouseDrag(t *testing.T) {
	l := NewVirtualList(1000, func(i int) string { return "item" })
	l.ShowScrollBar = true
	area := geometry.Rect{X: 2, Y: 3, W: 20, H: 20}
	barX := area.X + area.W - 1
	if !l.HandleMouse(input.MouseEvent{X: barX, Y: area.Y + area.H - 1, Action: input.MousePress}, area) {
		t.Fatal("scrollbar press was not handled")
	}
	if l.scroll <= 0 {
		t.Fatal("scrollbar press did not move scroll offset")
	}
	if !l.HandleMouse(input.MouseEvent{X: barX, Y: area.Y, Action: input.MouseDrag}, area) {
		t.Fatal("scrollbar drag was not handled")
	}
	if l.scroll != 0 {
		t.Fatalf("top scrollbar drag scroll=%d want 0", l.scroll)
	}
	if !l.HandleMouse(input.MouseEvent{X: barX, Y: area.Y, Action: input.MouseRelease}, area) {
		t.Fatal("scrollbar release was not handled")
	}
}

func TestVirtualTableScrollbarMouseDrag(t *testing.T) {
	table := NewVirtualTable([]Column{{Title: "A", Width: 4}}, 1000, func(row, col int) string { return "x" })
	table.ShowScrollBar = true
	area := geometry.Rect{X: 2, Y: 3, W: 20, H: 20}
	barX := area.X + area.W - 1
	bottom := area.Y + area.H - 1
	if !table.HandleMouse(input.MouseEvent{X: barX, Y: bottom, Action: input.MousePress}, area) {
		t.Fatal("scrollbar press was not handled")
	}
	if table.scroll <= 0 {
		t.Fatal("scrollbar press did not move scroll offset")
	}
	if !table.HandleMouse(input.MouseEvent{X: barX, Y: area.Y + 1, Action: input.MouseDrag}, area) {
		t.Fatal("scrollbar drag was not handled")
	}
	if table.scroll != 0 {
		t.Fatalf("top scrollbar drag scroll=%d want 0", table.scroll)
	}
}
