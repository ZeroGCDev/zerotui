package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// VirtualTable renders only rows that intersect the viewport. Cell is called
// for visible cells only, which makes million-row/order-flow views practical.
// The callback should return an existing string or format into caller-owned
// storage; VirtualTable itself never materializes the full dataset.
type VirtualTable struct {
	// SelectionForeground/SelectionBackground control the complete selected row.
	SelectionForeground *color.Color
	SelectionBackground *color.Color
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	FocusMixin
	Columns        []Column
	Rows           int
	Cell           func(row, col int) string
	RowStyle       func(row int) *style.Style
	CellStyle      func(row, col int) *style.Style // optional per-cell style; called only for painted cells
	Selected       int
	scroll         int
	widthCache     []int
	scrollDragging bool
	Background     *color.Color
	ScrollTrack    *color.Color
	ScrollThumb    *color.Color
	ShowScrollBar  bool
	Zebra          bool // alternate row surface for denser modern tables
	dirtySelection bool
	dirtySelected  int
	dirtyScroll    int
}

func NewVirtualTable(columns []Column, rows int, cell func(row, col int) string) *VirtualTable {
	return &VirtualTable{Columns: columns, Rows: rows, Cell: cell, widthCache: make([]int, len(columns))}
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (t *VirtualTable) OwnsBackground() bool { return t.Background != nil }

func (t *VirtualTable) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if t.ThemeOverride != nil {
		theme = t.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	contentW := area.W
	if t.ShowScrollBar && t.Rows > area.H-1 {
		contentW--
	}
	widths := t.resolveWidths(contentW)
	x := area.X
	for i, c := range t.Columns {
		buf.SetPaddedString(x, area.Y, c.Title, widths[i], c.Align == AlignRight, bgOr(theme.Title, t.Background))
		x += widths[i] + 1
	}
	bodyH := area.H - 1
	if bodyH <= 0 {
		return
	}
	if t.Selected < t.scroll {
		t.scroll = t.Selected
	}
	if t.Selected >= t.scroll+bodyH {
		t.scroll = t.Selected - bodyH + 1
	}
	if t.scroll < 0 {
		t.scroll = 0
	}
	// Respect the buffer clip before asking the data callbacks for rows. A
	// one-row selection repaint should not execute RowStyle/Cell for the entire
	// viewport. This is a meaningful CPU saving for expensive data adapters.
	clip := buf.Clip()
	rowStart, rowEnd := 0, bodyH
	if clip.Y > area.Y+1 {
		rowStart = clip.Y - (area.Y + 1)
	}
	if clip.Y+clip.H < area.Y+1+rowEnd {
		rowEnd = clip.Y + clip.H - (area.Y + 1)
	}
	if rowStart < 0 {
		rowStart = 0
	}
	if rowEnd > bodyH {
		rowEnd = bodyH
	}
	for row := rowStart; row < rowEnd; row++ {
		ri := t.scroll + row
		if ri >= t.Rows {
			break
		}
		st := bgOr(theme.Text, t.Background)
		if t.Zebra && ri&1 == 1 {
			st = bgOr(theme.TextMuted, t.Background)
		}
		if t.RowStyle != nil {
			if rs := t.RowStyle(ri); rs != nil {
				st = bgOr(*rs, t.Background)
			}
		}
		selected := ri == t.Selected && t.focused
		if selected {
			st = theme.Selected
			if t.SelectionForeground != nil {
				st = st.WithFg(*t.SelectionForeground)
			}
			if t.SelectionBackground != nil {
				st = st.WithBg(*t.SelectionBackground)
			}
		}
		buf.FillRect(area.X, area.Y+1+row, area.W, 1, ' ', st)
		x = area.X
		for ci := range t.Columns {
			cellSt := st
			if t.CellStyle != nil {
				if custom := t.CellStyle(ri, ci); custom != nil {
					cellSt = *custom
				}
			}
			if selected {
				if t.SelectionForeground != nil {
					cellSt = cellSt.WithFg(*t.SelectionForeground)
				} else {
					cellSt = cellSt.WithFg(theme.Selected.Fg)
				}
				if t.SelectionBackground != nil {
					cellSt = cellSt.WithBg(*t.SelectionBackground)
				} else {
					cellSt = cellSt.WithBg(theme.Selected.Bg)
				}
			}
			var v string
			if t.Cell != nil {
				v = t.Cell(ri, ci)
			}
			buf.SetPaddedString(x, area.Y+1+row, v, widths[ci], t.Columns[ci].Align == AlignRight, cellSt)
			x += widths[ci] + 1
		}
	}
	// Draw the scrollbar after rows; then repaint the selected scrollbar cell so
	// the selected row remains continuous across the full table width.
	if t.ShowScrollBar && t.Rows > bodyH && area.W >= 2 {
		ScrollBar{Total: t.Rows, Offset: t.scroll, Viewport: bodyH, Track: t.ScrollTrack, Thumb: t.ScrollThumb}.Draw(buf, geometry.Rect{X: area.X + area.W - 1, Y: area.Y + 1, W: 1, H: bodyH}, theme)
	}
	// Repaint the selected scrollbar cell with the selection style so the
	// highlight remains visually continuous across the full row.
	if t.focused && t.Selected >= t.scroll && t.Selected < t.scroll+bodyH && t.ShowScrollBar && t.Rows > bodyH && area.W >= 2 {
		st := theme.Selected
		if t.SelectionForeground != nil {
			st = st.WithFg(*t.SelectionForeground)
		}
		if t.SelectionBackground != nil {
			st = st.WithBg(*t.SelectionBackground)
		}
		// Keep the scrollbar thumb glyph while applying the row selection style.
		// Using a space here made the selected cell visually cut the thumb into
		// two pieces during fast scrolling. The row still owns the cell's full
		// background, while the thumb remains visually continuous.
		buf.FillRect(area.X+area.W-1, area.Y+1+(t.Selected-t.scroll), 1, 1, '█', st)
	}
}

// DirtyRegions returns the visible table body for selection/scroll changes.
// Keeping it as one contiguous region guarantees that an old selection or
// scrollbar paint can never leave a stale strip behind. It is still localized
// to the table (typically a few dozen rows) and uses no heap allocation.
func (t *VirtualTable) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	if !t.dirtySelection || area.W <= 0 || area.H <= 1 {
		return dst[:0]
	}
	t.dirtySelection = false
	return append(dst[:0], geometry.Rect{X: area.X, Y: area.Y + 1, W: area.W, H: area.H - 1})
}

