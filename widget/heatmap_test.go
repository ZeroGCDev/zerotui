package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestNewHeatmapClampsDimensionsAndRange(t *testing.T) {
	h := NewHeatmap(0, -3, 5, 5)
	if h.Cols() != 1 || h.Rows() != 1 {
		t.Fatalf("got cols=%d rows=%d, want 1,1", h.Cols(), h.Rows())
	}
	if h.max <= h.min {
		t.Fatalf("degenerate range not corrected: min=%v max=%v", h.min, h.max)
	}
}

func TestHeatmapSetAndSetRowBoundsChecking(t *testing.T) {
	h := NewHeatmap(3, 2, 0, 1)

	// Out-of-range Set is a no-op, not a panic.
	h.Set(-1, 0, 9)
	h.Set(0, 5, 9)
	h.Set(9, 0, 9)

	// Wrong-length SetRow is a no-op.
	h.SetRow(0, []float64{1, 2})
	h.SetRow(5, []float64{1, 2, 3})

	h.mu.RLock()
	for _, v := range h.data {
		if v != 0 {
			t.Fatalf("expected untouched grid, got %v", h.data)
		}
	}
	h.mu.RUnlock()

	h.Set(1, 1, 0.5)
	h.mu.RLock()
	got := h.data[1*3+1]
	h.mu.RUnlock()
	if got != 0.5 {
		t.Fatalf("Set(1,1,0.5) got %v", got)
	}

	h.SetRow(0, []float64{0.1, 0.2, 0.3})
	h.mu.RLock()
	row := append([]float64{}, h.data[0:3]...)
	h.mu.RUnlock()
	if row[0] != 0.1 || row[1] != 0.2 || row[2] != 0.3 {
		t.Fatalf("SetRow(0, ...) got %v", row)
	}
}

func TestHeatmapDrawColorMapsMinMaxAndMid(t *testing.T) {
	h := NewHeatmap(3, 1, 0, 10)
	h.Low = color.RGB(0, 0, 0)
	h.High = color.RGB(200, 0, 0)
	h.CellWidth = 1
	h.SetRow(0, []float64{0, 5, 10})

	buf := buffer.New(3, 1)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 3, H: 1}
	h.Draw(buf, area, theme)

	if got := buf.CellAt(0, 0).Style.Bg; got != h.Low {
		t.Fatalf("min cell bg = %v, want Low %v", got, h.Low)
	}
	if got := buf.CellAt(2, 0).Style.Bg; got != h.High {
		t.Fatalf("max cell bg = %v, want High %v", got, h.High)
	}
	mid := buf.CellAt(1, 0).Style.Bg
	if r, _, _ := mid.Components(); r == 0 || r == 200 {
		t.Fatalf("mid cell bg looks unmapped: %v", mid)
	}
}

func TestHeatmapDrawClampsOutOfRangeValues(t *testing.T) {
	h := NewHeatmap(2, 1, 0, 1)
	h.Low = color.RGB(0, 0, 0)
	h.High = color.RGB(255, 255, 255)
	h.CellWidth = 1
	h.SetRow(0, []float64{-100, 100})

	buf := buffer.New(2, 1)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 2, H: 1}
	h.Draw(buf, area, theme)

	if got := buf.CellAt(0, 0).Style.Bg; got != h.Low {
		t.Fatalf("below-range cell bg = %v, want clamped Low %v", got, h.Low)
	}
	if got := buf.CellAt(1, 0).Style.Bg; got != h.High {
		t.Fatalf("above-range cell bg = %v, want clamped High %v", got, h.High)
	}
}

