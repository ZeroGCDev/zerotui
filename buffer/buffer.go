/*
Package buffer implements a double-buffered terminal cell grid with a diff-based renderer, the core of zerotui's render hot path.

Design: two flat []Cell slices (front = last frame shown, back = frame being built) are allocated once at startup / resize and reused forever.

Render() walks both, emits ANSI only for cells that actually changed, swaps the buffers, and writes through a single persistent bytes.Buffer and byte scratch array. Steady-state redraws allocate nothing.
*/
package buffer

import (
	"io"
	"math/bits"
	"unicode/utf8"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/style"
)

type Cell struct {
	Ch    rune
	Style style.Style
}

var blank = Cell{Ch: ' '}

var syncBegin = [...]byte{0x1b, '[', '?', '2', '0', '2', '6', 'h'}
var syncEnd = [...]byte{0x1b, '[', '?', '2', '0', '2', '6', 'l'}

type Buffer struct {
	W, H  int
	front []Cell
	back  []Cell

	// dirty is a bitset of back-buffer cells whose contents may differ from front.
	// Mutations mark bits; RenderRegions clears them as cells become synchronized.
	// This lets unchanged frames inspect a handful of machine words instead of
	// scanning every terminal cell.
	dirty     []uint64
	dirtyRows []uint64
	// Per-row dirty bounds provide a second sparse-rejection layer. A row with
	// one changed cell scans only the affected word range instead of the whole row.
	dirtyMin []int
	dirtyMax []int

	out     []byte
	scratch [32]byte

	// last SGR state written to the terminal, to avoid re-emitting an escape sequence when consecutive changed cells share a style.
	lastFg, lastBg color.Color
	lastAttr       style.Attr
	sgrValid       bool
	outCursorX     int
	outCursorY     int
	outCursorValid bool
	outCursorASCII bool

	// Cached output state. out reserves aggressively once so steady-state
	// frames do not repeatedly grow the terminal byte buffer.
	outCapacityHint int

	// forceFull is set when terminal output fails or is short-written. The
	// front buffer may already contain cells that were encoded before the
	// failure, so the next render must conservatively repaint the entire
	// screen instead of assuming the terminal received those bytes.
	forceFull bool

	// clip limits drawing operations to a damage region. The retained-mode
	// app sets this before repainting a region so widgets can remain oblivious
	// to partial redraws while untouched cells stay resident.
	clip    Rect
	clipped bool

	// sgrCache stores encoded styles in fixed-size entries. Modern dashboards
	// reuse a small palette thousands of times per frame, so caching the ANSI
	// encoding removes repeated color-component formatting from Buffer.Render
	// without introducing a map, heap churn, or synchronization.
	sgrCache [64]sgrCacheEntry

	// dimCache memoizes the expensive RGB blend used by DimRect. Modal backdrops
	// usually contain a very small style palette repeated over thousands of
	// cells, so doing the blend once per distinct style instead of once per cell
	// removes the dominant arithmetic cost while keeping exact DimRect semantics.
	dimCache      [128]dimCacheEntry
	dimLastStyle  style.Style
	dimLastOut    style.Style
	dimLastAmount uint8
	dimLastValid  bool
}

type sgrCacheEntry struct {
	style style.Style
	data  [80]byte
	n     uint8
	valid bool
}

type dimCacheEntry struct {
	style  style.Style
	amount uint8
	out    style.Style
	valid  bool
}

func New(w, h int) *Buffer {
	b := &Buffer{}
	b.Resize(w, h)
	return b
}

// Resize reallocates the cell grids. This is the one place allocation is expected - it happens only on startup and on terminal resize (SIGWINCH), never per frame.
func (b *Buffer) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	cells := w * h
	words := (cells + 63) >> 6
	b.W, b.H = w, h
	// Reuse resize backing storage whenever capacity permits. Resize is an
	// infrequent boundary, but avoiding churn here makes repeated SIGWINCH
	// events cheap and prevents the allocator from becoming part of resize
	// stress tests. Growth still allocates when the existing backing store is
	// genuinely too small.
	if cap(b.front) < cells {
		b.front = make([]Cell, cells)
	} else {
		b.front = b.front[:cells]
	}
	if cap(b.back) < cells {
		b.back = make([]Cell, cells)
	} else {
		b.back = b.back[:cells]
	}
	if cap(b.dirty) < words {
		b.dirty = make([]uint64, words)
	} else {
		b.dirty = b.dirty[:words]
	}
	rowWords := (h + 63) >> 6
	if cap(b.dirtyRows) < rowWords {
		b.dirtyRows = make([]uint64, rowWords)
	} else {
		b.dirtyRows = b.dirtyRows[:rowWords]
	}
	if cap(b.dirtyMin) < h {
		b.dirtyMin = make([]int, h)
	} else {
		b.dirtyMin = b.dirtyMin[:h]
	}
	if cap(b.dirtyMax) < h {
		b.dirtyMax = make([]int, h)
	} else {
		b.dirtyMax = b.dirtyMax[:h]
	}
	for y := 0; y < h; y++ {
		b.dirtyMin[y], b.dirtyMax[y] = 0, w-1
	}
	for i := range b.front {
		b.front[i] = Cell{Ch: 0} // force full repaint of first frame
		b.back[i] = blank
	}
	// Resize is a full repaint boundary. Mark every cell dirty even though the
	// back buffer initially contains blanks, because front may contain terminal
	// state from a prior geometry.
	for i := range b.dirty {
		b.dirty[i] = ^uint64(0)
	}
	for i := range b.dirtyRows {
		b.dirtyRows[i] = ^uint64(0)
	}
	for y := 0; y < h; y++ {
		b.dirtyMin[y], b.dirtyMax[y] = 0, w-1
	}
	if extra := len(b.dirty)*64 - w*h; extra > 0 {
		b.dirty[len(b.dirty)-1] &= ^uint64(0) >> extra
	}
	b.sgrValid = false
	b.outCursorValid = false
	b.forceFull = false
	b.clipped = false
}

