package widget

import (
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

/*
Toggle is an on/off control bound directly to an atomic uint32 (0 or 1), the same pattern the market-data hot path uses for AlgoActive/SLEnabled. Reading/writing it from Draw and HandleKey/HandleMouse never allocates and is safe to touch concurrently from a separate risk-engine goroutine.
*/
type Toggle struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	FocusMixin
	Label      string
	Value      *uint32      // 0 = off, 1 = on
	OnFlag     string       // text shown when Value==1, default "ON"
	OffFlag    string       // text shown when Value==0, default "off"
	Background *color.Color // nil = inherit whatever's behind it (default)
}

func NewToggle(label string, value *uint32) *Toggle {
	return &Toggle{Label: label, Value: value, OnFlag: "ACTIVE", OffFlag: "off"}
}

func (t *Toggle) on() bool { return atomic.LoadUint32(t.Value) == 1 }

func (t *Toggle) flip() {
	for {
		cur := atomic.LoadUint32(t.Value)
		next := uint32(0)
		if cur == 0 {
			next = 1
		}
		if atomic.CompareAndSwapUint32(t.Value, cur, next) {
			return
		}
	}
}

/*
Draw renders "[X] Label  FLAG" for one row. Focus is shown with the same theme.Selected color swap every other interactive widget uses (Button, List, Table, Tabs) - no underline, so it stays legible over any Background override too.
*/
// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (t *Toggle) OwnsBackground() bool { return t.Background != nil }

func (t *Toggle) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if t.ThemeOverride != nil {
		theme = t.ThemeOverride
	}
	if area.H <= 0 {
		return
	}
	on := t.on()
	box, st := "[ ] ", theme.Text
	flag := t.OffFlag
	if on {
		box, st = "[X] ", theme.Positive
		flag = t.OnFlag
	}
	if t.focused {
		st = theme.Selected
	}
	if t.Background != nil {
		st = st.WithBg(*t.Background)
	}
	buf.FillRect(area.X, area.Y, area.W, 1, ' ', st)
	buf.SetString(area.X, area.Y, box, st)
	x := area.X + len(box)
	buf.SetString(x, area.Y, t.Label, st)
	x += len(t.Label)
	buf.SetString(x, area.Y, " "+flag, st)
}

func (t *Toggle) HandleKey(k input.Key) bool {
	if k.Type == input.KeySpace || k.Type == input.KeyEnter {
		t.flip()
		return true
	}
	return false
}

func (t *Toggle) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		t.flip()
		return true
	}
	return false
}
