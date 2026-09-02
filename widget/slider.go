package widget

import (
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
)

// SliderFormat renders v into dst (dst[:0]-style reuse) for zero-alloc value display. See FormatBasisPointsPct / FormatInt for ready-made ones.
type SliderFormat func(dst []byte, v uint32) []byte

// FormatBasisPointsPct renders a basis-point value (200 = 2.00%) with a leading sign, matching the SL/TP display in the reference terminal.
func FormatBasisPointsPct(sign byte) SliderFormat {
	return func(dst []byte, v uint32) []byte {
		dst = append(dst, sign)
		dst = numfmt.AppendFixed(dst, uint64(v), 2)
		return append(dst, '%')
	}
}

// FormatInt renders a plain integer with a suffix, e.g. "10x" leverage.
func FormatInt(suffix string) SliderFormat {
	return func(dst []byte, v uint32) []byte {
		dst = numfmt.AppendUint(dst, uint64(v))
		return append(dst, suffix...)
	}
}

// Slider is a horizontal track bound to an atomic uint32, driven by keyboard (Left/Right or h/l once routed by the app) and mouse click/drag. This generalizes the SL%/TP%/Leverage tracks from the reference terminal into one reusable, styleable component.
type Slider struct {
	FocusMixin
	Label      string
	Value      *uint32
	Min, Max   uint32
	Step       uint32
	TrackWidth int
	Format     SliderFormat
	Background *color.Color // nil = inherit whatever's behind it (default)
	scratch    [24]byte
}

func NewSlider(label string, value *uint32, min, max, step uint32, format SliderFormat) *Slider {
	return &Slider{
		Label: label, Value: value, Min: min, Max: max, Step: step,
		TrackWidth: 20, Format: format,
	}
}

func (s *Slider) get() uint32 { return atomic.LoadUint32(s.Value) }

func (s *Slider) addClamped(delta int32) {
	for {
		cur := atomic.LoadUint32(s.Value)
		next := int64(cur) + int64(delta)
		if next < int64(s.Min) {
			next = int64(s.Min)
		}
		if next > int64(s.Max) {
			next = int64(s.Max)
		}
		if atomic.CompareAndSwapUint32(s.Value, cur, uint32(next)) {
			return
		}
	}
}

func (s *Slider) setFromRatio(ratio float64) {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	v := uint32(float64(s.Min) + ratio*float64(s.Max-s.Min))
	atomic.StoreUint32(s.Value, v)
}

// Draw renders "Label : [ ━━━━━█──── ] value" on a single row.
// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (s *Slider) OwnsBackground() bool { return s.Background != nil }

func (s *Slider) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.H <= 0 || area.W <= 0 {
		return
	}
	if s.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, 1, ' ', style.Style{Bg: *s.Background})
	}

	labelSt := bgOr(theme.TextMuted, s.Background)
	if s.focused {
		labelSt = bgOr(theme.Info, s.Background)
	}
	buf.SetString(area.X, area.Y, s.Label, labelSt)

	trackX := area.X + len(s.Label) + 2
	buf.SetString(trackX-1, area.Y, "[", bgOr(theme.TextMuted, s.Background))

	val := s.get()
	tw := s.TrackWidth
	if tw > area.W-len(s.Label)-14 {
		tw = area.W - len(s.Label) - 14
	}
	if tw < 3 {
		tw = 3
	}
	ratio := 0.0
	if s.Max > s.Min {
		ratio = float64(val-s.Min) / float64(s.Max-s.Min)
	}
	filled := int(ratio * float64(tw))
	if filled > tw {
		filled = tw
	}
	for i := 0; i < tw; i++ {
		var ch rune
		var st style.Style
		switch {
		case i < filled:
			ch, st = '━', theme.TrackFull
		case i == filled:
			ch, st = '█', theme.Info
		default:
			ch, st = '─', theme.TrackEmpty
		}
		buf.Set(trackX+i, area.Y, ch, bgOr(st, s.Background))
	}
	buf.SetString(trackX+tw, area.Y, "]", bgOr(theme.TextMuted, s.Background))

	valBuf := s.scratch[:0]
	if s.Format != nil {
		valBuf = s.Format(valBuf, val)
	} else {
		valBuf = numfmt.AppendUint(valBuf, uint64(val))
	}
	buf.SetBytes(trackX+tw+2, area.Y, valBuf, bgOr(theme.Text, s.Background))
}

func (s *Slider) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyLeft:
		s.addClamped(-int32(s.Step))
		return true
	case input.KeyRight:
		s.addClamped(int32(s.Step))
		return true
	case input.KeyRune:
		switch k.Rune {
		case 'h':
			s.addClamped(-int32(s.Step))
			return true
		case 'l':
			s.addClamped(int32(s.Step))
			return true
		}
	}
	return false
}

func (s *Slider) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action != input.MousePress && ev.Action != input.MouseDrag {
		return false
	}
	if ev.Action == input.MousePress && !area.Contains(ev.X, ev.Y) {
		return false
	}
	trackX := area.X + len(s.Label) + 2
	tw := s.TrackWidth
	rel := float64(ev.X-trackX) / float64(tw)
	s.setFromRatio(rel)
	return true
}
