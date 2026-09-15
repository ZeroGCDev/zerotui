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
	Scroll     int
	ShowClose  bool
	Modified   []bool
	OnChange   func(index int)
	OnClose    func(index int) bool
	Background *color.Color // nil = inherit whatever's behind it (default); fills inactive tabs
}

func NewTabs(titles []string) *Tabs { return &Tabs{Titles: titles} }

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (t *Tabs) OwnsBackground() bool { return t.Background != nil }

// TabWidth exposes the deterministic hit-test width to sibling packages.
// It is primarily useful to adapters that need to preserve legacy hit testing.
func (t *Tabs) TabWidth(i int) int {
	if i < 0 || i >= len(t.Titles) {
		return 0
	}
	w := CellWidth(t.Titles[i]) + 2
	if t.ShowClose {
		w += 3
	}
	if w < 4 {
		w = 4
	}
	return w
}

func (t *Tabs) normalize(width int) {
	if len(t.Titles) == 0 || width <= 0 {
		t.Scroll = 0
		return
	}
	if t.Active < 0 {
		t.Active = 0
	}
	if t.Active >= len(t.Titles) {
		t.Active = len(t.Titles) - 1
	}
	if t.Scroll < 0 {
		t.Scroll = 0
	}
	if t.Scroll > t.Active {
		t.Scroll = t.Active
	}
	for t.Scroll < t.Active {
		w := 0
		for i := t.Scroll; i <= t.Active; i++ {
			w += t.TabWidth(i)
		}
		if w <= width {
			break
		}
		t.Scroll++
	}
}

func (t *Tabs) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if t.ThemeOverride != nil {
		theme = t.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	if t.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', style.Style{Bg: *t.Background})
	}
	t.normalize(area.W)
	x := area.X
	for i := t.Scroll; i < len(t.Titles); i++ {
		w := t.TabWidth(i)
		if x+w > area.X+area.W {
			break
		}
		st := bgOr(theme.TextMuted, t.Background)
		if i == t.Active {
			st = theme.Selected
		} else if t.focused {
			st = bgOr(theme.Info, t.Background)
		}
		buf.FillRect(x, area.Y, w, 1, ' ', st)
		buf.SetString(x+1, area.Y, t.Titles[i], st)
		if i < len(t.Modified) && t.Modified[i] {
			buf.SetString(x+1+CellWidth(t.Titles[i]), area.Y, " •", st)
		}
		if t.ShowClose {
			closeSt := st
			if i == t.Active {
				closeSt = theme.Text
			}
			buf.SetString(x+w-2, area.Y, "×", closeSt)
		}
		x += w
	}
	if area.H >= 2 && t.Active >= 0 && t.Active < len(t.Titles) {
		x = area.X
		for i := t.Scroll; i < t.Active; i++ {
			x += t.TabWidth(i)
		}
		if x < area.X+area.W {
			w := t.TabWidth(t.Active)
			if x+w > area.X+area.W {
				w = area.X + area.W - x
			}
			if w > 0 {
				buf.FillRect(x, area.Y+1, w, 1, '━', theme.BorderFocus)
			}
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
	if i == t.Active {
		return
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
	t.normalize(area.W)
	x := area.X
	for i := t.Scroll; i < len(t.Titles); i++ {
		w := t.TabWidth(i)
		if x+w > area.X+area.W {
			break
		}
		if ev.X >= x && ev.X < x+w {
			if (t.ShowClose && ev.X >= x+w-3) || ev.Button == input.MouseMiddle {
				if t.OnClose != nil {
					return t.OnClose(i)
				}
				return true
			}
			t.set(i)
			return true
		}
		x += w
	}
	return false
}
