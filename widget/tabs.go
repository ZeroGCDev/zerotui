package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// Tabs is a horizontal tab strip switching between named panels (e.g. "POSITIONS | ORDERS | BLOTTER | RISK"). It only draws the strip; pair it with your own conditional rendering of the active panel's content.
type Tabs struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	FocusMixin
	Titles     []string
	Active     int
	OnChange   func(index int)
	Background *color.Color // nil = inherit whatever's behind it (default); fills inactive tabs
}

func NewTabs(titles []string) *Tabs { return &Tabs{Titles: titles} }

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (t *Tabs) OwnsBackground() bool { return t.Background != nil }

func (t *Tabs) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if t.ThemeOverride != nil {
		theme = t.ThemeOverride
	}
	if area.H < 1 {
		return
	}
	if t.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', style.Style{Bg: *t.Background})
	}
	x := area.X
	for i, title := range t.Titles {
		st := bgOr(theme.TextMuted, t.Background)
		labelW := len(title) + 2
		if i == t.Active {
			st = theme.Selected // intentionally ignores Background: selection must stay legible
		} else if t.focused {
			st = bgOr(theme.Info, t.Background)
		}
		buf.SetString(x, area.Y, " ", st)
		buf.SetString(x+1, area.Y, title, st)
		buf.SetString(x+1+len(title), area.Y, " ", st)
		x += labelW + 1
	}
	// A second row becomes a modern active-tab underline when space allows it.
	if area.H >= 2 && t.Active >= 0 && t.Active < len(t.Titles) {
		x = area.X
		for i, title := range t.Titles {
			labelW := len(title) + 2
			if i == t.Active {
				buf.FillRect(x, area.Y+1, labelW, 1, '━', theme.BorderFocus)
			}
			x += labelW + 1
		}
	}
}

func (t *Tabs) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyLeft:
		t.set(t.Active - 1)
		return true
	case input.KeyRight:
		t.set(t.Active + 1)
		return true
	}
	return false
}

func (t *Tabs) set(i int) {
	if i < 0 {
		i = 0
	}
	if i >= len(t.Titles) {
		i = len(t.Titles) - 1
	}
	t.Active = i
	if t.OnChange != nil {
		t.OnChange(i)
	}
}

func (t *Tabs) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action != input.MousePress || !area.Contains(ev.X, ev.Y) {
		return false
	}
	x := area.X
	for i, title := range t.Titles {
		w := len(title) + 2
		if ev.X >= x && ev.X < x+w {
			t.set(i)
			return true
		}
		x += w + 1
	}
	return false
}
