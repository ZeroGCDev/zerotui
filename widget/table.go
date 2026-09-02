package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// Column describes one table column. Width is in cells; a Width of 0 means "fill remaining space" (only the last such column is honored).
type Column struct {
	Title string
	Width int
	Align Align
}

type Align uint8

const (
	AlignLeft Align = iota
	AlignRight
)

// Table renders a header row plus a scrollable, keyboard/mouse-navigable body - open positions, the order book ladder, a trade blotter, etc. Rows are supplied by the caller each frame (or cached and only replaced on change); Table itself does not allocate during Draw.
type Table struct {
	FocusMixin
	Columns    []Column
	Rows       [][]string                 // Rows[i][j] must line up with Columns
	RowStyle   func(row int) *style.Style // optional per-row override (e.g. green/red PNL)
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
		if ri == t.Selected && t.focused {
			rowSt = theme.Selected // intentionally ignores Background: selection must stay legible
		}
		buf.FillRect(area.X, bodyY+row, area.W, 1, ' ', rowSt)
		x := area.X
		for ci, cell := range t.Rows[ri] {
			if ci >= len(widths) {
				break
			}
			buf.SetPaddedString(x, bodyY+row, cell, widths[ci], t.Columns[ci].Align == AlignRight, rowSt)
			x += widths[ci] + 1
		}
	}
}

func (t *Table) resolveWidths(totalW int) []int {
	n := len(t.Columns)
	if n != len(t.widthCache) {
		t.widthCache = make([]int, n)
	}
	used, fillIdx := 0, -1
	for i := 0; i < n; i++ {
		c := t.Columns[i]
		if c.Width == 0 {
			fillIdx = i
			t.widthCache[i] = 0
			continue
		}
		t.widthCache[i] = c.Width
		used += c.Width + 1
	}
	if fillIdx >= 0 {
		remain := totalW - used
		if remain < 4 {
			remain = 4
		}
		t.widthCache[fillIdx] = remain
	}
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
