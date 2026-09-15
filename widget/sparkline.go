package widget

import (
	"sync"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

var sparkBlocks = [...]rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

/*
Sparkline is a fixed-capacity ring buffer rendered as a mini bar chart the classic tick-by-tick price/PNL strip in a trading dashboard.

Push is O(1) and allocates nothing: the backing array is sized once at construction. Push is commonly called from a market-data goroutine while Draw runs on the render goroutine.

A ring-buffer read/write isn't a single machine word, so - unlike the atomic-scalar widgets (Toggle, Slider, PriceTicker) - Sparkline uses a mutex rather than sync/atomic to stay race-free.

At UI-refresh cadence (tens to low hundreds of Hz) this costs nothing measurable; it does not allocate.
*/
type Sparkline struct {
	// ThemeOverride optionally replaces the application theme for this component only.
	// It is a pointer to a caller-owned theme, so steady-state rendering adds no allocations.
	ThemeOverride *style.Theme
	mu            sync.RWMutex
	data          []float64
	head          int
	count         int
	UpStyle       *style.Style
	DownStyle     *style.Style
	Background    *color.Color // nil = inherit whatever's behind it (default)
	last          float64
	haveLast      bool
	scratch       []float64 // reused render-side snapshot; allocated once at construction
}

func NewSparkline(capacity int) *Sparkline {
	if capacity < 1 {
		capacity = 1
	}
	return &Sparkline{data: make([]float64, capacity), scratch: make([]float64, capacity)}
}

// Push appends a new sample, overwriting the oldest once at capacity.
// Safe to call from any goroutine, concurrently with Draw.
func (s *Sparkline) Push(v float64) {
	s.mu.Lock()
	s.data[s.head] = v
	s.head++
	if s.head == len(s.data) {
		s.head = 0
	}
	if s.count < len(s.data) {
		s.count++
	}
	s.haveLast = true
	s.last = v
	s.mu.Unlock()
}

// OwnsBackground reports whether this widget establishes an opaque backdrop.
func (s *Sparkline) OwnsBackground() bool { return s.Background != nil }

// DirtyRegions reports the single terminal row occupied by the sparkline.
// Sparkline is intentionally a one-row chart, so high-frequency Push calls
// only repaint that row instead of invalidating its entire placement.
func (s *Sparkline) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	if area.W <= 0 || area.H <= 0 || len(dst) == 0 {
		return nil
	}
	dst[0] = geometry.Rect{X: area.X, Y: area.Y, W: area.W, H: 1}
	return dst[:1]
}

func (s *Sparkline) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if s.ThemeOverride != nil {
		theme = s.ThemeOverride
	}
	if area.H <= 0 || area.W <= 0 {
		return
	}
	if s.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *s.Background})
	}

	// Copy only the visible tail while holding the lock, then release it before
	// doing all min/max and cell rendering work. This keeps a high-frequency
	// market-data Push from ever waiting behind terminal rendering.
	s.mu.RLock()
	n := s.count
	if n > area.W {
		n = area.W
	}
	if n <= 0 {
		s.mu.RUnlock()
		return
	}
	start := s.head - n
	if start < 0 {
		start += len(s.data)
	}
	for i := 0; i < n; i++ {
		s.scratch[i] = s.data[start]
		start++
		if start == len(s.data) {
			start = 0
		}
	}
	s.mu.RUnlock()

	min, max := s.scratch[0], s.scratch[0]
	for i := 1; i < n; i++ {
		v := s.scratch[i]
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span == 0 {
		span = 1
	}

	up := bgOr(theme.Positive, s.Background)
	if s.UpStyle != nil {
		up = bgOr(*s.UpStyle, s.Background)
	}
	down := bgOr(theme.Negative, s.Background)
	if s.DownStyle != nil {
		down = bgOr(*s.DownStyle, s.Background)
	}

	prev := s.scratch[0]
	xOff := area.W - n
	for i := 0; i < n; i++ {
		v := s.scratch[i]
		level := int(((v - min) / span) * float64(len(sparkBlocks)-1))
		if level < 0 {
			level = 0
		} else if level >= len(sparkBlocks) {
			level = len(sparkBlocks) - 1
		}
		st := up
		if v < prev {
			st = down
		}
		buf.Set(area.X+xOff+i, area.Y, sparkBlocks[level], st)
		prev = v
	}
}

// at maps a logical index [0..n) - 0 oldest, n-1 newest of the last n
// samples - to a physical ring index.
func (s *Sparkline) at(n, i int) int {
	start := s.oldestIndexFor(n)
	return (start + i) % len(s.data)
}

func (s *Sparkline) oldestIndexFor(n int) int {
	return (s.head - n + len(s.data)*2) % len(s.data)
}

// Last returns the most recently pushed value.
func (s *Sparkline) Last() (float64, bool) {
	s.mu.RLock()
	v, ok := s.last, s.haveLast
	s.mu.RUnlock()
	return v, ok
}