func (t *VirtualTable) noteSelectionChange(oldSelected, oldScroll int) {
	if oldSelected == t.Selected && oldScroll == t.scroll {
		return
	}
	if !t.dirtySelection {
		t.dirtySelected = oldSelected
		t.dirtyScroll = oldScroll
	}
	t.dirtySelection = true
}

func (t *VirtualTable) resolveWidths(total int) []int {
	t.widthCache = resolveColumnWidths(t.Columns, total, t.widthCache)
	return t.widthCache
}

func (t *VirtualTable) HandleKey(k input.Key) bool {
	oldSelected, oldScroll := t.Selected, t.scroll
	switch k.Type {
	case input.KeyUp, input.KeyRune:
		if k.Type == input.KeyRune && k.Rune != 'k' {
			return false
		}
		if t.Selected > 0 {
			t.Selected--
		}
		t.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.KeyDown:
		if t.Selected+1 < t.Rows {
			t.Selected++
		}
		t.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.KeyEnter:
		return true
	}
	return false
}
func (t *VirtualTable) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	oldSelected, oldScroll := t.Selected, t.scroll
	if !area.Contains(ev.X, ev.Y) && !t.scrollDragging {
		return false
	}
	bodyH := area.H - 1
	bar := t.ShowScrollBar && t.Rows > bodyH && bodyH > 0 && area.W >= 2 && ev.X == area.X+area.W-1 && ev.Y >= area.Y+1
	if t.scrollDragging {
		switch ev.Action {
		case input.MouseDrag:
			t.setScrollFromMouse(ev.Y, area.Y+1, bodyH)
			t.Selected = t.scroll
			t.noteSelectionChange(oldSelected, oldScroll)
			return true
		case input.MouseRelease:
			t.scrollDragging = false
			return true
		}
	}
	switch ev.Action {
	case input.MouseWheelUp:
		if t.Selected > 0 {
			t.Selected--
		}
		t.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.MouseWheelDown:
		if t.Selected+1 < t.Rows {
			t.Selected++
		}
		t.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.MousePress:
		if bar {
			t.scrollDragging = true
			t.setScrollFromMouse(ev.Y, area.Y+1, bodyH)
			t.Selected = t.scroll
			t.noteSelectionChange(oldSelected, oldScroll)
			return true
		}
		if area.Contains(ev.X, ev.Y) {
			row := ev.Y - area.Y - 1 + t.scroll
			if row >= 0 && row < t.Rows {
				t.Selected = row
			}
			t.noteSelectionChange(oldSelected, oldScroll)
			return true
		}
	case input.MouseRelease:
		return true
	}
	return false
}

func (t *VirtualTable) setScrollFromMouse(y, trackY, trackH int) {
	if t.Rows <= trackH || trackH <= 0 {
		t.scroll = 0
		return
	}
	thumb := trackH * trackH / t.Rows
	if thumb < 1 {
		thumb = 1
	}
	if thumb > trackH {
		thumb = trackH
	}
	maxOffset := t.Rows - trackH
	maxPos := trackH - thumb
	pos := y - trackY - thumb/2
	if pos < 0 {
		pos = 0
	}
	if pos > maxPos {
		pos = maxPos
	}
	t.scroll = pos * maxOffset / maxPos
	if t.scroll > maxOffset {
		t.scroll = maxOffset
	}
}

func (t *VirtualTable) drawScrollBar(buf *buffer.Buffer, area geometry.Rect, bodyH int, theme *style.Theme) {
	if !t.ShowScrollBar || t.Rows <= bodyH || area.W < 2 {
		return
	}
	ScrollBar{Total: t.Rows, Offset: t.scroll, Viewport: bodyH, Track: t.ScrollTrack, Thumb: t.ScrollThumb}.Draw(buf, geometry.Rect{X: area.X + area.W - 1, Y: area.Y + 1, W: 1, H: bodyH}, theme)
}
