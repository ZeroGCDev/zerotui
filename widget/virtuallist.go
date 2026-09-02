package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// VirtualList is the allocation-free large-list counterpart to List. It asks
// the caller for only the strings that are currently visible.
type VirtualList struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color // nil = inherit the existing surface
	FocusMixin
	Count               int
	Item                func(index int) string
	Selected            int
	OnSelect            func(index int)
	ShowScrollBar       bool
	ScrollTrack         *color.Color
	ScrollThumb         *color.Color
	SelectionForeground *color.Color
	SelectionBackground *color.Color
	scroll              int
	scrollDragging      bool
	dirtySelection      bool
	dirtySelected       int
	dirtyScroll         int
}

func (l *VirtualList) OwnsBackground() bool { return l.Background != nil }

func NewVirtualList(count int, item func(index int) string) *VirtualList {
	return &VirtualList{Count: count, Item: item}
}
func (l *VirtualList) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if l.ThemeOverride != nil {
		theme = l.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	if l.Selected < l.scroll {
		l.scroll = l.Selected
	}
	if l.Selected >= l.scroll+area.H {
		l.scroll = l.Selected - area.H + 1
	}
	contentW := area.W
	if l.ShowScrollBar && l.Count > area.H {
		contentW--
	}
	if l.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *l.Background})
	}
	clip := buf.Clip()
	rowStart, rowEnd := 0, area.H
	if clip.Y > area.Y {
		rowStart = clip.Y - area.Y
	}
	if clip.Y+clip.H < area.Y+rowEnd {
		rowEnd = clip.Y + clip.H - area.Y
	}
	if rowStart < 0 {
		rowStart = 0
	}
	if rowEnd > area.H {
		rowEnd = area.H
	}
	for row := rowStart; row < rowEnd; row++ {
		i := l.scroll + row
		if i >= l.Count {
			break
		}
		st := theme.Text
		if l.Background != nil {
			st = st.WithBg(*l.Background)
		}
		prefix := "  "
		if i == l.Selected {
			prefix = "> "
			if l.focused {
				st = theme.Selected
				if l.SelectionForeground != nil {
					st = st.WithFg(*l.SelectionForeground)
				}
				if l.SelectionBackground != nil {
					st = st.WithBg(*l.SelectionBackground)
				}
			} else {
				st = theme.Info
			}
		}
		buf.FillRect(area.X, area.Y+row, contentW, 1, ' ', st)
		buf.SetString(area.X, area.Y+row, prefix, st)
		if l.Item != nil {
			buf.SetString(area.X+2, area.Y+row, l.Item(i), st)
		}
	}
	if l.ShowScrollBar && l.Count > area.H {
		ScrollBar{Total: l.Count, Offset: l.scroll, Viewport: area.H, Track: l.ScrollTrack, Thumb: l.ScrollThumb, Background: l.Background}.Draw(buf, geometry.Rect{X: area.X + area.W - 1, Y: area.Y, W: 1, H: area.H}, theme)
	}
}
func (l *VirtualList) HandleKey(k input.Key) bool {
	oldSelected, oldScroll := l.Selected, l.scroll
	consumed := false
	switch k.Type {
	case input.KeyUp:
		if l.Selected > 0 {
			l.Selected--
		}
		consumed = true
	case input.KeyDown:
		if l.Selected+1 < l.Count {
			l.Selected++
		}
		consumed = true
	case input.KeyEnter:
		if l.OnSelect != nil {
			l.OnSelect(l.Selected)
		}
		consumed = true
	case input.KeyRune:
		if k.Rune == 'j' {
			if l.Selected+1 < l.Count {
				l.Selected++
			}
			consumed = true
		}
		if k.Rune == 'k' {
			if l.Selected > 0 {
				l.Selected--
			}
			consumed = true
		}
	}
	l.noteSelectionChange(oldSelected, oldScroll)
	return consumed
}