func (b *Buffer) idx(x, y int) int { return y*b.W + x }

// CellAt returns the cell currently queued in the back buffer at (x,y) : i.e. what the next Render call will (attempt to) draw. Exported mainly for widget tests to assert on exact styling without parsing the ANSI byte stream; out-of-bounds coordinates return the zero Cell.
func (b *Buffer) CellAt(x, y int) Cell {
	if !b.InBounds(x, y) {
		return Cell{}
	}
	return b.back[b.idx(x, y)]
}

func (b *Buffer) InBounds(x, y int) bool {
	if x < 0 || x >= b.W || y < 0 || y >= b.H {
		return false
	}
	if b.clipped {
		return x >= b.clip.X && x < b.clip.X+b.clip.W && y >= b.clip.Y && y < b.clip.Y+b.clip.H
	}
	return true
}

// SetClip limits subsequent drawing operations to r. Clip is intentionally a
// single reusable rectangle rather than a stack to keep the hot path tiny.
func (b *Buffer) SetClip(r Rect) {
	if r.X < 0 {
		r.W += r.X
		r.X = 0
	}
	if r.Y < 0 {
		r.H += r.Y
		r.Y = 0
	}
	if r.X+r.W > b.W {
		r.W = b.W - r.X
	}
	if r.Y+r.H > b.H {
		r.H = b.H - r.Y
	}
	if r.W < 0 {
		r.W = 0
	}
	if r.H < 0 {
		r.H = 0
	}
	b.clip, b.clipped = r, true
}

func (b *Buffer) ClearClip() { b.clipped = false }

// Clip returns the current drawing clip.
func (b *Buffer) Clip() Rect {
	if b.clipped {
		return b.clip
	}
	return Rect{X: 0, Y: 0, W: b.W, H: b.H}
}

// Rect is the buffer-local rectangle type used by clipping and damage
// rendering. It deliberately mirrors geometry.Rect without importing layout
// packages into the renderer.
type Rect struct{ X, Y, W, H int }

func (b *Buffer) markDirtyIdx(i int) {
	b.dirty[i>>6] |= uint64(1) << uint(i&63)
	y := i / b.W
	b.dirtyRows[y>>6] |= uint64(1) << uint(y&63)
	x := i - y*b.W
	if x < b.dirtyMin[y] {
		b.dirtyMin[y] = x
	}
	if x > b.dirtyMax[y] {
		b.dirtyMax[y] = x
	}
}

func (b *Buffer) clearDirtyIdx(i int) {
	b.dirty[i>>6] &^= uint64(1) << uint(i&63)
}

func (b *Buffer) clearDirtyRowIfClean(y int) {
	base, end := y*b.W, (y+1)*b.W
	ws, we := base>>6, (end-1)>>6
	for wi := ws; wi <= we; wi++ {
		lo := uint(0)
		hi := uint(63)
		if wi == ws {
			lo = uint(base & 63)
		}
		if wi == we {
			hi = uint((end - 1) & 63)
		}
		mask := (^uint64(0) << lo) & (^uint64(0) >> (63 - hi))
		if b.dirty[wi]&mask != 0 {
			return
		}
	}
	b.dirtyRows[y>>6] &^= uint64(1) << uint(y&63)
	b.dirtyMin[y], b.dirtyMax[y] = b.W, -1
}

func (b *Buffer) clearDirtyRange(start, end int) {
	if start >= end {
		return
	}
	ws, we := start>>6, (end-1)>>6
	if ws == we {
		lo := uint(start & 63)
		hi := uint((end - 1) & 63)
		mask := (^uint64(0) << lo) & (^uint64(0) >> (63 - hi))
		b.dirty[ws] &^= mask
		return
	}
	b.dirty[ws] &^= ^uint64(0) << uint(start&63)
	for wi := ws + 1; wi < we; wi++ {
		b.dirty[wi] = 0
	}
	b.dirty[we] &^= ^uint64(0) >> uint(63-((end-1)&63))
}

