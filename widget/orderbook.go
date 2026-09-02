package widget

import (
	"math/bits"
	"sync"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
)

// Level is one price/size row of a depth ladder.
type Level struct {
	Price uint64 // scaled fixed-point, see Decimals
	Size  uint64 // scaled fixed-point, see SizeDecimals
}

/*
OrderBook renders a two-sided bid/ask depth ladder with size bars.

SetLevels publishes by copying into preallocated storage under a short RWMutex.
The old atomic snapshot path allocated a bookSnapshot on every update; the new
path avoids that steady-state GC pressure while keeping Draw race-free.
*/
type OrderBook struct {
	mu           sync.RWMutex
	bids         []Level
	asks         []Level
	bidCount     int
	askCount     int
	Decimals     int
	SizeDecimals int
	ShowDecimals int
	Background   *color.Color // nil = inherit whatever's behind it
	scratch      [32]byte
	dirtyFrom    int
	dirtyTo      int
	dirtyFull    bool
	bidMaxSize   uint64
	askMaxSize   uint64
}

func NewOrderBook(decimals, sizeDecimals, showDecimals int) *OrderBook {
	return &OrderBook{Decimals: decimals, SizeDecimals: sizeDecimals, ShowDecimals: showDecimals, bids: make([]Level, 32), asks: make([]Level, 32), dirtyFrom: 0, dirtyTo: 0, dirtyFull: true, bidMaxSize: 1, askMaxSize: 1}
}

// SetLevels copies the visible depth into preallocated storage. The copy is tiny
// (normally 8-20 levels) but removes the old per-update snapshot allocation.
func (o *OrderBook) SetLevels(bids, asks []Level) {
	o.mu.Lock()
	oldBidCount, oldAskCount := o.bidCount, o.askCount
	oldBidMax, oldAskMax := o.sideMaxSizesLocked(oldBidCount, oldAskCount)

	changedFrom, changedTo := -1, -1
	limit := oldBidCount
	if oldAskCount > limit {
		limit = oldAskCount
	}
	if len(bids) > limit {
		limit = len(bids)
	}
	if len(asks) > limit {
		limit = len(asks)
	}
	// Compare before growing/replacing backing storage so a >32-level resize
	// does not erase the old snapshot before change detection.
	for i := 0; i < limit; i++ {
		var ob, oa Level
		if i < oldBidCount {
			ob = o.bids[i]
		}
		if i < oldAskCount {
			oa = o.asks[i]
		}
		var nb, na Level
		if i < len(bids) {
			nb = bids[i]
		}
		if i < len(asks) {
			na = asks[i]
		}
		if ob != nb || oa != na {
			if changedFrom < 0 {
				changedFrom = i
			}
			changedTo = i
		}
	}

	if len(bids) > len(o.bids) {
		capHint := len(bids) * 2
		if capHint < 32 {
			capHint = 32
		}
		o.bids = make([]Level, capHint)
	}
	if len(asks) > len(o.asks) {
		capHint := len(asks) * 2
		if capHint < 32 {
			capHint = 32
		}
		o.asks = make([]Level, capHint)
	}
	copy(o.bids, bids)
	copy(o.asks, asks)
	o.bidCount = len(bids)
	o.askCount = len(asks)

	newBidMax, newAskMax := o.sideMaxSizesLocked(o.bidCount, o.askCount)
	o.bidMaxSize, o.askMaxSize = newBidMax, newAskMax
	if oldBidMax != newBidMax || oldAskMax != newAskMax {
		o.dirtyFull = true
		maxRows := oldBidCount
		if oldAskCount > maxRows {
			maxRows = oldAskCount
		}
		if o.bidCount > maxRows {
			maxRows = o.bidCount
		}
		if o.askCount > maxRows {
			maxRows = o.askCount
		}
		if maxRows > 0 {
			o.dirtyFrom, o.dirtyTo = 0, maxRows-1
		}
	} else if changedFrom >= 0 && !o.dirtyFull {
		if o.dirtyFrom < 0 || changedFrom < o.dirtyFrom {
			o.dirtyFrom = changedFrom
		}
		if changedTo > o.dirtyTo {
			o.dirtyTo = changedTo
		}
	}
	o.mu.Unlock()
}

