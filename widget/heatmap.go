package widget

import (
	"sync"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

/*
Heatmap renders a fixed Cols x Rows grid of scalar values as solid colored
cells - a per-core load matrix, a per-service error-rate grid bucketed by
minute, or an order-book depth/correlation matrix are the typical uses.

Unlike Sparkline (a scrolling one-row strip), Heatmap is a dense 2D layout
that is updated cell-by-cell or row-by-row rather than by pushing a new
sample onto the end. Set/SetRow copy into preallocated storage under a short
RWMutex - the same tradeoff Sparkline and OrderBook make, since a grid write
isn't a single machine word - and are safe to call from a market-data or
metrics goroutine while Draw runs on the render goroutine.

Color mapping uses a fixed [min, max] domain supplied at construction rather
than auto-ranging from the currently visible data. A fixed domain keeps
repeated Set calls O(1) (no full-grid rescan for a new max) and keeps a given
value's color stable from frame to frame, which matters when only a handful
of cells change per tick. Use SetRange to change the domain deliberately
(e.g. after a scale change), which invalidates the whole grid.
*/
type Heatmap struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color // nil = inherit whatever's behind it (default)
	Low, High     color.Color  // gradient endpoints; color.Default = theme.Info.Fg / theme.Negative.Fg
	CellWidth     int          // terminal columns per grid cell; <1 defaults to 2 (roughly square on most terminals)

	mu        sync.RWMutex
	cols      int
	rows      int
	min, max  float64
	data      []float64
	scratch   []float64
	dirtyFrom int
	dirtyTo   int
	dirtyFull bool
}

// NewHeatmap creates a cols x rows grid whose values are normalized into
// [min, max] for coloring. max must be greater than min; if it isn't, max is
// pushed to min+1 so the widget still renders something sane.
func NewHeatmap(cols, rows int, min, max float64) *Heatmap {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if max <= min {
		max = min + 1
	}
	return &Heatmap{
		Low: color.Default, High: color.Default, CellWidth: 2,
		cols: cols, rows: rows, min: min, max: max,
		data:      make([]float64, cols*rows),
		scratch:   make([]float64, cols*rows),
		dirtyFrom: 0, dirtyTo: rows - 1, dirtyFull: true,
	}
}

// Cols reports the grid width fixed at construction.
func (h *Heatmap) Cols() int { return h.cols }

// Rows reports the grid height fixed at construction.
func (h *Heatmap) Rows() int { return h.rows }

// Set writes one cell and marks only that row dirty. Out-of-range col/row is
// a no-op. Safe to call from any goroutine, concurrently with Draw.
func (h *Heatmap) Set(col, row int, v float64) {
	if col < 0 || col >= h.cols || row < 0 || row >= h.rows {
		return
	}
	h.mu.Lock()
	h.data[row*h.cols+col] = v
	h.markRowDirtyLocked(row)
	h.mu.Unlock()
}

// SetRow overwrites one full row at once - e.g. the latest tick's
// per-symbol readings - marking only that row dirty. len(values) must equal
// Cols(); a mismatched length is a no-op. Safe to call from any goroutine,
// concurrently with Draw.
func (h *Heatmap) SetRow(row int, values []float64) {
	if row < 0 || row >= h.rows || len(values) != h.cols {
		return
	}
	h.mu.Lock()
	copy(h.data[row*h.cols:(row+1)*h.cols], values)
	h.markRowDirtyLocked(row)
	h.mu.Unlock()
}

// SetRange changes the normalization domain and invalidates the whole grid.
// Intended for deliberate scale changes (e.g. switching a correlation matrix
// from [-1,1] to [0,1]), not for per-tick auto-ranging.
func (h *Heatmap) SetRange(min, max float64) {
	if max <= min {
		max = min + 1
	}
	h.mu.Lock()
	h.min, h.max = min, max
	h.dirtyFull = true
	h.mu.Unlock()
}

func (h *Heatmap) markRowDirtyLocked(row int) {
	if h.dirtyFull {
		return
	}
	if h.dirtyFrom < 0 || row < h.dirtyFrom {
		h.dirtyFrom = row
	}
	if row > h.dirtyTo {
		h.dirtyTo = row
	}
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (h *Heatmap) OwnsBackground() bool { return h.Background != nil }

/*
DirtyRegions reports the screen-space row band touched by Set/SetRow calls
since the last DirtyRegions call, so a high-frequency feed only repaints the
rows that actually changed instead of the whole grid. It consumes the
pending damage the same way OrderBook.DirtyRegions does, so several bursts
of Set/SetRow calls can be published before the renderer runs without
allocating or losing a later, distinct update.
*/
func (h *Heatmap) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	h.mu.Lock()
	from, to, full := h.dirtyFrom, h.dirtyTo, h.dirtyFull
	h.dirtyFrom, h.dirtyTo, h.dirtyFull = -1, -1, false
	h.mu.Unlock()
	if area.W <= 0 || area.H <= 0 || from < 0 {
		return dst[:0]
	}
	if full {
		from, to = 0, h.rows-1
	}
	if from < 0 {
		from = 0
	}
	if to >= area.H {
		to = area.H - 1
	}
	if from > to {
		return dst[:0]
	}
	dst = dst[:0]
	dst = append(dst, geometry.Rect{X: area.X, Y: area.Y + from, W: area.W, H: to - from + 1})
	return dst
}

func (h *Heatmap) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if h.ThemeOverride != nil {
		theme = h.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	cw := h.CellWidth
	if cw < 1 {
		cw = 2
	}
	low, high := h.Low, h.High
	if low == color.Default {
		low = theme.Info.Fg
	}
	if high == color.Default {
		high = theme.Negative.Fg
	}
	bg := theme.Panel.Bg
	if h.Background != nil {
		bg = *h.Background
	}

	h.mu.RLock()
	copy(h.scratch, h.data)
	min, max := h.min, h.max
	h.mu.RUnlock()

	span := max - min
	if span == 0 {
		span = 1
	}

	cols := area.W / cw
	if cols > h.cols {
		cols = h.cols
	}
	rows := area.H
	if rows > h.rows {
		rows = h.rows
	}

	for r := 0; r < rows; r++ {
		rowOff := r * h.cols
		for c := 0; c < cols; c++ {
			v := h.scratch[rowOff+c]
			t := (v - min) / span
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			cellColor := color.Lerp(low, high, uint8(t*255))
			buf.FillRect(area.X+c*cw, area.Y+r, cw, 1, ' ', style.Style{Bg: cellColor})
		}
		if rem := area.W - cols*cw; rem > 0 {
			buf.FillRect(area.X+cols*cw, area.Y+r, rem, 1, ' ', style.Style{Bg: bg})
		}
	}
	for r := rows; r < area.H; r++ {
		buf.FillRect(area.X, area.Y+r, area.W, 1, ' ', style.Style{Bg: bg})
	}
}
