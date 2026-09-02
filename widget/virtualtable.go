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
	FocusMixin
	Columns        []Column
	Rows           int
	Cell           func(row, col int) string
	RowStyle       func(row int) *style.Style
	Selected       int
	scroll         int
	widthCache     []int
	scrollDragging bool
	Background     *color.Color
	ShowScrollBar  bool
	Zebra          bool // alternate row surface for denser modern tables
}

func NewVirtualTable(columns []Column, rows int, cell func(row, col int) string) *VirtualTable {
	return &VirtualTable{Columns: columns, Rows: rows, Cell: cell, widthCache: make([]int, len(columns))}
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (t *VirtualTable) OwnsBackground() bool { return t.Background != nil }

func (t *VirtualTable) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
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
	for row := 0; row < bodyH; row++ {
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
		if ri == t.Selected && t.focused {
			st = theme.Selected
		}
		buf.FillRect(area.X, area.Y+1+row, contentW, 1, ' ', st)
		x = area.X
		for ci := range t.Columns {
			var v string
			if t.Cell != nil {
				v = t.Cell(ri, ci)
			}
			buf.SetPaddedString(x, area.Y+1+row, v, widths[ci], t.Columns[ci].Align == AlignRight, st)
			x += widths[ci] + 1
		}
	}
	t.drawScrollBar(buf, area, bodyH, theme)
}
func (t *VirtualTable) resolveWidths(total int) []int {
	if len(t.widthCache) != len(t.Columns) {
		t.widthCache = make([]int, len(t.Columns))
	}
	used, fill := 0, -1
	for i, c := range t.Columns {
		if c.Width == 0 {
			fill = i
			t.widthCache[i] = 0
		} else {
			t.widthCache[i] = c.Width
			used += c.Width + 1
		}
	}
	if fill >= 0 {
		r := total - used
		if r < 4 {
			r = 4
		}
		t.widthCache[fill] = r
	}
	return t.widthCache
}
func (t *VirtualTable) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyUp, input.KeyRune:
		if k.Type == input.KeyRune && k.Rune != 'k' {
			return false
		}
		if t.Selected > 0 {
			t.Selected--
		}
		return true
	case input.KeyDown:
		if t.Selected+1 < t.Rows {
			t.Selected++
		}
		return true
	case input.KeyEnter:
		return true
	}
	return false
}
func (t *VirtualTable) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
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
		return true
	case input.MouseWheelDown:
		if t.Selected+1 < t.Rows {
			t.Selected++
		}
		return true
	case input.MousePress:
		if bar {
			t.scrollDragging = true
			t.setScrollFromMouse(ev.Y, area.Y+1, bodyH)
			t.Selected = t.scroll
			return true
		}
		if area.Contains(ev.X, ev.Y) {
			row := ev.Y - area.Y - 1 + t.scroll
			if row >= 0 && row < t.Rows {
				t.Selected = row
			}
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
	ScrollBar{Total: t.Rows, Offset: t.scroll, Viewport: bodyH}.Draw(buf, geometry.Rect{X: area.X + area.W - 1, Y: area.Y + 1, W: 1, H: bodyH}, theme)
}
