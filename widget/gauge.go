package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
)

/*
Gauge is a non-interactive progress/utilization bar, e.g. margin usage or risk budget consumed. Value is fine to set directly ONLY from the same goroutine that calls App.Run (the render goroutine). If a separate feed/risk goroutine needs to update it - the common case - store the reading behind your own atomic (e.g. math.Float64bits in an atomic.Uint64) and set ValueFn to load and decode it; ValueFn is called from Draw on every frame and takes priority over Value when non-nil.
*/
type Gauge struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	Label         string
	Value         float64 // 0..1
	ValueFn       func() float64
	Style         *style.Style // nil = theme.Positive, overridden by thresholds
	WarnAt        float64      // >0: switch to theme.Warning above this ratio
	DangerAt      float64      // >0: switch to theme.Negative above this ratio
	Background    *color.Color // nil = inherit whatever's behind it (default)
	scratch       [8]byte
}

func NewGauge(label string) *Gauge { return &Gauge{Label: label} }

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (g *Gauge) OwnsBackground() bool { return g.Background != nil }

func (g *Gauge) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if g.ThemeOverride != nil {
		theme = g.ThemeOverride
	}
	if area.H <= 0 {
		return
	}
	v := g.Value
	if g.ValueFn != nil {
		v = g.ValueFn()
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	st := theme.Positive
	if g.Style != nil {
		st = *g.Style
	}
	if g.DangerAt > 0 && v >= g.DangerAt {
		st = theme.Negative
	} else if g.WarnAt > 0 && v >= g.WarnAt {
		st = theme.Warning
	}
	st = bgOr(st, g.Background)

	if g.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', style.Style{Bg: *g.Background})
	}

	labelW := len(g.Label)
	buf.SetString(area.X, area.Y, g.Label, bgOr(theme.TextMuted, g.Background))

	barX := area.X + labelW + 1
	barW := area.W - labelW - 7
	if barW < 1 {
		barW = 1
	}
	filled := int(v * float64(barW))
	buf.FillRect(barX, area.Y, barW, 1, '░', bgOr(theme.TrackEmpty, g.Background))
	buf.FillRect(barX, area.Y, filled, 1, '█', st)

	pct := g.scratch[:0]
	pct = numfmt.AppendUint(pct, uint64(v*100))
	pct = append(pct, '%')
	buf.SetBytes(barX+barW+1, area.Y, pct, bgOr(theme.Text, g.Background))
}
