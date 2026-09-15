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
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Text          string
	Background    *color.Color
	Foreground    *color.Color
	Positive      bool
	Negative      bool
	Info          bool
}

func NewBadge(text string) *Badge { return &Badge{Text: text} }

func (b *Badge) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if b.ThemeOverride != nil {
		theme = b.ThemeOverride
	}
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
	textW := CellWidth(b.Text) + 2
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
