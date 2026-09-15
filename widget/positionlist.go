package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// PositionListItem binds a stable key to a reusable PositionPanel.
// The key is used only by callers to track identity; PositionList never
// performs trading logic or network operations.
type PositionListItem struct {
	Key   string
	Panel *PositionPanel
}

// PositionList lays out multiple PositionPanels vertically and owns their
// viewport, scrolling, focus routing, and mouse hit testing.
//
// It deliberately stores panels supplied by the application instead of
// creating panels during Draw. This keeps rendering allocation-free and lets
// the application retain per-position control state across market updates.
type PositionList struct {
	ThemeOverride *style.Theme

	Items       []PositionListItem
	PanelHeight int
	Gap         int
	Background  *color.Color
	EmptyText   string

	focusedPosition int
	focusedControl  int
	scroll          int
	focused         bool
	dirty           bool
}

func NewPositionList() *PositionList {
	return &PositionList{PanelHeight: 6, Gap: 1, EmptyText: "No open positions"}
}

func (p *PositionList) OwnsBackground() bool { return p.Background != nil }

// PreferredHeight reports the compact height required to show every position
// without making the enclosing layout stretch the list to the full terminal.
// If there are more positions than fit on screen, the normal viewport/scroll
// logic still clips them to the available area.
func (p *PositionList) PreferredHeight() int {
	if p == nil || len(p.Items) == 0 {
		return 1
	}
	h := p.PanelHeight
	if h < 1 {
		h = 6
	}
	gap := p.Gap
	if gap < 0 {
		gap = 0
	}
	count := 0
	for i := range p.Items {
		if p.Items[i].Panel != nil {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count*h + (count-1)*gap
}

func (p *PositionList) Focus(v bool) {
	if p.focused == v {
		return
	}
	p.focused = v
	p.dirty = true
	if !v {
		for i := range p.Items {
			if p.Items[i].Panel != nil {
				p.Items[i].Panel.Focus(false)
			}
		}
		return
	}
	p.normalize()
}

func (p *PositionList) IsFocused() bool { return p.focused }

// SetItems replaces the visible position set. The backing slice is copied so
// callers may reuse their own scratch slice immediately after the call.
func (p *PositionList) SetItems(items []PositionListItem) bool {
	oldLen := len(p.Items)
	same := oldLen == len(items)
	if same {
		for i := range items {
			if p.Items[i].Key != items[i].Key || p.Items[i].Panel != items[i].Panel {
				same = false
				break
			}
		}
	}
	if same {
		p.normalize()
		return false
	}
	if cap(p.Items) < len(items) {
		p.Items = make([]PositionListItem, len(items))
	} else {
		p.Items = p.Items[:len(items)]
	}
	copy(p.Items, items)
	// Drop stale panel references from the backing array when the list shrinks.
	// This keeps removed positions eligible for GC without allocating a new slice.
	if oldLen > len(items) {
		clear(p.Items[:oldLen][len(items):oldLen])
	}
	p.normalize()
	if !same {
		p.dirty = true
	}
	return true
}

func (p *PositionList) normalize() {
	if len(p.Items) == 0 {
		p.focusedPosition, p.focusedControl, p.scroll = 0, 0, 0
		return
	}
	if p.focusedPosition < 0 {
		p.focusedPosition = 0
	}
	if p.focusedPosition >= len(p.Items) {
		p.focusedPosition = len(p.Items) - 1
	}
	if p.focusedControl < 0 {
		p.focusedControl = 0
	}
	if p.focusedControl > 3 {
		p.focusedControl = 3
	}
	if p.scroll < 0 {
		p.scroll = 0
	}
	if p.scroll >= len(p.Items) {
		p.scroll = len(p.Items) - 1
	}
}

func (p *PositionList) visibleCount(area geometry.Rect) int {
	h := p.PanelHeight
	if h < 1 {
		h = 6
	}
	gap := p.Gap
	if gap < 0 {
		gap = 0
	}
	if area.H < h {
		return 0
	}
	return 1 + (area.H-h)/(h+gap)
}

func (p *PositionList) keepFocusVisible(area geometry.Rect) {
	visible := p.visibleCount(area)
	if visible <= 0 || len(p.Items) == 0 {
		return
	}
	if p.focusedPosition < p.scroll {
		p.scroll = p.focusedPosition
	}
	if p.focusedPosition >= p.scroll+visible {
		p.scroll = p.focusedPosition - visible + 1
	}
	maxScroll := len(p.Items) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scroll > maxScroll {
		p.scroll = maxScroll
	}
}

func (p *PositionList) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if p.ThemeOverride != nil {
		theme = p.ThemeOverride
	}
	if area.W < 24 || area.H < 1 {
		return
	}
	if p.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *p.Background})
	}
	if len(p.Items) == 0 {
		buf.SetString(area.X+1, area.Y, p.EmptyText, theme.TextMuted)
		return
	}

	p.normalize()
	p.keepFocusVisible(area)
	visible := p.visibleCount(area)
	if visible <= 0 {
		return
	}

	end := p.scroll + visible
	if end > len(p.Items) {
		end = len(p.Items)
	}
	panelH := p.PanelHeight
	if panelH < 1 {
		panelH = 6
	}
	gap := p.Gap
	if gap < 0 {
		gap = 0
	}
	for i := p.scroll; i < end; i++ {
		item := p.Items[i]
		if item.Panel == nil {
			continue
		}
		panelFocused := p.focused && i == p.focusedPosition
		if item.Panel.IsFocused() != panelFocused {
			item.Panel.Focus(panelFocused)
		}
		if item.Panel.Control() != p.focusedControl {
			item.Panel.SetControl(p.focusedControl)
		}
		y := area.Y + (i-p.scroll)*(panelH+gap)
		item.Panel.Draw(buf, geometry.Rect{X: area.X, Y: y, W: area.W, H: panelH}, theme)
	}
}

