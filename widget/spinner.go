package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// Spinner is an allocation-free animated status glyph. Advance with Tick and
// invalidate the app; keeping the clock outside Draw prevents hidden timers.
type Spinner struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color
	Label         string
	Frame         uint8
	Foreground    *color.Color
}

func (s *Spinner) OwnsBackground() bool { return s.Background != nil }

func NewSpinner(label string) *Spinner { return &Spinner{Label: label} }
func (s *Spinner) Tick()               { s.Frame++ }
func (s *Spinner) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if s.ThemeOverride != nil {
		theme = s.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	st := bgOr(theme.Info, s.Background)
	if s.Foreground != nil {
		st = st.WithFg(*s.Foreground)
	}
	if s.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', style.Style{Bg: *s.Background})
	}
	frames := [...]rune{'·', '⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	buf.Set(area.X, area.Y, frames[int(s.Frame)%len(frames)], st)
	buf.SetString(area.X+2, area.Y, s.Label, bgOr(theme.TextMuted, s.Background))
}
