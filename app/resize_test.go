package app

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
)

type resizeTestWidget struct{}

func (*resizeTestWidget) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func TestHandleResizeToRebuildsGeometryAndWakesRenderer(t *testing.T) {
	root := layout.Wrap(&resizeTestWidget{})
	a := New(root, nil)
	a.width, a.height = 80, 24
	a.buf = buffer.New(80, 24)
	a.relayout()

	called := 0
	a.OnResize = func(w, h int) {
		called++
		if w != 120 || h != 40 {
			t.Fatalf("callback size=%dx%d want 120x40", w, h)
		}
	}

	a.handleResizeTo(120, 40)
	if a.width != 120 || a.height != 40 {
		t.Fatalf("app size=%dx%d want 120x40", a.width, a.height)
	}
	if a.buf.W != 120 || a.buf.H != 40 {
		t.Fatalf("buffer size=%dx%d want 120x40", a.buf.W, a.buf.H)
	}
	if len(a.placements) != 1 || a.placements[0].Area.W != 120 || a.placements[0].Area.H != 40 {
		t.Fatalf("placement=%v want full resized area", a.placements)
	}
	if !a.dirty.Load() {
		t.Fatal("resize did not mark renderer dirty")
	}
	select {
	case <-a.wake:
	default:
		t.Fatal("resize did not wake renderer")
	}
	if called != 1 {
		t.Fatalf("OnResize called %d times want 1", called)
	}
}

func TestHandleResizeToClampsInvalidTerminalSize(t *testing.T) {
	a := New(layout.Wrap(&resizeTestWidget{}), nil)
	a.handleResizeTo(0, -10)
	if a.width != 1 || a.height != 1 {
		t.Fatalf("size=%dx%d want 1x1", a.width, a.height)
	}
	if a.buf == nil || a.buf.W != 1 || a.buf.H != 1 {
		t.Fatalf("buffer not clamped to 1x1")
	}
}
