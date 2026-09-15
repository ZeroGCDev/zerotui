package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestToggleUsesCellWidthForBoxAndUnicodeLabel(t *testing.T) {
	var value uint32 = 1
	toggle := NewToggle("界😀", &value)
	buf := buffer.New(32, 1)
	toggle.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 32, H: 1}, style.TokyoNightTheme())
	flagStart := CellWidth("[X] ") + CellWidth("界😀") + 1
	if got := string(buf.CellAt(flagStart, 0).Ch); got != "A" {
		t.Fatalf("flag starts at wrong terminal cell %d: got %q", flagStart, got)
	}
}