func (b *Buffer) markDirtyRange(start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(b.back) {
		end = len(b.back)
	}
	if start >= end {
		return
	}
	// Keep the row index in sync for callers that mark linear ranges directly.
	firstRow, lastRow := start/b.W, (end-1)/b.W
	for y := firstRow; y <= lastRow; y++ {
		b.dirtyRows[y>>6] |= uint64(1) << uint(y&63)
	}
	ws, we := start>>6, (end-1)>>6
	if ws == we {
		lo := uint(start & 63)
		hi := uint((end - 1) & 63)
		mask := (^uint64(0) << lo) & (^uint64(0) >> (63 - hi))
		b.dirty[ws] |= mask
		return
	}
	b.dirty[ws] |= ^uint64(0) << uint(start&63)
	for w := ws + 1; w < we; w++ {
		b.dirty[w] = ^uint64(0)
	}
	b.dirty[we] |= ^uint64(0) >> uint(63-((end-1)&63))
}

func (b *Buffer) markDirtyRect(r Rect) {
	r = b.normalizeRect(r)
	if r.W <= 0 || r.H <= 0 {
		return
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		b.markDirtyRange(y*b.W+r.X, y*b.W+r.X+r.W)
		b.dirtyRows[y>>6] |= uint64(1) << uint(y&63)
	}
}

// DirtyCells returns the number of cells currently pending synchronization with
// the terminal. It is O(number of bitset words), not O(screen cells).
func (b *Buffer) DirtyCells() int {
	n := 0
	for _, w := range b.dirty {
		n += bits.OnesCount64(w)
	}
	return n
}

// Clear fills the back buffer with a blank cell in the given style : typically called once at the start of each frame before widgets draw.
func (b *Buffer) Clear(st style.Style) {
	c := Cell{Ch: ' ', Style: st}
	fillCells(b.back, c)
	for i := range b.dirty {
		b.dirty[i] = ^uint64(0)
	}
	for i := range b.dirtyRows {
		b.dirtyRows[i] = ^uint64(0)
	}
	for y := 0; y < b.H; y++ {
		b.dirtyMin[y], b.dirtyMax[y] = 0, b.W-1
	}
	if extra := len(b.dirty)*64 - len(b.back); extra > 0 && len(b.dirty) > 0 {
		b.dirty[len(b.dirty)-1] &= ^uint64(0) >> extra
	}
}

// fillCells uses geometric bulk copies instead of a per-cell assignment loop.
// The slice is still fully written, but the hot path is delegated to the
// runtime's optimized memmove implementation.
func fillCells(dst []Cell, c Cell) {
	if len(dst) == 0 {
		return
	}
	dst[0] = c
	filled := 1
	for filled < len(dst) {
		n := filled
		if n > len(dst)-filled {
			n = len(dst) - filled
		}
		copy(dst[filled:filled+n], dst[:n])
		filled += n
	}
}

// ClearRegion clears only r in the retained back buffer. This is the key
// primitive for damage-based redraws: a dirty widget can be repainted without
// touching unrelated cells.
func (b *Buffer) ClearRegion(r Rect, st style.Style) {
	r = b.normalizeRect(r)
	if r.W <= 0 || r.H <= 0 {
		return
	}
	c := Cell{Ch: ' ', Style: st}
	for y := r.Y; y < r.Y+r.H; y++ {
		row := b.back[y*b.W+r.X : y*b.W+r.X+r.W]
		fillCells(row, c)
		b.markDirtyRange(y*b.W+r.X, y*b.W+r.X+r.W)
	}
}

func (b *Buffer) normalizeRect(r Rect) Rect {
	if r.X < 0 {
		r.W += r.X
		r.X = 0
	}
	if r.Y < 0 {
		r.H += r.Y
		r.Y = 0
	}
	if r.X+r.W > b.W {
		r.W = b.W - r.X
	}
	if r.Y+r.H > b.H {
		r.H = b.H - r.Y
	}
	if r.W < 0 {
		r.W = 0
	}
	if r.H < 0 {
		r.H = 0
	}
	return r
}

// Set writes a single rune into the back buffer.
func (b *Buffer) Set(x, y int, ch rune, st style.Style) {
	if !b.InBounds(x, y) {
		return
	}
	i := b.idx(x, y)
	c := Cell{Ch: ch, Style: st}
	if b.back[i] != c {
		b.back[i] = c
		b.markDirtyIdx(i)
	}
}

// SetString writes a string starting at (x,y), clipped to buffer/width bounds. Ranging over a string yields runes without allocating.
func (b *Buffer) SetString(x, y int, s string, st style.Style) {
	clip := b.Clip()
	if y < clip.Y || y >= clip.Y+clip.H || x >= clip.X+clip.W || x+len(s) <= clip.X {
		return
	}
	col := x
	for _, r := range s {
		if col >= clip.X+clip.W {
			break
		}
		if col >= clip.X {
			i := b.idx(col, y)
			c := Cell{Ch: r, Style: st}
			if b.back[i] != c {
				b.back[i] = c
				b.markDirtyIdx(i)
			}
		}
		col++
	}
}

