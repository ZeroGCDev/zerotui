package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// Stat renders a compact two-line KPI card. It is useful for modern dashboard
// headers without introducing a rich-text or formatted-string dependency.
type Stat struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color
	Label         string
	Value         string
	Delta         string
	Up            bool
	Down          bool
}

func (s *Stat) OwnsBackground() bool { return s.Background != nil }

func NewStat(label, value string) *Stat { return &Stat{Label: label, Value: value} }

func (s *Stat) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if s.ThemeOverride != nil {
		theme = s.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	if s.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, minInt(area.H, 3), ' ', style.Style{Bg: *s.Background})
	}
	buf.SetString(area.X, area.Y, s.Label, bgOr(theme.TextMuted, s.Background))
	if area.H < 2 {
		return
	}
	buf.SetString(area.X, area.Y+1, s.Value, bgOr(theme.Text, s.Background))
	if s.Delta != "" && area.H >= 3 {
		st := bgOr(theme.Info, s.Background)
		if s.Up {
			st = theme.Positive
		} else if s.Down {
			st = theme.Negative
		}
		buf.SetString(area.X, area.Y+2, s.Delta, bgOr(st, s.Background))
	}
}
