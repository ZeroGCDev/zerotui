package layout

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/widget"
)

func TestSplitResizeHandleDragChangesRatio(t *testing.T) {
	s := NewSplit(Horizontal, Wrap(&testWidget{}), Wrap(&testWidget{}), 0.5)
	area := geometry.Rect{X: 0, Y: 0, W: 100, H: 20}
	placements := s.Compute(area)
	var handleArea geometry.Rect
	for _, p := range placements {
		if _, ok := p.Widget.(*widget.ResizeHandle); ok {
			handleArea = p.Area
			break
		}
	}
	if handleArea.W == 0 {
		t.Fatal("resize handle was not placed")
	}
	before := s.Ratio
	if !s.Handle.HandleMouse(input.MouseEvent{X: handleArea.X, Y: 5, Action: input.MousePress}, handleArea) {
		t.Fatal("resize handle press was not handled")
	}
	s.Handle.OnDrag(input.MouseEvent{X: 70, Y: 5, Action: input.MouseDrag}, handleArea)
	if s.Ratio <= before {
		t.Fatalf("ratio=%v did not increase from %v", s.Ratio, before)
	}
	if s.Ratio < s.Handle.MinRatio || s.Ratio > s.Handle.MaxRatio {
		t.Fatalf("ratio=%v outside clamp", s.Ratio)
	}
}
