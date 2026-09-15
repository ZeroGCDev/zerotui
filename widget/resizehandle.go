package widget

import (
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// ResizeDirection identifies the orientation of a draggable layout divider.
type ResizeDirection uint8

const (
	ResizeVertical   ResizeDirection = iota // vertical divider; drag X
	ResizeHorizontal                        // horizontal divider; drag Y
)

// ResizeHandle is a pointer-only layout divider. It deliberately does not join
// the keyboard focus ring: a divider is a geometry control, not an input field.
type ResizeHandle struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color
	Direction     ResizeDirection
	Ratio         *float64
	MinRatio      float64
	MaxRatio      float64
	Dragging      atomic.Bool
	OnResize      func(float64)
	OnDrag        func(ev input.MouseEvent, area geometry.Rect)
}

func NewResizeHandle(dir ResizeDirection, ratio *float64) *ResizeHandle {
	return &ResizeHandle{Direction: dir, Ratio: ratio, MinRatio: 0.15, MaxRatio: 0.85}
}

func (h *ResizeHandle) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if h.ThemeOverride != nil {
		theme = h.ThemeOverride
	}
	if area.W <= 0 || area.H <= 0 {
		return
	}
	st := bgOr(theme.Border, h.Background)
	if h.Dragging.Load() {
		st = theme.Info
	}
	if h.Direction == ResizeVertical {
		x := area.X + area.W/2
		for y := area.Y; y < area.Y+area.H; y++ {
			buf.Set(x, y, '│', st)
		}
	} else {
		y := area.Y + area.H/2
		for x := area.X; x < area.X+area.W; x++ {
			buf.Set(x, y, '─', st)
		}
	}
}

func (h *ResizeHandle) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		h.Dragging.Store(true)
		return true
	}
	if ev.Action == input.MouseDrag && h.Dragging.Load() {
		if h.OnDrag != nil {
			h.OnDrag(ev, area)
		}
		if h.OnResize != nil && h.Ratio != nil {
			h.OnResize(*h.Ratio)
		}
		return true
	}
	if ev.Action == input.MouseRelease && h.Dragging.Load() {
		h.Dragging.Store(false)
		return true
	}
	return false
}