// SetBytes is the zero-alloc counterpart of SetString for numfmt output.
func (b *Buffer) SetBytes(x, y int, s []byte, st style.Style) {
	clip := b.Clip()
	if y < clip.Y || y >= clip.Y+clip.H || x >= clip.X+clip.W || x+len(s) <= clip.X {
		return
	}
	col := x
	for _, r := range s {
		if col >= clip.X+clip.W {
			break
		}
		if col >= clip.X {
			i := b.idx(col, y)
			c := Cell{Ch: rune(r), Style: st}
			if b.back[i] != c {
				b.back[i] = c
				b.markDirtyIdx(i)
			}
		}
		col++
	}
}

// SetPaddedString writes a clipped, padded string directly into the cell
// buffer. It is intended for table-like widgets where values are already
// ASCII/UTF-8 strings and constructing temporary padded strings would create
// garbage on every frame.
func (b *Buffer) SetPaddedString(x, y int, s string, width int, right bool, st style.Style) {
	if y < 0 || y >= b.H || width <= 0 || x >= b.W {
		return
	}
	if x < 0 {
		x = 0
	}
	limit := width
	if limit > b.W-x {
		limit = b.W - x
	}
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		truncated := len(s) > width
		n := len(s)
		if truncated {
			if width == 1 {
				n = 1
			} else {
				n = width
			}
		}
		pad := width - n
		if right {
			for i := 0; i < pad && i < limit; i++ {
				ci := b.idx(x+i, y)
				c := Cell{Ch: ' ', Style: st}
				if b.back[ci] != c {
					b.back[ci] = c
					b.markDirtyIdx(ci)
				}
			}
		}
		writeN := n
		if truncated && width > 1 {
			writeN = width - 1
		}
		for i := 0; i < writeN && i < limit; i++ {
			pos := i
			if right {
				pos += pad
			}
			if pos >= limit {
				break
			}
			ci := b.idx(x+pos, y)
			c := Cell{Ch: rune(s[i]), Style: st}
			if b.back[ci] != c {
				b.back[ci] = c
				b.markDirtyIdx(ci)
			}
		}
		if truncated {
			pos := width - 1
			if right {
				pos += pad
			}
			if pos < limit {
				ci := b.idx(x+pos, y)
				c := Cell{Ch: '…', Style: st}
				if b.back[ci] != c {
					b.back[ci] = c
					b.markDirtyIdx(ci)
				}
			}
		}
		startPad := n
		if truncated {
			startPad = width
		}
		if !right {
			for i := startPad; i < limit; i++ {
				ci := b.idx(x+i, y)
				c := Cell{Ch: ' ', Style: st}
				if b.back[ci] != c {
					b.back[ci] = c
					b.markDirtyIdx(ci)
				}
			}
		}
		return
	}

	// Unicode path: determine rune count, then truncate while writing directly.
	n := 0
	for range s {
		n++
	}
	truncated := n > width
	if truncated {
		if width == 1 {
			n = 0
		} else {
			n = width - 1
		}
	}
	pad := width - n
	col := x
	if right {
		col += pad
	}
	written := 0
	for _, r := range s {
		if written >= n {
			break
		}
		ci := b.idx(col, y)
		c := Cell{Ch: r, Style: st}
		if b.back[ci] != c {
			b.back[ci] = c
			b.markDirtyIdx(ci)
		}
		col++
		written++
	}
	if truncated {
		pos := x + width - 1
		ci := b.idx(pos, y)
		c := Cell{Ch: '…', Style: st}
		if b.back[ci] != c {
			b.back[ci] = c
			b.markDirtyIdx(ci)
		}
	}
	if right {
		for i := 0; i < pad && i < limit; i++ {
			ci := b.idx(x+i, y)
			c := Cell{Ch: ' ', Style: st}
			if b.back[ci] != c {
				b.back[ci] = c
				b.markDirtyIdx(ci)
			}
		}
	} else {
		for i := n; i < limit; i++ {
			ci := b.idx(x+i, y)
			c := Cell{Ch: ' ', Style: st}
			if b.back[ci] != c {
				b.back[ci] = c
				b.markDirtyIdx(ci)
			}
		}
	}
}

// Fill paints a rectangular region with a repeated rune (borders, tracks, solid panels).
func (b *Buffer) FillRect(x, y, w, h int, ch rune, st style.Style) {
	r := b.clipRect(Rect{X: x, Y: y, W: w, H: h})
	if r.W <= 0 || r.H <= 0 {
		return
	}
	c := Cell{Ch: ch, Style: st}
	for row := r.Y; row < r.Y+r.H; row++ {
		cells := b.back[row*b.W+r.X : row*b.W+r.X+r.W]
		fillCells(cells, c)
		b.markDirtyRange(row*b.W+r.X, row*b.W+r.X+r.W)
	}
}