func (p *PositionList) HandleKey(k input.Key) bool {
	if len(p.Items) == 0 || !p.focused {
		return false
	}
	p.normalize()
	current := p.Items[p.focusedPosition].Panel
	if current == nil {
		return false
	}

	switch k.Type {
	case input.KeyUp:
		p.focusedPosition--
		if p.focusedPosition < 0 {
			p.focusedPosition = len(p.Items) - 1
		}
		return true
	case input.KeyDown:
		p.focusedPosition++
		if p.focusedPosition >= len(p.Items) {
			p.focusedPosition = 0
		}
		return true
	case input.KeyTab:
		p.focusedControl++
		if p.focusedControl > 3 {
			p.focusedControl = 0
			p.focusedPosition++
			if p.focusedPosition >= len(p.Items) {
				p.focusedPosition = 0
			}
		}
		return true
	case input.KeyShiftTab:
		p.focusedControl--
		if p.focusedControl < 0 {
			p.focusedControl = 3
			p.focusedPosition--
			if p.focusedPosition < 0 {
				p.focusedPosition = len(p.Items) - 1
			}
		}
		return true
	}

	current.Focus(true)
	current.SetControl(p.focusedControl)
	return current.HandleKey(k)
}

func (p *PositionList) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if len(p.Items) == 0 || !area.Contains(ev.X, ev.Y) {
		return false
	}
	panelH := p.PanelHeight
	if panelH < 1 {
		panelH = 6
	}
	gap := p.Gap
	if gap < 0 {
		gap = 0
	}

	switch ev.Action {
	case input.MouseWheelUp:
		if p.scroll > 0 {
			p.scroll--
		}
		if p.focusedPosition > p.scroll {
			p.focusedPosition = p.scroll
		}
		p.focused = true
		return true
	case input.MouseWheelDown:
		maxScroll := len(p.Items) - p.visibleCount(area)
		if maxScroll < 0 {
			maxScroll = 0
		}
		if p.scroll < maxScroll {
			p.scroll++
		}
		if p.focusedPosition < p.scroll {
			p.focusedPosition = p.scroll
		}
		p.focused = true
		return true
	}

	relY := ev.Y - area.Y
	step := panelH + gap
	idx := p.scroll + relY/step
	if idx < 0 || idx >= len(p.Items) || relY%step >= panelH {
		return false
	}
	item := p.Items[idx]
	if item.Panel == nil {
		return false
	}
	panelArea := geometry.Rect{X: area.X, Y: area.Y + (idx-p.scroll)*step, W: area.W, H: panelH}
	p.focused = true
	p.focusedPosition = idx
	item.Panel.Focus(true)
	if item.Panel.HandleMouse(ev, panelArea) {
		p.focusedControl = item.Panel.Control()
		return true
	}
	return false
}

func (p *PositionList) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	if area.W <= 0 || area.H <= 0 {
		return dst[:0]
	}
	if p.dirty {
		p.dirty = false
		return append(dst[:0], area)
	}
	if len(p.Items) == 0 {
		return dst[:0]
	}
	panelH := p.PanelHeight
	if panelH < 1 {
		panelH = 6
	}
	gap := p.Gap
	if gap < 0 {
		gap = 0
	}
	visible := p.visibleCount(area)
	end := p.scroll + visible
	if end > len(p.Items) {
		end = len(p.Items)
	}
	dst = dst[:0]
	for i := p.scroll; i < end; i++ {
		item := p.Items[i]
		if item.Panel == nil {
			continue
		}
		panelArea := geometry.Rect{X: area.X, Y: area.Y + (i-p.scroll)*(panelH+gap), W: area.W, H: panelH}
		before := len(dst)
		dst = item.Panel.DirtyRegions(panelArea, dst)
		if len(dst) == before {
			continue
		}
	}
	return dst
}