func (l *VirtualList) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	oldSelected, oldScroll := l.Selected, l.scroll
	if !area.Contains(ev.X, ev.Y) && !l.scrollDragging {
		return false
	}
	barX := area.X + area.W - 1
	bar := l.ShowScrollBar && l.Count > area.H && area.W >= 2 && ev.X == barX
	if l.scrollDragging {
		switch ev.Action {
		case input.MouseDrag:
			l.setScrollFromMouse(ev.Y, area.Y, area.H)
			l.Selected = l.scroll
			l.noteSelectionChange(oldSelected, oldScroll)
			return true
		case input.MouseRelease:
			l.scrollDragging = false
			return true
		}
	}
	switch ev.Action {
	case input.MouseWheelUp:
		if l.Selected > 0 {
			l.Selected--
		}
		l.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.MouseWheelDown:
		if l.Selected+1 < l.Count {
			l.Selected++
		}
		l.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.MousePress:
		if bar {
			l.scrollDragging = true
			l.setScrollFromMouse(ev.Y, area.Y, area.H)
			l.Selected = l.scroll
			l.noteSelectionChange(oldSelected, oldScroll)
			return true
		}
		i := ev.Y - area.Y + l.scroll
		if i >= 0 && i < l.Count {
			l.Selected = i
			if l.OnSelect != nil {
				l.OnSelect(i)
			}
		}
		l.noteSelectionChange(oldSelected, oldScroll)
		return true
	case input.MouseRelease:
		return true
	}
	return false
}

// DirtyRegions reports only the old/new selection rows; a viewport movement
// repaints the visible body because every visible item changes.
func (l *VirtualList) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	if !l.dirtySelection || area.W <= 0 || area.H <= 0 {
		return dst[:0]
	}
	oldSelected, oldScroll := l.dirtySelected, l.dirtyScroll
	l.dirtySelection = false
	newScroll := oldScroll
	if l.Selected < newScroll {
		newScroll = l.Selected
	}
	if l.Selected >= newScroll+area.H {
		newScroll = l.Selected - area.H + 1
	}
	if newScroll < 0 {
		newScroll = 0
	}
	if newScroll != oldScroll {
		return append(dst[:0], geometry.Rect{X: area.X, Y: area.Y, W: area.W, H: area.H})
	}
	dst = dst[:0]
	if oldSelected >= newScroll && oldSelected < newScroll+area.H && oldSelected >= 0 && oldSelected < l.Count {
		dst = append(dst, geometry.Rect{X: area.X, Y: area.Y + oldSelected - newScroll, W: area.W, H: 1})
	}
	if l.Selected != oldSelected && l.Selected >= newScroll && l.Selected < newScroll+area.H && l.Selected >= 0 && l.Selected < l.Count {
		dst = append(dst, geometry.Rect{X: area.X, Y: area.Y + l.Selected - newScroll, W: area.W, H: 1})
	}
	return dst
}

func (l *VirtualList) noteSelectionChange(oldSelected, oldScroll int) {
	if oldSelected == l.Selected && oldScroll == l.scroll {
		return
	}
	if !l.dirtySelection {
		l.dirtySelected, l.dirtyScroll = oldSelected, oldScroll
	}
	l.dirtySelection = true
}

func (l *VirtualList) setScrollFromMouse(y, trackY, trackH int) {
	if l.Count <= trackH || trackH <= 0 {
		l.scroll = 0
		return
	}
	thumb := trackH * trackH / l.Count
	if thumb < 1 {
		thumb = 1
	}
	if thumb > trackH {
		thumb = trackH
	}
	maxOffset := l.Count - trackH
	maxPos := trackH - thumb
	pos := y - trackY - thumb/2
	if pos < 0 {
		pos = 0
	}
	if pos > maxPos {
		pos = maxPos
	}
	l.scroll = pos * maxOffset / maxPos
	if l.scroll > maxOffset {
		l.scroll = maxOffset
	}
}

var _ style.Style
