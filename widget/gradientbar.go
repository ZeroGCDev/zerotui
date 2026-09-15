package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// GradientBar is an allocation-free truecolor progress/heat strip. It is
// intentionally a cell primitive rather than an animation engine: callers can
// change Value and invalidate only when the underlying metric changes.
type GradientBar struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Background    *color.Color
	Value         float64
	Start         color.Color
	End           color.Color
	Track         color.Color
}

func (g *GradientBar) OwnsBackground() bool { return g.Background != nil }

func NewGradientBar(start, end, track color.Color) *GradientBar {
	return &GradientBar{Start: start, End: end, Track: track}
}

func (g *GradientBar) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if g.ThemeOverride != nil {
		theme = g.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	v := g.Value
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	start, end, track := g.Start, g.End, g.Track
	if start == color.Default {
		start = theme.Negative.Fg
	}
	if end == color.Default {
		end = theme.Positive.Fg
	}
	if track == color.Default {
		track = theme.TrackEmpty.Fg
	}
	filled := int(v * float64(area.W))
	for i := 0; i < area.W; i++ {
		bg := theme.Panel.Bg
		if g.Background != nil {
			bg = *g.Background
		}
		st := style.Style{Fg: track, Bg: bg}
		if i < filled {
			t := uint8(0)
			if area.W > 1 {
				t = uint8(i * 255 / (area.W - 1))
			}
			st.Fg = color.Lerp(start, end, t)
		}
		buf.Set(area.X+i, area.Y, '━', st)
	}
}