// DimRect dims the existing composited cells without erasing their glyphs.
// It is used by modal overlays to create a translucent terminal backdrop.
// The operation is allocation-free and intentionally mutates only the back buffer.
func (b *Buffer) DimRect(x, y, w, h int, amount uint8) {
	r := b.clipRect(Rect{X: x, Y: y, W: w, H: h})
	if r.W <= 0 || r.H <= 0 || amount == 0 {
		return
	}
	// The blend is computed once per distinct (style, amount) pair and then
	// reused for every cell carrying that style. Keep a local pair as well as
	// the buffer-level cache: terminal rows are usually style runs, so the
	// common case avoids even a helper call and cache-index calculation.
	var runStyle style.Style
	var runOut style.Style
	runValid := false
	for row := r.Y; row < r.Y+r.H; row++ {
		base := row * b.W
		rowStart := base + r.X
		rowEnd := rowStart + r.W
		rowChanged := false
		for i := rowStart; i < rowEnd; i++ {
			old := b.back[i]
			c := old
			if c.Ch == 0 {
				c.Ch = ' '
			}
			if !runValid || old.Style != runStyle {
				runStyle = old.Style
				runOut = b.dimmedStyle(runStyle, amount)
				runValid = true
			}
			c.Style = runOut
			if c != old {
				b.back[i] = c
				rowChanged = true
			}
		}
		// DimRect can change most or all cells in the row. Marking the row as a
		// range avoids one atomic-looking bit operation per cell.
		if rowChanged {
			b.markDirtyRange(rowStart, rowEnd)
		}
	}
}

// DimRectAttr is the allocation-free, terminal-native dim path used by
// modal backdrops. Unlike DimRect, it does not blend RGB channels; it applies
// the ANSI SGR Dim attribute while preserving the existing foreground and
// background colors. This is substantially cheaper for full-screen overlays
// because it avoids six channel divisions per cell.
func (b *Buffer) DimRectAttr(x, y, w, h int) {
	r := b.clipRect(Rect{X: x, Y: y, W: w, H: h})
	if r.W <= 0 || r.H <= 0 {
		return
	}
	for row := r.Y; row < r.Y+r.H; row++ {
		start := row*b.W + r.X
		end := start + r.W
		changed := false
		for i := start; i < end; i++ {
			old := b.back[i]
			c := old
			if c.Ch == 0 {
				c.Ch = ' '
			}
			c.Style.Attr |= style.Dim
			if c != old {
				b.back[i] = c
				changed = true
			}
		}
		if changed {
			b.markDirtyRange(start, end)
		}
	}
}

func (b *Buffer) dimmedStyle(st style.Style, amount uint8) style.Style {
	if b.dimLastValid && b.dimLastAmount == amount && b.dimLastStyle == st {
		return b.dimLastOut
	}
	// Keep the cache index deliberately cheap: the full style is still compared
	// on hit, so collisions are harmless. The previous hash mixed every byte of
	// both colors; that was measurable because dimmedStyle is called once per
	// cell. Two low-cost byte selections are enough for a fixed direct-mapped
	// cache and avoid four shifts/xors in the common path.
	h := uint8(st.Fg) ^ uint8(st.Fg>>8) ^ uint8(st.Bg>>16) ^ uint8(st.Attr) ^ amount
	idx := h & (uint8(len(b.dimCache)) - 1)
	e := &b.dimCache[idx]
	if e.valid && e.amount == amount && e.style == st {
		return e.out
	}

	out := st
	if st.Fg == st.Bg {
		// Blending identical colors is a pure attribute operation.
		out.Attr |= style.Dim
		b.dimLastStyle = st
		b.dimLastAmount = amount
		b.dimLastOut = out
		b.dimLastValid = true
		e.style = st
		e.amount = amount
		e.out = out
		e.valid = true
		return out
	}
	if amount == 255 {
		// Exact endpoint: Lerp(fg,bg,255) == bg, with no arithmetic required.
		out.Fg = st.Bg
	} else {
		inv := uint32(255 - amount)
		amt := uint32(amount)
		fg := uint32(st.Fg)
		bg := uint32(st.Bg)
		fr, fg8, fb := (fg>>16)&0xff, (fg>>8)&0xff, fg&0xff
		br, bg8, bb := (bg>>16)&0xff, (bg>>8)&0xff, bg&0xff
		rr := (fr*inv + br*amt) / 255
		gg := (fg8*inv + bg8*amt) / 255
		bl := (fb*inv + bb*amt) / 255
		out.Fg = color.Color((rr << 16) | (gg << 8) | bl)
	}
	out.Attr |= style.Dim
	e.style = st
	e.amount = amount
	e.out = out
	e.valid = true
	b.dimLastStyle = st
	b.dimLastAmount = amount
	b.dimLastOut = out
	b.dimLastValid = true
	return out
}

