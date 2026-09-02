package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// Column describes one table column. Width is fixed cells; Width 0 makes the column flexible. Weight controls the share of remaining space.
type Column struct {
	Title  string
	Width  int // fixed width in cells; 0 = flexible
	Weight int // flexible share; 0 means weight 1
	Align  Align
}

type Align uint8

const (
	AlignLeft Align = iota
	AlignRight
)

// Table renders a header row plus a scrollable, keyboard/mouse-navigable body - open positions, the order book ladder, a trade blotter, etc. Rows are supplied by the caller each frame (or cached and only replaced on change); Table itself does not allocate during Draw.
type Table struct {
	// SelectionForeground/SelectionBackground control the complete selected row.
	SelectionForeground *color.Color
	SelectionBackground *color.Color
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	FocusMixin
	Columns    []Column
	Rows       [][]string                      // Rows[i][j] must line up with Columns
	RowStyle   func(row int) *style.Style      // optional per-row override (e.g. green/red PNL)
	CellStyle  func(row, col int) *style.Style // optional per-cell style
	Selected   int
	Background *color.Color // nil = inherit whatever's behind it (default); fills header + unselected rows
	Zebra      bool         // alternate row surface for dense modern tables
	scroll     int
	widthCache []int
}

func NewTable(columns []Column) *Table {
	return &Table{Columns: columns, widthCache: make([]int, len(columns))}
}

func (t *Table) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if t.ThemeOverride != nil {
		theme = t.ThemeOverride
	}
	if area.H < 1 {
		return
	}
	if t.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *t.Background})
	}
	widths := t.resolveWidths(area.W)

	x := area.X
	for i, c := range t.Columns {
		buf.SetPaddedString(x, area.Y, c.Title, widths[i], c.Align == AlignRight, bgOr(theme.Title, t.Background))
		x += widths[i] + 1
	}
	bodyY := area.Y + 1
	bodyH := area.H - 1
	if bodyH < 1 {
		return
	}

	if t.Selected < t.scroll {
		t.scroll = t.Selected
	}
	if t.Selected >= t.scroll+bodyH {
		t.scroll = t.Selected - bodyH + 1
	}

	for row := 0; row < bodyH; row++ {
		ri := t.scroll + row
		if ri >= len(t.Rows) {
			break
		}
		rowSt := bgOr(theme.Text, t.Background)
		if t.Zebra && ri&1 == 1 {
			rowSt = bgOr(theme.TextMuted, t.Background)
		}
		if t.RowStyle != nil {
			if custom := t.RowStyle(ri); custom != nil {
				rowSt = bgOr(*custom, t.Background)
			}
		}
		selected := ri == t.Selected && t.focused
		if selected {
			rowSt = theme.Selected
			if t.SelectionForeground != nil {
				rowSt = rowSt.WithFg(*t.SelectionForeground)
			}
			if t.SelectionBackground != nil {
				rowSt = rowSt.WithBg(*t.SelectionBackground)
			}
		}
		buf.FillRect(area.X, bodyY+row, area.W, 1, ' ', rowSt)
		x := area.X
		for ci, cell := range t.Rows[ri] {
			if ci >= len(widths) {
				break
			}
			cellSt := rowSt
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
			buf.SetPaddedString(x, bodyY+row, cell, widths[ci], t.Columns[ci].Align == AlignRight, cellSt)
			x += widths[ci] + 1
		}
	}
}

func resolveColumnWidths(columns []Column, total int, cache []int) []int {
	if len(cache) != len(columns) {
		cache = make([]int, len(columns))
	}
	used, flex, weightTotal, lastFlex := 0, 0, 0, -1
	for i, c := range columns {
		if c.Width > 0 {
			cache[i] = c.Width
			used += c.Width + 1
			continue
		}
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		cache[i] = 0
		flex++
		weightTotal += w
		lastFlex = i
	}
	if flex == 0 {
		return cache
	}
	remaining := total - used - (flex - 1)
	if remaining < flex*4 {
		remaining = flex * 4
	}
	allocated := 0
	for i, c := range columns {
		if c.Width > 0 {
			continue
		}
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		width := remaining * w / weightTotal
		if width < 4 {
			width = 4
		}
		if i == lastFlex {
			width = remaining - allocated
		}
		if width < 1 {
			width = 1
		}
		cache[i] = width
		allocated += width
	}
	return cache
}

func (t *Table) resolveWidths(totalW int) []int {
	t.widthCache = resolveColumnWidths(t.Columns, totalW, t.widthCache)
	return t.widthCache
}

func (t *Table) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyUp:
		if t.Selected > 0 {
			t.Selected--
		}
		return true
	case input.KeyDown:
		if t.Selected < len(t.Rows)-1 {
			t.Selected++
		}
		return true
	case input.KeyRune:
		switch k.Rune {
		case 'k':
			if t.Selected > 0 {
				t.Selected--
			}
			return true
		case 'j':
			if t.Selected < len(t.Rows)-1 {
				t.Selected++
			}
			return true
		}
	}
	return false
}

func (t *Table) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MouseWheelUp {
		if t.Selected > 0 {
			t.Selected--
		}
		return true
	}
	if ev.Action == input.MouseWheelDown {
		if t.Selected < len(t.Rows)-1 {
			t.Selected++
		}
		return true
	}
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		row := ev.Y - area.Y - 1 + t.scroll
		if row >= 0 && row < len(t.Rows) {
			t.Selected = row
		}
		return true
	}
	return false
}