func (o *OrderBook) sideMaxSizesLocked(bidCount, askCount int) (uint64, uint64) {
	bidMax, askMax := uint64(1), uint64(1)
	for i := 0; i < bidCount; i++ {
		if o.bids[i].Size > bidMax {
			bidMax = o.bids[i].Size
		}
	}
	for i := 0; i < askCount; i++ {
		if o.asks[i].Size > askMax {
			askMax = o.asks[i].Size
		}
	}
	return bidMax, askMax
}

// DirtyRegions returns the smallest currently known row band affected by the
// latest SetLevels calls. It consumes the pending book damage so a producer can
// publish several bursts before the renderer runs without allocating or losing
// a later, distinct update. A full band is used whenever bar normalization
// changes, because every visible bar depends on the shared maximum size.
func (o *OrderBook) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	o.mu.Lock()
	from, to, full := o.dirtyFrom, o.dirtyTo, o.dirtyFull
	o.dirtyFrom, o.dirtyTo, o.dirtyFull = -1, -1, false
	o.mu.Unlock()
	if area.W <= 0 || area.H <= 1 || from < 0 {
		return dst[:0]
	}
	if full {
		from, to = 0, area.H-2
	}
	if from < 0 {
		from = 0
	}
	if to >= area.H-1 {
		to = area.H - 2
	}
	if from > to {
		return dst[:0]
	}
	dst = dst[:0]
	dst = append(dst, geometry.Rect{X: area.X, Y: area.Y + 1 + from, W: area.W, H: to - from + 1})
	return dst
}

// OwnsBackground marks an explicitly configured book backdrop as opaque.
func (o *OrderBook) OwnsBackground() bool { return o.Background != nil }