func (b *Buffer) clipRect(r Rect) Rect {
	r = b.normalizeRect(r)
	if b.clipped {
		if r.X < b.clip.X {
			r.W -= b.clip.X - r.X
			r.X = b.clip.X
		}
		if r.Y < b.clip.Y {
			r.H -= b.clip.Y - r.Y
			r.Y = b.clip.Y
		}
		if r.X+r.W > b.clip.X+b.clip.W {
			r.W = b.clip.X + b.clip.W - r.X
		}
		if r.Y+r.H > b.clip.Y+b.clip.H {
			r.H = b.clip.Y + b.clip.H - r.Y
		}
	}
	if r.W < 0 {
		r.W = 0
	}
	if r.H < 0 {
		r.H = 0
	}
	return r
}

// DrawBorder paints a titled box border and fills its interior, the shared primitive behind widget.Panel and layout.Bordered so both the "wrap one widget" and "wrap a whole sub-layout" composition styles render identically.
func DrawBorder(b *Buffer, x, y, w, h int, title string, borderSt, titleSt, fillSt style.Style, rounded bool) {
	if w < 2 || h < 2 {
		return
	}
	tl, tr, bl, br, hc, vc := '┌', '┐', '└', '┘', '─', '│'
	if rounded {
		tl, tr, bl, br = '╭', '╮', '╰', '╯'
	}
	b.Set(x, y, tl, borderSt)
	b.Set(x+w-1, y, tr, borderSt)
	b.Set(x, y+h-1, bl, borderSt)
	b.Set(x+w-1, y+h-1, br, borderSt)
	b.FillRect(x+1, y, w-2, 1, hc, borderSt)
	b.FillRect(x+1, y+h-1, w-2, 1, hc, borderSt)
	b.FillRect(x, y+1, 1, h-2, vc, borderSt)
	b.FillRect(x+w-1, y+1, 1, h-2, vc, borderSt)
	b.FillRect(x+1, y+1, w-2, h-2, ' ', fillSt)
	if title != "" {
		b.SetString(x+2, y, " "+title+" ", titleSt)
	}
}

// Render diffs back against front, writes the minimal ANSI byte stream to w, and swaps buffers. Returns bytes written (0 on an unchanged frame).
func (b *Buffer) Render(w io.Writer) (int, error) {
	return b.RenderRegions(w, []Rect{{X: 0, Y: 0, W: b.W, H: b.H}})
}

// RenderSynchronized emits a frame inside DEC synchronized-output mode (mode
// 2026). Terminals that support it present the frame atomically, reducing
// visible tearing during high-frequency market-data updates. Unsupported
// terminals safely ignore the private mode sequence.
func (b *Buffer) RenderSynchronized(w io.Writer) (int, error) {
	return b.RenderRegionsSynchronized(w, []Rect{{X: 0, Y: 0, W: b.W, H: b.H}})
}

// RenderRegionsSynchronized is the synchronized-output counterpart of
// RenderRegions. The begin/end sequences are emitted only when the frame has
// actual changed cells, so idle renders remain silent.
func (b *Buffer) RenderRegionsSynchronized(w io.Writer, regions []Rect) (int, error) {
	return b.renderRegions(w, regions, true)
}

// RenderRegions diffs only the supplied damage rectangles. Cells outside the
// regions remain resident in the front/back buffers, enabling retained-mode
// rendering without changing the Widget.Draw contract. Overlapping regions
// are harmless because the first region updates front and the second sees no
// remaining differences.
func (b *Buffer) RenderRegions(w io.Writer, regions []Rect) (int, error) {
	return b.renderRegions(w, regions, false)
}

