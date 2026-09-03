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

	// Labels are commonly used for compact blocks of copy in examples and
	// dashboards. SetString intentionally treats a string as one terminal row,
	// so do the line walking here instead of ever writing a literal newline into
	// the cell buffer. A newline stored in a cell would be emitted as an ANSI
	// control character by the terminal renderer and could move the real cursor
	// away from the buffer's tracked position, producing missing widgets and
	// stray horizontal/vertical lines.
	row := area.Y
	col := area.X
	for _, r := range text {
		if row >= area.Y+area.H {
			break
		}
		if r == '\n' {
			row++
			col = area.X
			continue
		}
		if col >= area.X+area.W {
			continue
		}
		if col >= area.X {
			buf.Set(col, row, r, st)
		}
		col++
	}
}
