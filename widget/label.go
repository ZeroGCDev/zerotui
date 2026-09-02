package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

/*
Label draws static or externally-updated text. It never allocates in Draw; SetText replaces the string reference (allocation happens there, on the UI-update path, not the render path).

SetText is only safe to call from the render goroutine; if another goroutine (e.g. a feed status callback) needs to update it, set TextFn instead - same pattern as Gauge.ValueFn - reading from your own atomic.Value/atomic.Uint64.
*/
type Label struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Text          string
	TextFn        func() string // if set, takes priority over Text every Draw
	Style         *style.Style  // nil = theme.Text
	Bold          bool
	Background    *color.Color // nil = inherit whatever's behind it (default)
}

func NewLabel(text string) *Label { return &Label{Text: text} }

func (l *Label) SetText(s string) { l.Text = s }

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (l *Label) OwnsBackground() bool { return l.Background != nil }

func (l *Label) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if l.ThemeOverride != nil {
		theme = l.ThemeOverride
	}
	if area.H <= 0 {
		return
	}
	st := theme.Text
	if l.Style != nil {
		st = *l.Style
	}
	if l.Bold {
		st = st.WithAttr(style.Bold)
	}
	if l.Background != nil {
		st = st.WithBg(*l.Background)
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', st)
	}
	text := l.Text
	if l.TextFn != nil {
		text = l.TextFn()
	}
	buf.SetString(area.X, area.Y, text, st)
}