func (b *Buffer) renderRegions(w io.Writer, regions []Rect, synchronized bool) (int, error) {
	b.out = b.out[:0]
	b.outCursorValid = false

	// A changed cell needs at most four UTF-8 bytes. Cursor/style transitions
	// add overhead, but a modest area-based reserve eliminates repeated growth
	// for the common dense-frame case. The capacity itself is retained by the
	// Buffer, so this is a one-time cost per larger workload.
	area := 0
	for _, rr := range regions {
		r := b.normalizeRect(rr)
		if r.W > 0 && r.H > 0 {
			area += r.W * r.H
		}
	}
	if area > 0 {
		hint := len(b.out) + area*6
		if hint > b.outCapacityHint {
			b.outCapacityHint = hint
		}
		if cap(b.out) < b.outCapacityHint {
			b.out = make([]byte, 0, b.outCapacityHint)
		}
	}

	for _, rr := range regions {
		r := b.normalizeRect(rr)
		if r.W <= 0 || r.H <= 0 {
			continue
		}
		for y := r.Y; y < r.Y+r.H; y++ {
			rowBit := uint64(1) << uint(y&63)
			if b.dirtyRows[y>>6]&rowBit == 0 {
				continue
			}

			rowBase := y * b.W
			x0, x1 := r.X, r.X+r.W-1
			if b.dirtyMin[y] > x0 {
				x0 = b.dirtyMin[y]
			}
			if b.dirtyMax[y] < x1 {
				x1 = b.dirtyMax[y]
			}
			if x0 > x1 {
				continue
			}
			start := rowBase + x0
			end := rowBase + x1 + 1
			ws := start >> 6
			we := (end - 1) >> 6

			for wi := ws; wi <= we; wi++ {
				word := b.dirty[wi]
				if wi == ws {
					word &= ^uint64(0) << uint(start&63)
				}
				if wi == we {
					word &= ^uint64(0) >> uint(63-((end-1)&63))
				}

				for word != 0 {
					bit := bits.TrailingZeros64(word)
					idx := wi*64 + bit
					if idx >= end {
						break
					}

					// Dirty is conservative: a mutation may have restored the old
					// value before the render pass. Drop such cells without emitting.
					if !b.forceFull && b.back[idx] == b.front[idx] {
						b.clearDirtyIdx(idx)
						word &^= uint64(1) << uint(bit)
						continue
					}

					// Build one maximal dirty/different run. The common case is a
					// contiguous SetString/FillRect range, so this remains a tight
					// linear walk over changed cells and avoids scanning clean cells.
					runEnd := idx + 1
					for runEnd < end {
						m := uint64(1) << uint(runEnd&63)
						if b.dirty[runEnd>>6]&m == 0 || b.back[runEnd] == b.front[runEnd] {
							break
						}
						runEnd++
					}

					b.writeCursor(idx-rowBase, y)
					asciiRun := true
					for i := idx; i < runEnd; i++ {
						c := b.back[i]
						b.writeStyle(c.Style)
						ch := c.Ch
						if ch == 0 {
							ch = ' '
						}
						if uint32(ch) >= utf8.RuneSelf {
							asciiRun = false
						}
						b.out = appendRuneFast(b.out, ch)
					}
					copy(b.front[idx:runEnd], b.back[idx:runEnd])
					b.clearDirtyRange(idx, runEnd)
					b.outCursorX = runEnd - rowBase
					b.outCursorY = y
					b.outCursorValid = true
					b.outCursorASCII = asciiRun

					// Refresh the current word from the dirty bitset. This is
					// important when the run crossed a machine-word boundary.
					word = b.dirty[wi]
					if wi == ws {
						word &= ^uint64(0) << uint(start&63)
					}
					if wi == we {
						word &= ^uint64(0) >> uint(63-((end-1)&63))
					}
				}
			}
			b.clearDirtyRowIfClean(y)
		}
	}
	if len(b.out) == 0 {
		b.forceFull = false
		return 0, nil
	}
	total := 0
	if synchronized {
		if n, err := w.Write(syncBegin[:]); err != nil || n != 8 {
			b.forceFull = true
			b.markAllDirty()
			if err == nil {
				return n, io.ErrShortWrite
			}
			return n, err
		} else {
			total += n
		}
	}
	n, err := w.Write(b.out)
	total += n
	if err == nil && n == len(b.out) && synchronized {
		endN, endErr := w.Write(syncEnd[:])
		total += endN
		if endErr != nil || endN != 8 {
			b.forceFull = true
			b.markAllDirty()
			if endErr == nil {
				return total, io.ErrShortWrite
			}
			return total, endErr
		}
	}
	if err != nil || n != len(b.out) {
		// A partial terminal write leaves the terminal state unknown. When
		// synchronized output was enabled, close the mode on a best-effort basis
		// before forcing the conservative full repaint.
		if synchronized {
			_, _ = w.Write(syncEnd[:])
		}
		// front was synchronized while constructing the output, so retrying only
		// the remaining dirty bits would be incorrect.
		b.forceFull = true
		b.markAllDirty()
		b.sgrValid = false
		b.outCursorValid = false
		if err != nil {
			return total, err
		}
		return total, io.ErrShortWrite
	}
	b.forceFull = false
	return total, nil
}

func (b *Buffer) markAllDirty() {
	for i := range b.dirty {
		b.dirty[i] = ^uint64(0)
	}
	for i := range b.dirtyRows {
		b.dirtyRows[i] = ^uint64(0)
	}
	for y := 0; y < b.H; y++ {
		b.dirtyMin[y], b.dirtyMax[y] = 0, b.W-1
	}
	if extra := len(b.dirty)*64 - len(b.back); extra > 0 && len(b.dirty) > 0 {
		b.dirty[len(b.dirty)-1] &= ^uint64(0) >> extra
	}
}

func (b *Buffer) writeCursor(x, y int) {
	if b.outCursorValid && b.outCursorASCII && b.outCursorY == y {
		if b.outCursorX == x {
			return
		}
		// Horizontal relative addressing is shorter for small gaps and avoids
		// decimal row/column formatting. Keep the threshold conservative because
		// absolute H becomes cheaper once the distance grows.
		if x > b.outCursorX {
			d := x - b.outCursorX
			if d <= 6 {
				b.out = append(b.out, '\x1b', '[')
				b.out = appendUint(b.out, uint(d))
				b.out = append(b.out, 'C')
				return
			}
		}
	}
	b.out = append(b.out, '\x1b', '[')
	b.out = appendUint(b.out, uint(y+1))
	b.out = append(b.out, ';')
	b.out = appendUint(b.out, uint(x+1))
	b.out = append(b.out, 'H')
}

