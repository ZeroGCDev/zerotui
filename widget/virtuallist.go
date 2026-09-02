package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// VirtualList is the allocation-free large-list counterpart to List. It asks
// the caller for only the strings that are currently visible.
type VirtualList struct {
	FocusMixin
	Count          int
	Item           func(index int) string
	Selected       int
	OnSelect       func(index int)
	ShowScrollBar  bool
	scroll         int
	scrollDragging bool
}

func NewVirtualList(count int, item func(index int) string) *VirtualList {
	return &VirtualList{Count: count, Item: item}
}
func (l *VirtualList) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
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
	for row := 0; row < area.H; row++ {
		i := l.scroll + row
		if i >= l.Count {
			break
		}
		st := theme.Text
		prefix := "  "
		if i == l.Selected {
			prefix = "> "
			if l.focused {
				st = theme.Selected
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
		ScrollBar{Total: l.Count, Offset: l.scroll, Viewport: area.H}.Draw(buf, geometry.Rect{X: area.X + area.W - 1, Y: area.Y, W: 1, H: area.H}, theme)
	}
}
func (l *VirtualList) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyUp:
		if l.Selected > 0 {
			l.Selected--
		}
		return true
	case input.KeyDown:
		if l.Selected+1 < l.Count {
			l.Selected++
		}
		return true
	case input.KeyEnter:
		if l.OnSelect != nil {
			l.OnSelect(l.Selected)
		}
		return true
	case input.KeyRune:
		if k.Rune == 'j' {
			if l.Selected+1 < l.Count {
				l.Selected++
			}
			return true
		}
		if k.Rune == 'k' {
			if l.Selected > 0 {
				l.Selected--
			}
			return true
		}
	}
	return false
}
func (l *VirtualList) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if !area.Contains(ev.X, ev.Y) && !l.scrollDragging {
		return false
	}
	barX := area.X + area.W - 1
	bar := l.ShowScrollBar && l.Count > area.H && area.W >= 2 && ev.X == barX
	if l.scrollDragging {
		switch ev.Action {
		case input.MouseDrag:
			l.setScrollFromMouse(ev.Y, area.Y, area.H)
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
		return true
	case input.MouseWheelDown:
		if l.Selected+1 < l.Count {
			l.Selected++
		}
		return true
	case input.MousePress:
		if bar {
			l.scrollDragging = true
			l.setScrollFromMouse(ev.Y, area.Y, area.H)
			l.Selected = l.scroll
			return true
		}
		if area.Contains(ev.X, ev.Y) {
			i := ev.Y - area.Y + l.scroll
			if i >= 0 && i < l.Count {
				l.Selected = i
				if l.OnSelect != nil {
					l.OnSelect(i)
				}
			}
			return true
		}
	case input.MouseRelease:
		return true
	}
	return false
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
