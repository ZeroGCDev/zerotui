package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
	"sync"
)

// Order is a compact order-blotter row.
type Order struct {
	ID, Symbol, Side, Status string
	Price, Qty, Filled       uint64
}

// Orders renders a compact order blotter.
type Orders struct {
	ThemeOverride               *style.Theme
	Background                  *color.Color
	mu                          sync.RWMutex
	rows                        []Order
	count                       int
	PriceDecimals, ShowDecimals int
	scratch                     [48]byte
	dirty                       bool
	dirtyHeader                 bool
	dirtyWords                  []uint64
}

func NewOrders(capacity, priceDecimals, showDecimals int) *Orders {
	if capacity < 1 {
		capacity = 1
	}
	return &Orders{rows: make([]Order, capacity), dirtyWords: make([]uint64, (capacity+63)/64), PriceDecimals: priceDecimals, ShowDecimals: showDecimals, dirty: true, dirtyHeader: true}
}
func (o *Orders) OwnsBackground() bool { return o.Background != nil }

// PreferredHeight includes the header plus one row per visible order. The
// enclosing layout can therefore keep a small order blotter compact.
func (o *Orders) PreferredHeight() int {
	o.mu.RLock()
	n := o.count
	o.mu.RUnlock()
	return n + 1
}
func (o *Orders) markDirtyRow(i int) {
	if i < 0 {
		return
	}
	word := i >> 6
	bit := uint(i & 63)
	if word >= len(o.dirtyWords) {
		return
	}
	o.dirtyWords[word] |= uint64(1) << bit
}

func (o *Orders) SetRows(v []Order) bool {
	o.mu.Lock()
	oldCount := o.count
	changed := oldCount != len(v)
	maxChanged := oldCount
	if len(v) > maxChanged {
		maxChanged = len(v)
	}
	if len(v) > len(o.rows) {
		capHint := len(v) * 2
		if capHint < 1 {
			capHint = 1
		}
		o.rows = make([]Order, capHint)
		o.dirtyWords = make([]uint64, (capHint+63)/64)
	}
	if len(o.dirtyWords) < (len(o.rows)+63)/64 {
		o.dirtyWords = make([]uint64, (len(o.rows)+63)/64)
	}
	for i := 0; i < maxChanged; i++ {
		var before Order
		if i < oldCount {
			before = o.rows[i]
		}
		var after Order
		if i < len(v) {
			after = v[i]
		}
		if before != after {
			o.markDirtyRow(i)
			changed = true
		}
	}
	copy(o.rows, v)
	o.count = len(v)
	if changed {
		o.dirty = true
	}
	o.mu.Unlock()
	return changed
}

// DirtyRegions keeps order-book style updates localized to the header and the
// rows that can actually change. The app provides the destination storage.
func (o *Orders) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	o.mu.Lock()
	if !o.dirty || area.W <= 0 || area.H <= 0 {
		o.mu.Unlock()
		return dst[:0]
	}
	header := o.dirtyHeader
	o.dirty = false
	o.dirtyHeader = false
	n := o.count
	words := o.dirtyWords
	visible := n
	if visible+1 > area.H {
		visible = area.H - 1
	}
	if visible < 0 {
		visible = 0
	}
	dst = dst[:0]
	if header {
		dst = append(dst, geometry.Rect{X: area.X, Y: area.Y, W: area.W, H: 1})
	}
	for i := 0; i < len(words)*64 && i < area.H-1; i++ {
		if words[i>>6]&(uint64(1)<<uint(i&63)) != 0 {
			dst = append(dst, geometry.Rect{X: area.X, Y: area.Y + 1 + i, W: area.W, H: 1})
		}
	}
	for i := range words {
		words[i] = 0
	}
	o.mu.Unlock()
	return dst
}

func (o *Orders) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if o.ThemeOverride != nil {
		th = o.ThemeOverride
	}
	if a.W < 24 || a.H < 1 {
		return
	}
	if o.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *o.Background})
	}
	muted := bgOr(th.TextMuted, o.Background)
	b.SetString(a.X, a.Y, "ORDER", muted)
	b.SetString(a.X+a.W/4, a.Y, "SIDE", muted)
	b.SetString(a.X+a.W/2, a.Y, "PRICE", muted)
	b.SetString(a.X+a.W*2/3, a.Y, "QTY", muted)
	b.SetString(a.X+a.W*3/4, a.Y, "STATUS", muted)
	rows := a.H - 1
	if rows <= 0 {
		return
	}
	o.mu.RLock()
	n := o.count
	if n > rows {
		n = rows
	}
	for i := 0; i < rows; i++ {
		y := a.Y + 1 + i
		b.FillRect(a.X, y, a.W, 1, ' ', bgOr(th.Panel, o.Background))
		if i >= n {
			continue
		}
		v := o.rows[i]
		st := bgOr(th.Text, o.Background)
		if v.Side == "BUY" || v.Side == "Buy" {
			st = bgOr(th.Positive, o.Background)
		} else if v.Side == "SELL" || v.Side == "Sell" {
			st = bgOr(th.Negative, o.Background)
		}
		b.SetString(a.X, y, v.Symbol, st)
		b.SetString(a.X+a.W/4, y, v.Side, st)
		out := o.scratch[:0]
		out = numfmt.AppendFixedPrec(out, v.Price, o.PriceDecimals, o.ShowDecimals)
		b.SetBytes(a.X+a.W/2, y, out, st)
		out = o.scratch[:0]
		out = numfmt.AppendFixedPrec(out, v.Qty, 2, 2)
		b.SetBytes(a.X+a.W*2/3, y, out, st)
		b.SetString(a.X+a.W*3/4, y, v.Status, st)
	}
	o.mu.RUnlock()
}
