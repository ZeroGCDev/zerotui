package widget

import (
	"sync/atomic"
	"unicode/utf8"

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
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
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
	if max < min {
		min, max = max, min
	}
	return &Slider{
		Label: label, Value: value, Min: min, Max: max, Step: step,
		TrackWidth: 20, Format: format,
	}
}

func (s *Slider) get() uint32 {
	if s == nil || s.Value == nil {
		return s.Min
	}
	return atomic.LoadUint32(s.Value)
}

func (s *Slider) addClamped(delta int32) {
	if s == nil || s.Value == nil {
		return
	}
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
	if s == nil || s.Value == nil {
		return
	}
	v := uint32(float64(s.Min) + ratio*float64(s.Max-s.Min))
	atomic.StoreUint32(s.Value, v)
}

// trackGeometry returns the exact geometry used by Draw and mouse input. If
// the placement is too narrow to fit the value after a usable track, the value
// is intentionally omitted so the track remains interactive instead of
// collapsing into an invisible control.
func (s *Slider) trackGeometry(area geometry.Rect, valueWidth int) (trackX, trackW int, showValue bool) {
	if area.W <= 0 {
		return area.X, 0, false
	}
	labelW := CellWidth(s.Label)
	trackX = area.X + labelW + 2 // one cell after the label, then '['
	maxTrack := area.W - labelW - valueWidth - 4
	showValue = valueWidth > 0 && maxTrack > 0
	if !showValue {
		// Narrow controls keep a compact label + track and drop the value text.
		maxTrack = area.W - labelW - 4
	}
	if maxTrack < 0 {
		maxTrack = 0
	}
	trackW = s.TrackWidth
	if trackW > maxTrack {
		trackW = maxTrack
	}
	if trackW < 0 {
		trackW = 0
	}
	return trackX, trackW, showValue
}

func (s *Slider) formatValue(v uint32) []byte {
	dst := s.scratch[:0]
	if s.Format != nil {
		return s.Format(dst, v)
	}
	return numfmt.AppendUint(dst, uint64(v))
}

func byteCellWidth(src []byte) int {
	w := 0
	for i := 0; i < len(src); {
		r, n := utf8.DecodeRune(src[i:])
		if n == 0 {
			n = 1
		}
		rw := buffer.RuneWidth(r)
		if rw == 0 {
			rw = 1
		}
		w += rw
		i += n
	}
	return w
}

func (s *Slider) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if s.ThemeOverride != nil {
		theme = s.ThemeOverride
	}
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

	val := s.get()
	if val < s.Min {
		val = s.Min
	} else if val > s.Max {
		val = s.Max
	}
	valBuf := s.formatValue(val)
	trackX, tw, showValue := s.trackGeometry(area, byteCellWidth(valBuf))
	if trackX-1 >= area.X && trackX-1 < area.X+area.W {
		buf.SetString(trackX-1, area.Y, "[", bgOr(theme.TextMuted, s.Background))
	}

	ratio := 0.0
	if s.Max > s.Min {
		ratio = float64(val-s.Min) / float64(s.Max-s.Min)
	}
	// Keep a dedicated thumb cell inside the track. Using tw for the
	// interpolation previously made the thumb land at index tw when the
	// value reached Max, so the closing bracket replaced the thumb and a
	// 100% slider looked like a completely filled line with no handle.
	thumb := 0
	if tw > 1 {
		thumb = int(ratio * float64(tw-1))
		if thumb >= tw {
			thumb = tw - 1
		}
	}
	for i := 0; i < tw; i++ {
		var ch rune
		var st style.Style
		switch {
		case i < thumb:
			ch, st = '━', theme.TrackFull
		case i == thumb:
			ch, st = '█', theme.Info
		default:
			ch, st = '─', theme.TrackEmpty
		}
		buf.Set(trackX+i, area.Y, ch, bgOr(st, s.Background))
	}
	if trackX+tw >= area.X && trackX+tw < area.X+area.W {
		buf.SetString(trackX+tw, area.Y, "]", bgOr(theme.TextMuted, s.Background))
	}

	if showValue {
		valueX := trackX + tw + 2
		if valueX < area.X+area.W {
			buf.SetBytes(valueX, area.Y, valBuf, bgOr(theme.Text, s.Background))
		}
	}
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
	valueBuf := s.formatValue(s.get())
	trackX, tw, _ := s.trackGeometry(area, byteCellWidth(valueBuf))
	if tw <= 0 {
		return false
	}
	// Treat the whole visible slider track, including its endpoints, as the
	// interactive range. Mouse coordinates are mapped against the exact same
	// width that Draw uses after responsive clamping.
	rel := 0.0
	if tw > 1 {
		rel = float64(ev.X-trackX) / float64(tw-1)
	}
	s.setFromRatio(rel)
	return true
}