func TestHeatmapDirtyRegionsTrackOnlyChangedRows(t *testing.T) {
	h := NewHeatmap(4, 5, 0, 1)
	area := geometry.Rect{X: 2, Y: 3, W: 40, H: 5}

	// Fresh heatmap starts fully dirty.
	dst := h.DirtyRegions(area, make([]geometry.Rect, 0, 4))
	if len(dst) != 1 || dst[0] != (geometry.Rect{X: 2, Y: 3, W: 40, H: 5}) {
		t.Fatalf("initial dirty regions = %v", dst)
	}

	// Consumed: nothing pending until a new write happens.
	dst = h.DirtyRegions(area, dst[:0])
	if len(dst) != 0 {
		t.Fatalf("expected no pending damage, got %v", dst)
	}

	// A single Set only dirties its own row.
	h.Set(0, 2, 0.9)
	dst = h.DirtyRegions(area, dst[:0])
	if len(dst) != 1 || dst[0] != (geometry.Rect{X: 2, Y: 5, W: 40, H: 1}) {
		t.Fatalf("expected row 2 only, got %v", dst)
	}

	// Two Set calls straddling a range coalesce into one band.
	h.Set(0, 0, 0.1)
	h.Set(0, 3, 0.4)
	dst = h.DirtyRegions(area, dst[:0])
	if len(dst) != 1 || dst[0] != (geometry.Rect{X: 2, Y: 3, W: 40, H: 4}) {
		t.Fatalf("expected rows 0-3 band, got %v", dst)
	}

	// SetRange invalidates the whole grid regardless of prior narrow damage.
	h.Set(0, 1, 0.2)
	h.SetRange(0, 5)
	dst = h.DirtyRegions(area, dst[:0])
	if len(dst) != 1 || dst[0] != (geometry.Rect{X: 2, Y: 3, W: 40, H: 5}) {
		t.Fatalf("expected full grid after SetRange, got %v", dst)
	}
}

func TestHeatmapDirtyRegionsClampToVisibleArea(t *testing.T) {
	h := NewHeatmap(2, 10, 0, 1)
	area := geometry.Rect{X: 0, Y: 0, W: 10, H: 4} // only 4 of 10 rows visible
	dst := h.DirtyRegions(area, make([]geometry.Rect, 0, 2))
	if len(dst) != 1 || dst[0].H != 4 {
		t.Fatalf("expected damage clamped to visible height 4, got %v", dst)
	}
}

func TestHeatmapDrawAllocations(t *testing.T) {
	h := NewHeatmap(20, 10, 0, 100)
	for r := 0; r < 10; r++ {
		row := make([]float64, 20)
		for c := range row {
			row[c] = float64(r*20 + c)
		}
		h.SetRow(r, row)
	}
	buf := buffer.New(80, 24)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 10}
	h.Draw(buf, area, theme)
	allocs := testing.AllocsPerRun(100, func() { h.Draw(buf, area, theme) })
	if allocs != 0 {
		t.Fatalf("Heatmap.Draw allocated %v times/run", allocs)
	}
}

func TestHeatmapDrawFillsBeyondGridWithBackdrop(t *testing.T) {
	h := NewHeatmap(2, 2, 0, 1)
	h.CellWidth = 2
	bgOverride := color.RGB(9, 9, 9)
	h.Background = &bgOverride
	h.SetRow(0, []float64{0, 1})
	h.SetRow(1, []float64{0, 1})

	buf := buffer.New(10, 5)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{X: 0, Y: 0, W: 10, H: 5} // wider and taller than the 2x2 grid (4 cols, 2 rows of cells)
	h.Draw(buf, area, theme)

	// Column past the grid's 4 occupied columns (2 cells * CellWidth 2) on an
	// occupied row should show the backdrop, not a stale/garbage cell.
	if got := buf.CellAt(9, 0).Style.Bg; got != bgOverride {
		t.Fatalf("trailing column bg = %v, want backdrop %v", got, bgOverride)
	}
	// Row past the grid's 2 occupied rows should also show the backdrop.
	if got := buf.CellAt(0, 4).Style.Bg; got != bgOverride {
		t.Fatalf("trailing row bg = %v, want backdrop %v", got, bgOverride)
	}
}
