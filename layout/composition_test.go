package layout

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

type testWidget struct{}

func (*testWidget) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func TestNestedSplitProducesMultipleResizeHandles(t *testing.T) {
	a := NewSplit(Horizontal, Wrap(&testWidget{}), Wrap(&testWidget{}), 0.5)
	b := NewSplit(Vertical, a, Wrap(&testWidget{}), 0.5)
	placements := b.Compute(geometry.Rect{W: 120, H: 40})
	count := 0
	for _, p := range placements {
		if _, ok := p.Widget.(*widget.ResizeHandle); ok {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("resize handles=%d want 2", count)
	}
}
