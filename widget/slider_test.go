package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestSliderMouseUsesRenderedTrackWidth(t *testing.T) {
	var v uint32 = 50
	s := NewSlider("Leverage", &v, 0, 100, 1, FormatInt("x"))
	area := geometry.Rect{X: 10, Y: 2, W: 24, H: 1}
	b := buffer.New(80, 10)
	s.Draw(b, area, style.NordTheme())
	valueBuf := s.formatValue(s.get())
	trackX, trackW, _ := s.trackGeometry(area, byteCellWidth(valueBuf))
	if trackW <= 0 {
		t.Fatal("expected a visible track")
	}
	if !s.HandleMouse(input.MouseEvent{Action: input.MousePress, X: trackX + trackW - 1, Y: area.Y}, area) {
		t.Fatal("track click was not handled")
	}
	if v != 100 {
		t.Fatalf("value=%d, want 100", v)
	}
}

func TestSliderNarrowAreaDoesNotDrawOutsidePlacement(t *testing.T) {
	var v uint32 = 50
	s := NewSlider("Long", &v, 0, 100, 1, FormatInt("%"))
	area := geometry.Rect{X: 20, Y: 3, W: 6, H: 1}
	b := buffer.New(40, 8)
	b.SetClip(buffer.Rect{X: area.X, Y: area.Y, W: area.W, H: area.H})
	s.Draw(b, area, style.NordTheme())
	// The assertion is structural: every cell changed by the slider must be
	// inside the placement clip. The buffer's clipping contract guarantees no
	// writes outside it; this also guards against future direct writes.
	for x := 0; x < b.W; x++ {
		for y := 0; y < b.H; y++ {
			if x < area.X || x >= area.X+area.W || y != area.Y {
				if b.CellAt(x, y).Ch != 0 {
					t.Fatalf("narrow slider wrote outside area at (%d,%d)", x, y)
				}
			}
		}
	}
}

func TestSliderThumbRemainsVisibleAtEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name        string
		value       uint32
		wantXOffset int
	}{
		{name: "min", value: 0, wantXOffset: 0},
		{name: "max", value: 100, wantXOffset: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.value
			s := NewSlider("Slider", &v, 0, 100, 1, FormatInt("%"))
			area := geometry.Rect{X: 10, Y: 1, W: 30, H: 1}
			b := buffer.New(60, 4)
			s.Draw(b, area, style.NordTheme())
			valueBuf := s.formatValue(v)
			trackX, trackW, _ := s.trackGeometry(area, byteCellWidth(valueBuf))
			wantOffset := tc.wantXOffset
			if wantOffset < 0 {
				wantOffset = trackW - 1
			}
			cell := b.CellAt(trackX+wantOffset, area.Y)
			if cell.Ch != '█' {
				t.Fatalf("thumb cell=%q, want █", cell.Ch)
			}
			if trackW <= 0 {
				t.Fatalf("trackW=%d, want thumb inside track", trackW)
			}
		})
	}
}
