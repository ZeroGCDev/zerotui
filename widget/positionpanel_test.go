package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestPositionPanelControlsAreIndependent(t *testing.T) {
	var tpOn, slOn uint32 = 1, 1
	var tp, sl uint32 = 100, 200
	p := NewPositionPanel(&tpOn, &slOn, &tp, &sl)
	p.SetControl(1)
	p.Focus(true)
	if !p.HandleKey(input.Key{Type: input.KeyRight}) {
		t.Fatal("expected TP slider to consume right key")
	}
	if tp != 105 || sl != 200 {
		t.Fatalf("controls crossed: tp=%d sl=%d", tp, sl)
	}
	p.SetControl(3)
	if !p.HandleKey(input.Key{Type: input.KeyRight}) {
		t.Fatal("expected SL slider to consume right key")
	}
	if tp != 105 || sl != 205 {
		t.Fatalf("controls crossed: tp=%d sl=%d", tp, sl)
	}
}

func TestPositionPanelDraw(t *testing.T) {
	var tpOn, slOn uint32 = 1, 1
	var tp, sl uint32 = 100, 200
	p := NewPositionPanel(&tpOn, &slOn, &tp, &sl)
	p.Title = "POSITION"
	p.SetData(PositionPanelData{
		Symbol: "B-BTC_USDT", Side: "LONG", Quantity: "0.01",
		Entry: "78000.00", Mark: "78100.00", ROI: "+1.28%",
		Leverage: "10x", PnL: "+1.00", TPPrice: "85800.00", TPOrder: "PENDING",
	})
	b := buffer.New(100, 6)
	p.Draw(b, geometry.Rect{X: 0, Y: 0, W: 100, H: 6}, style.TokyoNightTheme())
}

func TestPositionPanelDirtyRegionsAreRowLocal(t *testing.T) {
	var tpOn, slOn, tpValue, slValue uint32
	p := NewPositionPanel(&tpOn, &slOn, &tpValue, &slValue)
	area := geometry.Rect{X: 2, Y: 4, W: 80, H: 6}
	regions := p.DirtyRegions(area, make([]geometry.Rect, 0, 8))
	if len(regions) == 0 {
		t.Fatal("new panel should be dirty")
	}
	if got := p.DirtyRegions(area, regions[:0]); len(got) != 0 {
		t.Fatalf("dirty regions were not consumed: %v", got)
	}
	p.SetData(PositionPanelData{Symbol: "BTC"})
	regions = p.DirtyRegions(area, regions[:0])
	if len(regions) != 3 {
		t.Fatalf("expected three data rows, got %d: %v", len(regions), regions)
	}
	if regions[0].Y != area.Y+1 || regions[1].Y != area.Y+2 || regions[2].Y != area.Y+5 {
		t.Fatalf("unexpected data dirty rows: %v", regions)
	}
}

func TestPositionPanelNarrowLayoutStaysInsideArea(t *testing.T) {
	var tpOn, slOn, tpValue, slValue uint32
	p := NewPositionPanel(&tpOn, &slOn, &tpValue, &slValue)
	p.SetData(PositionPanelData{Symbol: "BTC-PERP-VERY-LONG", Side: "LONG", Quantity: "999999"})
	th := style.TokyoNightTheme()
	for _, w := range []int{24, 28, 32, 40} {
		area := geometry.Rect{X: 5, Y: 7, W: w, H: 6}
		t1, s1, t2, s2 := p.controlAreas(area)
		for _, r := range []geometry.Rect{t1, s1, t2, s2} {
			if r.W < 1 || r.X < area.X || r.X+r.W > area.X+area.W-1 || r.Y < area.Y || r.Y+r.H > area.Y+area.H-1 {
				t.Fatalf("width %d produced out-of-bounds control %v in %v", w, r, area)
			}
		}
		b := buffer.New(80, 20)
		p.Draw(b, area, th)
	}
}

func TestPositionPanelCloseHitAreaMatchesDrawnButton(t *testing.T) {
	closed := false
	p := NewPositionPanel(nil, nil, nil, nil)
	p.OnClose = func() { closed = true }
	p.SetData(PositionPanelData{Symbol: "BTC-PERP-VERY-LONG", Quantity: "999999999999999999"})
	area := geometry.Rect{X: 0, Y: 0, W: 24, H: 6}
	// With a crowded summary row the close button is not drawn, so clicking its
	// former location must not trigger a phantom close action.
	if p.HandleMouse(input.MouseEvent{Action: input.MousePress, X: 17, Y: 1}, area) {
		if closed {
			t.Fatal("phantom close button hit")
		}
	}
}
