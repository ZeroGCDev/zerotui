package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// FastLogView is a specialized retained log/console primitive. It keeps a
// bounded ring of lines and renders only the visible viewport. Appending a
// line happens on the producer/update path; Draw never allocates.
type FastLogView struct {
	FocusMixin
	lines      []string
	head       int
	count      int
	offset     int
	FollowTail bool
	Background *color.Color // optional; retained app paths should leave this nil
}

func NewFastLogView(capacity int) *FastLogView {
	if capacity < 1 {
		capacity = 1
	}
	return &FastLogView{lines: make([]string, capacity), FollowTail: true}
}

// Append adds a line in O(1), overwriting the oldest line when the ring is full.
func (l *FastLogView) Append(line string) {
	l.lines[l.head] = line
	l.head++
	if l.head == len(l.lines) {
		l.head = 0
	}
	if l.count < len(l.lines) {
		l.count++
	}
	if l.FollowTail {
		l.offset = l.count
	}
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (l *FastLogView) OwnsBackground() bool { return l.Background != nil }

func (l *FastLogView) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W <= 0 || area.H <= 0 {
		return
	}
	st := bgOr(theme.Text, l.Background)
	if l.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', bgOr(theme.Panel, l.Background))
	}
	visible := area.H
	maxStart := l.count - visible
	if maxStart < 0 {
		maxStart = 0
	}
	start := l.offset - visible
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	for row := 0; row < visible; row++ {
		idx := start + row
		if idx >= l.count {
			break
		}
		buf.SetString(area.X, area.Y+row, l.at(idx), st)
	}
}

func (l *FastLogView) at(logical int) string {
	start := l.head - l.count
	if start < 0 {
		start += len(l.lines)
	}
	return l.lines[(start+logical)%len(l.lines)]
}

func (l *FastLogView) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyUp:
		l.offset--
		if l.offset < 0 {
			l.offset = 0
		}
		l.FollowTail = l.offset >= l.count
		return true
	case input.KeyDown:
		l.offset++
		if l.offset > l.count {
			l.offset = l.count
		}
		l.FollowTail = l.offset >= l.count
		return true
	case input.KeyRune:
		if k.Rune == 'G' {
			l.offset = l.count
			l.FollowTail = true
			return true
		}
	}
	return false
}

func (l *FastLogView) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	switch ev.Action {
	case input.MouseWheelUp:
		l.offset -= 3
		if l.offset < 0 {
			l.offset = 0
		}
		l.FollowTail = false
		return true
	case input.MouseWheelDown:
		l.offset += 3
		if l.offset > l.count {
			l.offset = l.count
		}
		l.FollowTail = l.offset >= l.count
		return true
	case input.MousePress:
		return area.Contains(ev.X, ev.Y)
	}
	return false
}

var _ Widget = (*FastLogView)(nil)
var _ Focusable = (*FastLogView)(nil)
