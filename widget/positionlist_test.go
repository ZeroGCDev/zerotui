package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

func testTheme() *style.Theme { return style.TokyoNightTheme() }

func testPanel() *PositionPanel {
	var tp, sl, tpv, slv uint32
	return NewPositionPanel(&tp, &sl, &tpv, &slv)
}

func TestPositionListNavigationAndViewport(t *testing.T) {
	items := make([]PositionListItem, 5)
	for i := range items {
		items[i] = PositionListItem{Key: string(rune('A' + i)), Panel: testPanel()}
	}
	l := NewPositionList()
	l.SetItems(items)
	l.Focus(true)

	area := geometry.Rect{X: 0, Y: 0, W: 80, H: 13} // two panels plus gap
	if !l.HandleKey(input.Key{Type: input.KeyDown}) || l.focusedPosition != 1 {
		t.Fatalf("down: pos=%d", l.focusedPosition)
	}
	l.keepFocusVisible(area)
	if l.scroll != 0 {
		t.Fatalf("unexpected scroll after second item: %d", l.scroll)
	}
	if !l.HandleKey(input.Key{Type: input.KeyDown}) || l.focusedPosition != 2 {
		t.Fatalf("second down: pos=%d", l.focusedPosition)
	}
	l.keepFocusVisible(area)
	if l.scroll != 1 {
		t.Fatalf("expected scroll=1, got %d", l.scroll)
	}

	if !l.HandleKey(input.Key{Type: input.KeyTab}) || l.focusedControl != 1 {
		t.Fatalf("tab control=%d", l.focusedControl)
	}
	if !l.HandleKey(input.Key{Type: input.KeyShiftTab}) || l.focusedControl != 0 {
		t.Fatalf("shift-tab control=%d", l.focusedControl)
	}
}

func TestPositionListMouseSelectsPanel(t *testing.T) {
	items := []PositionListItem{
		{Key: "a", Panel: testPanel()},
		{Key: "b", Panel: testPanel()},
	}
	l := NewPositionList()
	l.SetItems(items)
	area := geometry.Rect{X: 2, Y: 3, W: 80, H: 13}
	if !l.HandleMouse(input.MouseEvent{X: 4, Y: 13, Action: input.MousePress}, area) {
		t.Fatal("mouse press not consumed")
	}
	if l.focusedPosition != 1 || !l.focused {
		t.Fatalf("position=%d focused=%v", l.focusedPosition, l.focused)
	}
}

func TestPositionListDrawAllocations(t *testing.T) {
	items := make([]PositionListItem, 2)
	for i := range items {
		items[i] = PositionListItem{Key: string(rune('A' + i)), Panel: testPanel()}
		items[i].Panel.SetData(PositionPanelData{Symbol: "BTC-PERP", Side: "LONG", Quantity: "1", Entry: "100000", Mark: "100100", ROI: "+1.00%", Leverage: "10x", PnL: "+10", TPPrice: "101000", TPOrder: "PENDING"})
	}
	l := NewPositionList()
	l.SetItems(items)
	l.Focus(true)
	buf := tradingTestBuffer()
	area := geometry.Rect{X: 0, Y: 0, W: 120, H: 13}
	theme := style.TokyoNightTheme()
	l.Draw(buf, area, theme)
	allocs := testing.AllocsPerRun(100, func() { l.Draw(buf, area, theme) })
	if allocs != 0 {
		t.Fatalf("PositionList.Draw allocated %v times/run", allocs)
	}
}

func TestPositionListSetItemsReportsIdentityChange(t *testing.T) {
	p := NewPositionList()
	panel := NewPositionPanel(nil, nil, nil, nil)
	if !p.SetItems([]PositionListItem{{Key: "p1", Panel: panel}}) {
		t.Fatal("first item publication should report changed")
	}
	if p.SetItems([]PositionListItem{{Key: "p1", Panel: panel}}) {
		t.Fatal("same item identity should not report changed")
	}
}
