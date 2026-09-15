package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

func TestTabsClosableAndModified(t *testing.T) {
	tabs := NewTabs([]string{"one.go", "two.go"})
	tabs.ShowClose = true
	tabs.Modified = []bool{false, true}
	closed := -1
	tabs.OnClose = func(i int) bool { closed = i; return true }
	b := buffer.New(30, 2)
	tabs.Draw(b, geometry.Rect{W: 30, H: 1}, style.NordTheme())
	if tabs.TabWidth(1) != 11 {
		t.Fatalf("second tab width = %d, want 11", tabs.TabWidth(1))
	}
	// The second tab is visible; its close affordance is the final cell.
	x := tabs.TabWidth(0) + tabs.TabWidth(1) - 2
	if !tabs.HandleMouse(input.MouseEvent{Action: input.MousePress, Button: input.MouseLeft, X: x, Y: 0}, geometry.Rect{W: 30, H: 1}) {
		t.Fatal("close click not handled")
	}
	if closed != 1 {
		t.Fatalf("closed tab = %d, want 1", closed)
	}
}
