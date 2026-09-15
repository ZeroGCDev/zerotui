package layout

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

type closableLeaf struct{}

func (*closableLeaf) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func TestClosablePanelCloseReclaimsSplitSpace(t *testing.T) {
	left := ClosableRounded("LEFT", Wrap(&closableLeaf{}), nil, nil)
	right := ClosableRounded("RIGHT", Wrap(&closableLeaf{}), nil, nil)
	s := NewSplit(Horizontal, left, right, .5)
	placements := s.Compute(geometry.Rect{W: 100, H: 20})
	if len(placements) != 7 {
		t.Fatalf("open split placements=%d want 7", len(placements))
	}
	left.Close()
	placements = s.Compute(geometry.Rect{W: 100, H: 20})
	if len(placements) != 3 {
		t.Fatalf("closed split placements=%d want 3", len(placements))
	}
	if placements[0].Area.W != 100 {
		t.Fatalf("remaining panel width=%d want 100", placements[0].Area.W)
	}
}

func TestClosablePanelCloseButtonIsPointerOnly(t *testing.T) {
	closed := false
	p := ClosableRounded("PANEL", Wrap(&closableLeaf{}), nil, func() { closed = true })
	placements := p.Compute(geometry.Rect{W: 30, H: 10})
	if len(placements) != 3 {
		t.Fatalf("placements=%d want painter+close+child", len(placements))
	}
	close := placements[1].Widget
	h, ok := close.(interface {
		HandleMouse(input.MouseEvent, geometry.Rect) bool
	})
	if !ok {
		t.Fatal("close control is not a mouse handler")
	}
	if !h.HandleMouse(input.MouseEvent{Action: input.MousePress, X: 27, Y: 0}, placements[1].Area) {
		t.Fatal("close click not consumed")
	}
	if !closed || p.Visible() {
		t.Fatalf("close click did not hide panel")
	}
}