func (o *OrderBook) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 10 || area.H < 1 {
		return
	}
	if o.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *o.Background})
	}
	rowFill := bgOr(theme.Panel, o.Background)
	bidSt := bgOr(theme.Positive, o.Background)
	askSt := bgOr(theme.Negative, o.Background)
	mutedSt := bgOr(theme.TextMuted, o.Background)

	o.mu.RLock()
	bids := o.bids[:o.bidCount]
	asks := o.asks[:o.askCount]
	bidMaxSize, askMaxSize := o.bidMaxSize, o.askMaxSize
	decimals, sizeDecimals, showDecimals := o.Decimals, o.SizeDecimals, o.ShowDecimals
	o.mu.RUnlock()

	// The renderer's clip is authoritative, so Draw can repaint the whole logical
	// book while a damage pass cheaply limits the actual cells touched. For very
	// narrow layouts, omit bars instead of allowing columns to overlap.
	half := area.W / 2
	compact := half < 16
	buf.SetString(area.X, area.Y, "BID SIZE PRICE", mutedSt)
	buf.SetString(area.X+half, area.Y, "PRICE ASK SIZE", mutedSt)

	rows := area.H - 1
	clip := buf.Clip()
	rowStart, rowEnd := 0, rows
	if clip.Y > area.Y+1 {
		rowStart = clip.Y - (area.Y + 1)
	}
	if clip.Y+clip.H < area.Y+1+rowEnd {
		rowEnd = clip.Y + clip.H - (area.Y + 1)
	}
	if rowStart < 0 {
		rowStart = 0
	}
	if rowEnd > rows {
		rowEnd = rows
	}
	if rowStart > rowEnd {
		rowStart = rowEnd
	}
	if compact {
		leftMid := area.X + half/2
		rightMid := area.X + half + half/2
		for i := rowStart; i < rowEnd; i++ {
			y := area.Y + 1 + i
			if i < len(bids) {
				lv := bids[i]
				buf.FillRect(area.X, y, half, 1, ' ', rowFill)
				sz := o.scratch[:0]
				sz = numfmt.AppendFixedPrec(sz, lv.Size, sizeDecimals, showDecimals)
				buf.SetBytes(area.X, y, sz, bidSt)
				px := o.scratch[:0]
				px = numfmt.AppendFixedPrec(px, lv.Price, decimals, showDecimals)
				buf.SetBytes(leftMid, y, px, bidSt)
			}
			if i < len(asks) {
				lv := asks[i]
				buf.FillRect(area.X+half, y, area.W-half, 1, ' ', rowFill)
				px := o.scratch[:0]
				px = numfmt.AppendFixedPrec(px, lv.Price, decimals, showDecimals)
				buf.SetBytes(area.X+half, y, px, askSt)
				sz := o.scratch[:0]
				sz = numfmt.AppendFixedPrec(sz, lv.Size, sizeDecimals, showDecimals)
				buf.SetBytes(rightMid, y, sz, askSt)
			}
		}
		return
	}

	// Wide layout: each half is [size][bar][price] for bids and
	// [price][bar][size] for asks. Keep the numbers out of the bar cells so
	// both depth bars remain visible even when the panel is moderately narrow.
	const sizeW = 8
	const priceW = 12
	barW := half - sizeW - priceW
	if barW < 1 {
		// Fall back to the compact layout before allowing columns to overlap.
		leftMid := area.X + half/2
		rightMid := area.X + half + (area.W-half)/2
		for i := rowStart; i < rowEnd; i++ {
			y := area.Y + 1 + i
			if i < len(bids) {
				lv := bids[i]
				buf.FillRect(area.X, y, half, 1, ' ', rowFill)
				sz := o.scratch[:0]
				sz = numfmt.AppendFixedPrec(sz, lv.Size, sizeDecimals, showDecimals)
				buf.SetBytes(area.X, y, sz, bidSt)
				px := o.scratch[:0]
				px = numfmt.AppendFixedPrec(px, lv.Price, decimals, showDecimals)
				buf.SetBytes(leftMid, y, px, bidSt)
			}
			if i < len(asks) {
				lv := asks[i]
				buf.FillRect(area.X+half, y, area.W-half, 1, ' ', rowFill)
				px := o.scratch[:0]
				px = numfmt.AppendFixedPrec(px, lv.Price, decimals, showDecimals)
				buf.SetBytes(area.X+half, y, px, askSt)
				sz := o.scratch[:0]
				sz = numfmt.AppendFixedPrec(sz, lv.Size, sizeDecimals, showDecimals)
				buf.SetBytes(rightMid, y, sz, askSt)
			}
		}
		return
	}

	leftBarX := area.X + sizeW
	leftPriceX := area.X + half - priceW
	rightPriceX := area.X + half
	rightBarX := rightPriceX + priceW
	rightSizeX := area.X + area.W - sizeW

	for i := rowStart; i < rowEnd; i++ {
		y := area.Y + 1 + i
		if i < len(bids) {
			lv := bids[i]
			buf.FillRect(leftBarX, y, barW, 1, ' ', rowFill)
			filled := orderBookBar(lv.Size, bidMaxSize, barW)
			if filled > 0 {
				buf.FillRect(leftBarX+barW-filled, y, filled, 1, '▌', bidSt)
			}
			sz := o.scratch[:0]
			sz = numfmt.AppendFixedPrec(sz, lv.Size, sizeDecimals, showDecimals)
			buf.SetBytes(area.X, y, sz, bidSt)
			px := o.scratch[:0]
			px = numfmt.AppendFixedPrec(px, lv.Price, decimals, showDecimals)
			buf.SetBytes(leftPriceX, y, px, bidSt)
		}
		if i < len(asks) {
			lv := asks[i]
			buf.FillRect(rightBarX, y, barW, 1, ' ', rowFill)
			filled := orderBookBar(lv.Size, askMaxSize, barW)
			if filled > 0 {
				buf.FillRect(rightBarX, y, filled, 1, '▌', askSt)
			}
			px := o.scratch[:0]
			px = numfmt.AppendFixedPrec(px, lv.Price, decimals, showDecimals)
			buf.SetBytes(rightPriceX, y, px, askSt)
			sz := o.scratch[:0]
			sz = numfmt.AppendFixedPrec(sz, lv.Size, sizeDecimals, showDecimals)
			buf.SetBytes(rightSizeX, y, sz, askSt)
		}
	}

}

func orderBookBar(size, maxSize uint64, width int) int {
	if size == 0 || width <= 0 {
		return 0
	}
	if maxSize == 0 || size >= maxSize {
		return width
	}
	// Compute floor(size*width/maxSize) without converting to float and without
	// overflowing uint64 when market-data sizes use the full integer range.
	hi, lo := bits.Mul64(size, uint64(width))
	q, _ := bits.Div64(hi, lo, maxSize)
	if q > uint64(width) {
		return width
	}
	return int(q)
}
