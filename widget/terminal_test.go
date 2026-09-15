package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

func TestTerminalWheelScrollClamps(t *testing.T) {
	tr := NewTerminal(".")
	for i := 0; i < 50; i++ {
		tr.Append("line\r\n")
	}
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 8}
	for i := 0; i < 100; i++ {
		tr.HandleMouse(input.MouseEvent{Action: input.MouseWheelUp}, area)
	}
	tr.mu.Lock()
	up := tr.scroll
	tr.mu.Unlock()
	if up <= 0 {
		t.Fatal("terminal did not scroll up")
	}
	for i := 0; i < 100; i++ {
		tr.HandleMouse(input.MouseEvent{Action: input.MouseWheelDown}, area)
	}
	tr.mu.Lock()
	down := tr.scroll
	tr.mu.Unlock()
	if down != 0 {
		t.Fatalf("terminal scroll did not clamp to zero: %d", down)
	}
}

func TestTerminalDrawsPromptCursor(t *testing.T) {
	tr := NewTerminal(".")
	tr.Focus(true)
	tr.input = []rune("abc")
	b := buffer.New(30, 6)
	tr.Draw(b, geometry.Rect{X: 0, Y: 0, W: 30, H: 6}, style.NordTheme())
}

func TestTerminalDrawUsesCompactCloseGlyph(t *testing.T) {
	tr := NewTerminal(".")
	b := buffer.New(30, 6)
	tr.Draw(b, geometry.Rect{X: 0, Y: 0, W: 30, H: 6}, style.NordTheme())
	if got := b.CellAt(28, 0).Ch; got != '×' {
		t.Fatalf("close glyph at right edge=%q want ×", got)
	}
	if got := b.CellAt(25, 0).Ch; got != ' ' {
		// This cell is intentionally blank; the old [x] control should
		// no longer reserve/publish a padded close control.
		t.Fatalf("unexpected wide close control at x=25: %q", got)
	}
}

func TestTerminalDirtyRegionsMapsEmulatorRows(t *testing.T) {
	tm := NewTerminal(".")
	tm.SetSize(20, 8)
	tm.Append("initial")
	var dst [8]geometry.Rect
	_ = tm.DirtyRegions(geometry.Rect{X: 2, Y: 3, W: 20, H: 9}, dst[:])
	tm.Append("hello")
	got := tm.DirtyRegions(geometry.Rect{X: 2, Y: 3, W: 20, H: 9}, dst[:])
	if len(got) != 1 {
		t.Fatalf("regions=%d, want 1", len(got))
	}
	if got[0] != (geometry.Rect{X: 2, Y: 4, W: 20, H: 1}) {
		t.Fatalf("region=%+v", got[0])
	}
	if got = tm.DirtyRegions(geometry.Rect{X: 2, Y: 3, W: 20, H: 9}, dst[:]); len(got) != 0 {
		t.Fatalf("damage was not consumed: %+v", got)
	}
}

func TestTerminalScrollbackInvalidationIsContentWide(t *testing.T) {
	tm := NewTerminal(".")
	tm.SetSize(20, 8)
	tm.Append("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\n")
	var dst [8]geometry.Rect
	_ = tm.DirtyRegions(geometry.Rect{X: 0, Y: 0, W: 20, H: 9}, dst[:])
	if !tm.HandleMouse(input.MouseEvent{Action: input.MouseWheelUp}, geometry.Rect{X: 0, Y: 0, W: 20, H: 9}) {
		t.Fatal("wheel not handled")
	}
	got := tm.DirtyRegions(geometry.Rect{X: 0, Y: 0, W: 20, H: 9}, dst[:])
	if len(got) != 1 || got[0] != (geometry.Rect{X: 0, Y: 1, W: 20, H: 8}) {
		t.Fatalf("scroll damage=%+v", got)
	}
}
