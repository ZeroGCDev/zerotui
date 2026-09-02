package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/geometry"
)

func TestSparklineDirtyRegionIsSingleRow(t *testing.T) {
	s := NewSparkline(16)
	dst := make([]geometry.Rect, 2)
	got := s.DirtyRegions(geometry.Rect{X: 4, Y: 7, W: 20, H: 6}, dst)
	if len(got) != 1 {
		t.Fatalf("got %d dirty regions, want 1", len(got))
	}
	want := geometry.Rect{X: 4, Y: 7, W: 20, H: 1}
	if got[0] != want {
		t.Fatalf("got %+v, want %+v", got[0], want)
	}
}