func (b *Buffer) writeStyle(st style.Style) {
	if b.sgrValid && st == (style.Style{Fg: b.lastFg, Bg: b.lastBg, Attr: b.lastAttr}) {
		return
	}

	if b.sgrValid {
		prev := style.Style{Fg: b.lastFg, Bg: b.lastBg, Attr: b.lastAttr}
		// Color-only transitions can be emitted without resetting attributes.
		if st.Attr == prev.Attr {
			if st.Fg != prev.Fg {
				b.appendFG(st.Fg)
			}
			if st.Bg != prev.Bg {
				b.appendBG(st.Bg)
			}
			b.lastFg, b.lastBg, b.lastAttr = st.Fg, st.Bg, st.Attr
			return
		}
	}

	// Fixed direct-mapped cache stores the complete style sequence. This path
	// is used when attributes change or when no previous terminal style exists.
	h := uint8(st.Fg) ^ uint8(st.Fg>>8) ^ uint8(st.Fg>>16)
	h ^= uint8(st.Bg) ^ uint8(st.Bg>>8) ^ uint8(st.Bg>>16)
	h ^= uint8(st.Attr * 31)
	idx := h & (uint8(len(b.sgrCache)) - 1)
	entry := &b.sgrCache[idx]
	if entry.valid && entry.style == st {
		b.out = append(b.out, entry.data[:entry.n]...)
	} else {
		entry.style = st
		buf := entry.data[:0]
		buf = append(buf, '\x1b', '[', '0')
		if st.Attr.Has(style.Bold) {
			buf = append(buf, ';', '1')
		}
		if st.Attr.Has(style.Dim) {
			buf = append(buf, ';', '2')
		}
		if st.Attr.Has(style.Underline) {
			buf = append(buf, ';', '4')
		}
		if st.Attr.Has(style.Reverse) {
			buf = append(buf, ';', '7')
		}
		if st.Attr.Has(style.Blink) {
			buf = append(buf, ';', '5')
		}
		if st.Fg != color.Default {
			r, g, bl := st.Fg.Components()
			buf = append(buf, ';', '3', '8', ';', '2', ';')
			buf = appendUint(buf, uint(r))
			buf = append(buf, ';')
			buf = appendUint(buf, uint(g))
			buf = append(buf, ';')
			buf = appendUint(buf, uint(bl))
		}
		if st.Bg != color.Default {
			r, g, bl := st.Bg.Components()
			buf = append(buf, ';', '4', '8', ';', '2', ';')
			buf = appendUint(buf, uint(r))
			buf = append(buf, ';')
			buf = appendUint(buf, uint(g))
			buf = append(buf, ';')
			buf = appendUint(buf, uint(bl))
		}
		buf = append(buf, 'm')
		entry.n = uint8(len(buf))
		entry.valid = true
		b.out = append(b.out, buf...)
	}
	b.lastFg, b.lastBg, b.lastAttr = st.Fg, st.Bg, st.Attr
	b.sgrValid = true
}

func (b *Buffer) appendFG(c color.Color) {
	if c == color.Default {
		b.out = append(b.out, '\x1b', '[', '3', '9', 'm')
		return
	}
	r, g, bl := c.Components()
	b.out = append(b.out, '\x1b', '[', '3', '8', ';', '2', ';')
	b.out = appendUint(b.out, uint(r))
	b.out = append(b.out, ';')
	b.out = appendUint(b.out, uint(g))
	b.out = append(b.out, ';')
	b.out = appendUint(b.out, uint(bl))
	b.out = append(b.out, 'm')
}

func (b *Buffer) appendBG(c color.Color) {
	if c == color.Default {
		b.out = append(b.out, '\x1b', '[', '4', '9', 'm')
		return
	}
	r, g, bl := c.Components()
	b.out = append(b.out, '\x1b', '[', '4', '8', ';', '2', ';')
	b.out = appendUint(b.out, uint(r))
	b.out = append(b.out, ';')
	b.out = appendUint(b.out, uint(g))
	b.out = append(b.out, ';')
	b.out = appendUint(b.out, uint(bl))
	b.out = append(b.out, 'm')
}

func appendRuneFast(dst []byte, r rune) []byte {
	if uint32(r) < utf8.RuneSelf {
		return append(dst, byte(r))
	}
	u := uint32(r)
	if u <= 0x7FF {
		return append(dst, byte(0xC0|(u>>6)), byte(0x80|(u&0x3F)))
	}
	if u <= 0xFFFF {
		return append(dst, byte(0xE0|(u>>12)), byte(0x80|((u>>6)&0x3F)), byte(0x80|(u&0x3F)))
	}
	return append(dst, byte(0xF0|(u>>18)), byte(0x80|((u>>12)&0x3F)), byte(0x80|((u>>6)&0x3F)), byte(0x80|(u&0x3F)))
}

func appendUint(dst []byte, v uint) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var tmp [10]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, tmp[i:]...)
}
