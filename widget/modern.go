package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// Badge is a compact status/pill control. It is intentionally string-free in
// the hot path: all geometry is written directly into the cell buffer.
type Badge struct {
	Text       string
	Background *color.Color
	Foreground *color.Color
	Positive   bool
	Negative   bool
	Info       bool
}

func NewBadge(text string) *Badge { return &Badge{Text: text} }

func (b *Badge) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 1 || area.H < 1 {
		return
	}
	st := theme.Text
	switch {
	case b.Negative:
		st = theme.Negative
	case b.Positive:
		st = theme.Positive
	case b.Info:
		st = theme.Info
	}
	if b.Foreground != nil {
		st = st.WithFg(*b.Foreground)
	}
	if b.Background != nil {
		st = st.WithBg(*b.Background)
	}
	buf.FillRect(area.X, area.Y, area.W, 1, ' ', st)
	textW := len(b.Text) + 2
	if textW > area.W {
		textW = area.W
	}
	buf.Set(area.X, area.Y, '⟦', st)
	if textW > 2 {
		buf.SetString(area.X+1, area.Y, b.Text, st)
	}
	if textW > 1 {
		buf.Set(area.X+textW-1, area.Y, '⟧', st)
	}
}

// Divider draws a light structural divider. A dedicated primitive avoids
// forcing users to create a one-off label containing box-drawing characters.
type Divider struct {
	Horizontal bool
}

func NewDivider(horizontal bool) *Divider { return &Divider{Horizontal: horizontal} }

func (d *Divider) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	st := theme.Border
	if d.Horizontal {
		buf.FillRect(area.X, area.Y, area.W, 1, '─', st)
		return
	}
	buf.FillRect(area.X, area.Y, 1, area.H, '│', st)
}

// Stat renders a compact two-line KPI card. It is useful for modern dashboard
// headers without introducing a rich-text or formatted-string dependency.
type Stat struct {
	Label string
	Value string
	Delta string
	Up    bool
	Down  bool
}

func NewStat(label, value string) *Stat { return &Stat{Label: label, Value: value} }

func (s *Stat) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 1 || area.H < 1 {
		return
	}
	buf.SetString(area.X, area.Y, s.Label, theme.TextMuted)
	if area.H < 2 {
		return
	}
	buf.SetString(area.X, area.Y+1, s.Value, theme.Text)
	if s.Delta != "" && area.H >= 3 {
		st := theme.Info
		if s.Up {
			st = theme.Positive
		} else if s.Down {
			st = theme.Negative
		}
		buf.SetString(area.X, area.Y+2, s.Delta, st)
	}
}

// Spinner is an allocation-free animated status glyph. Advance with Tick and
// invalidate the app; keeping the clock outside Draw prevents hidden timers.
type Spinner struct {
	Label      string
	Frame      uint8
	Foreground *color.Color
}

func NewSpinner(label string) *Spinner { return &Spinner{Label: label} }
func (s *Spinner) Tick()               { s.Frame++ }
func (s *Spinner) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 1 || area.H < 1 {
		return
	}
	st := theme.Info
	if s.Foreground != nil {
		st = st.WithFg(*s.Foreground)
	}
	frames := [...]rune{'·', '⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	buf.Set(area.X, area.Y, frames[int(s.Frame)%len(frames)], st)
	buf.SetString(area.X+2, area.Y, s.Label, theme.TextMuted)
}

// ScrollBar is a tiny terminal-native scrollbar shared by virtual views. It
// draws only the viewport track and thumb; it never allocates.
type ScrollBar struct {
	Total    int
	Offset   int
	Viewport int
}

func (s ScrollBar) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 1 || area.H < 1 || s.Total <= s.Viewport {
		return
	}
	buf.FillRect(area.X, area.Y, 1, area.H, '│', theme.TrackEmpty)
	thumb := area.H * s.Viewport / s.Total
	if thumb < 1 {
		thumb = 1
	}
	maxOffset := s.Total - s.Viewport
	pos := 0
	if maxOffset > 0 {
		pos = (area.H - thumb) * s.Offset / maxOffset
	}
	buf.FillRect(area.X, area.Y+pos, 1, thumb, '█', theme.Info)
}

// GradientBar is an allocation-free truecolor progress/heat strip. It is
// intentionally a cell primitive rather than an animation engine: callers can
// change Value and invalidate only when the underlying metric changes.
type GradientBar struct {
	Value float64
	Start color.Color
	End   color.Color
	Track color.Color
}

func NewGradientBar(start, end, track color.Color) *GradientBar {
	return &GradientBar{Start: start, End: end, Track: track}
}

func (g *GradientBar) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
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
		st := style.Style{Fg: track, Bg: theme.Panel.Bg}
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
