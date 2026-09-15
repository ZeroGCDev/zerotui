package app

import (
	"io"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/term"
	"github.com/ZeroGCDev/zerotui/widget"
)

// DirtyFlags describe which portion of a retained scene node needs work.
// The app currently uses these flags to retain widget identity across reflows
// and to distinguish paint invalidation from geometry invalidation. They are
// intentionally a byte-sized bitset so hot-path state stays compact.
type DirtyFlags uint8

const (
	DirtyNone  DirtyFlags = 0
	DirtyPaint DirtyFlags = 1 << iota
	DirtyLayout
	DirtyChildren
	DirtyVisibility
)

type placementState struct {
	id    uint64
	flags DirtyFlags
	area  geometry.Rect
}

// widgetPlacements keeps the common one-placement case inline. Most widgets
// occur exactly once in a layout, so storing []int for every widget forced one
// heap allocation per placement during every reflow. The overflow slice is
// only allocated when the same widget is placed more than once.
type widgetPlacements struct {
	first int
	extra []int
}

// App owns the render loop, terminal lifecycle, focus chain, and input routing for a zerotui program.
type App struct {
	Root  layout.Node
	Theme *style.Theme

	// TargetFPS caps live/interactive rendering. Static apps remain fully demand-driven.
	// Zero uses 60 FPS while live or dragging, and no periodic frame clock while idle.
	TargetFPS int

	// SynchronizedOutput wraps changed terminal frames in DEC mode 2026 when
	// enabled. Supporting terminals present each frame atomically; unsupported
	// terminals ignore the private mode sequence. It is enabled by default.
	SynchronizedOutput bool

	// QuitKeys are runes that exit Run immediately (default 'q'). Ctrl+C always quits regardless of this list.
	QuitKeys []rune

	// OnKey, if set, is called before focus routing for every key event; returning true consumes the event (e.g. global hotkeys like 'r' to reset, or panic-button kill switches independent of which widget has focus).
	OnKey func(k input.Key) bool

	// OnResize, if set, is called after every relayout with the new
	// terminal size.
	OnResize func(w, h int)

	buf              *buffer.Buffer
	placements       []layout.Placement
	placementsMu     sync.RWMutex
	focus            focusRing
	focusScratch     []widget.Focusable
	mouseCapture     widget.MouseHandler
	mouseCaptureArea geometry.Rect
	width, height    int
	dirty            atomic.Bool
	layoutDirty      atomic.Bool
	interactive      atomic.Bool
	live             atomic.Int32
	wake             chan struct{}
	uiQueue          chan func()
	spatial          spatialIndex
	widgetIndex      map[widget.Widget]widgetPlacements
	widgetIDs        map[widget.Widget]uint64
	oldStateByID     map[uint64]placementState
	placementState   []placementState
	nextNodeID       uint64
	drawLock         sync.Mutex
	metrics          metricsState
	metricsEnabled   atomic.Bool
	lastLayoutArea   geometry.Rect
	lastLayoutRoot   layout.Node
	layoutEpoch      uint64
	computedEpoch    uint64
	batchDepth       atomic.Int32
	hasLayoutCache   bool
	hasPainted       bool

	damageMu            sync.Mutex
	damage              [32]geometry.Rect
	damageCount         int
	damageFull          bool
	damageScratch       [32]buffer.Rect
	damageRegionScratch [8]geometry.Rect

	// liveRegions lets animations repaint only their own rectangles. RequestLive
	// remains available for truly global animations; region-scoped live rendering
	// avoids turning a tiny spinner/sparkline animation into a full-screen frame.
	liveRegionMu    sync.Mutex
	liveRegions     [8]geometry.Rect
	liveRegionRefs  [8]uint32
	liveRegionCount int
}

func New(root layout.Node, theme *style.Theme) *App {
	if theme == nil {
		theme = style.NordTheme()
	}
	return &App{Root: root, Theme: theme, TargetFPS: 30, SynchronizedOutput: true, QuitKeys: []rune{'q'}, wake: make(chan struct{}, 1), uiQueue: make(chan func(), 256), widgetIDs: make(map[widget.Widget]uint64)}
}

// Run takes over the terminal and blocks until a quit key, Ctrl+C, SIGINT, or SIGTERM is received. It always restores the terminal on the way out, including on panic.
func (a *App) Run() error {
	w, h, _ := term.Size()
	a.width, a.height = w, h
	a.buf = buffer.New(w, h)
	a.relayout()

	restore := term.EnableRaw()
	defer func() {
		restore()
		if r := recover(); r != nil {
			panic(r) // re-panic after terminal is restored
		}
	}()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGWINCH)

	events := make(chan input.Event, 256)
	reader := input.NewReader(os.Stdin)
	go reader.Run(events)

	a.dirty.Store(true)
	if err := a.draw(); err != nil {
		return err
	}

	// Event-driven scheduler: while idle there is no frame ticker waking the
	// process. Invalidate() wakes it immediately; live rendering arms a bounded
	// frame timer. This avoids a permanent 30/60Hz scheduler for static TUIs.
	frameTimer := time.NewTimer(time.Hour)
	if !frameTimer.Stop() {
		<-frameTimer.C
	}
	// Cursor blinking is software-driven. Relying on SGR 5 is not reliable
	// across terminal emulators, and the hardware cursor is intentionally hidden
	// because zerotui owns cursor placement inside widgets.
	blinkTimer := time.NewTimer(500 * time.Millisecond)
	blinkTimer.Stop()
	armBlinkTimer := func() {
		if a.focus.current() == nil {
			if !blinkTimer.Stop() {
				select {
				case <-blinkTimer.C:
				default:
				}
			}
			return
		}
		if !blinkTimer.Stop() {
			select {
			case <-blinkTimer.C:
			default:
			}
		}
		blinkTimer.Reset(500 * time.Millisecond)
	}
	armFrameTimer := func() {
		if a.live.Load() <= 0 && !a.interactive.Load() {
			return
		}
		d := a.frameInterval()
		if !frameTimer.Stop() {
			select {
			case <-frameTimer.C:
			default:
			}
		}
		frameTimer.Reset(d)
	}
	defer frameTimer.Stop()
	defer blinkTimer.Stop()
	armFrameTimer()
	armBlinkTimer()

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGWINCH:
				a.handleResize()
			default:
				return nil
			}

		case ev := <-events:
			if quit := a.dispatch(ev); quit {
				return nil
			}

		case fn := <-a.uiQueue:
			if fn != nil {
				fn()
				a.requestRender()
			}

		case <-a.wake:
			if err := a.draw(); err != nil {
				return err
			}
			armFrameTimer()
			armBlinkTimer()

		case <-frameTimer.C:
			if a.live.Load() > 0 || a.interactive.Load() {
				if err := a.draw(); err != nil {
					return err
				}
				armFrameTimer()
			}

		case <-blinkTimer.C:
			if a.focus.current() != nil {
				widget.ToggleCursorBlink()
				a.Invalidate()
			}
			armBlinkTimer()
		}
	}
}

func (a *App) frameInterval() time.Duration {
	fps := a.TargetFPS
	if fps <= 0 {
		fps = 60
	}
	if a.interactive.Load() {
		if fps < 60 {
			fps = 60
		}
	}
	if fps > 120 {
		fps = 120
	}
	return time.Second / time.Duration(fps)
}

func (a *App) wakeRender() {
	if a.wake == nil {
		return
	}
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// requestRender coalesces producer wakeups. The first invalidation after an
// idle frame transition wakes the render loop; subsequent mutations while the
// frame is already pending only set the same atomic dirty bit. This is useful
// for market-data bursts because hundreds of updates can collapse into one
// render wakeup without a mutex or allocation.
func (a *App) requestRender() {
	if a.batchDepth.Load() > 0 {
		a.dirty.Store(true)
		return
	}
	if !a.dirty.Swap(true) {
		a.wakeRender()
	}
}

// Queue schedules fn on the application's render/UI goroutine. It is the
// preferred bridge from background workers into widget state: background code
// should compute or receive data off-thread, then Queue the short state mutation
// that owns the widget. fn is never run concurrently with Draw or input routing.
// Queue returns false only when the bounded queue is full; callers can retry or
// coalesce work rather than blocking a producer goroutine.
func (a *App) Queue(fn func()) bool {
	if fn == nil || a.uiQueue == nil {
		return false
	}
	select {
	case a.uiQueue <- fn:
		a.wakeRender()
		return true
	default:
		return false
	}
}

// BeginBatch coalesces a burst of invalidations into one render wakeup. It is
// useful when a producer updates several widgets as one logical transaction.
// Batching affects scheduling only; it does not hold the renderer lock.
func (a *App) BeginBatch() {
	a.batchDepth.Add(1)
}

// EndBatch releases one update batch. The final EndBatch wakes the renderer if
// anything was invalidated while the batch was open. Calls are nestable.
func (a *App) EndBatch() {
	for {
		n := a.batchDepth.Load()
		if n <= 0 {
			return
		}
		if !a.batchDepth.CompareAndSwap(n, n-1) {
			continue
		}
		if n == 1 && a.dirty.Load() {
			a.wakeRender()
		}
		return
	}
}

// SetTheme swaps the application theme without rebuilding the layout. The
// next frame repaints the screen; callers can therefore implement a runtime
// theme switch without restarting the application.
func (a *App) SetTheme(theme *style.Theme) {
	if theme == nil {
		return
	}
	a.Theme = theme
	a.Invalidate()
}

// RequestLive enables continuous frames for animation/streaming widgets. It
// is reference-counted so independent widgets can request live rendering.
func (a *App) RequestLive() { a.live.Add(1); a.wakeRender() }

// RequestLiveRect enables a bounded live-render region. Each frame, when the
// app has no ordinary damage, only these rectangles are repainted. This is the
// preferred API for localized animations such as spinners, cursors, gauges, or
// streaming charts. Returns false when the fixed region budget is exhausted.
func (a *App) RequestLiveRect(r geometry.Rect) bool {
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	a.liveRegionMu.Lock()
	defer a.liveRegionMu.Unlock()
	for i := 0; i < a.liveRegionCount; i++ {
		if a.liveRegions[i] == r {
			a.liveRegionRefs[i]++
			a.live.Add(1)
			a.wakeRender()
			return true
		}
	}
	if a.liveRegionCount >= len(a.liveRegions) {
		return false
	}
	i := a.liveRegionCount
	a.liveRegions[i] = r
	a.liveRegionRefs[i] = 1
	a.liveRegionCount++
	a.live.Add(1)
	a.wakeRender()
	return true
}

// DropLiveRect removes one live request for a localized animation. The region
// remains registered while other owners still hold a request for it.
func (a *App) DropLiveRect(r geometry.Rect) {
	a.liveRegionMu.Lock()
	defer a.liveRegionMu.Unlock()
	for i := 0; i < a.liveRegionCount; i++ {
		if a.liveRegions[i] == r {
			if a.liveRegionRefs[i] > 1 {
				a.liveRegionRefs[i]--
				a.DropLive()
				return
			}
			last := a.liveRegionCount - 1
			a.liveRegions[i] = a.liveRegions[last]
			a.liveRegionRefs[i] = a.liveRegionRefs[last]
			a.liveRegions[last] = geometry.Rect{}
			a.liveRegionRefs[last] = 0
			a.liveRegionCount--
			a.DropLive()
			return
		}
	}
}

// DropLive disables one live-render request.
func (a *App) DropLive() {
	for {
		n := a.live.Load()
		if n <= 0 {
			return
		}
		if a.live.CompareAndSwap(n, n-1) {
			return
		}
	}
}

func (a *App) handleResize() {
	w, h, _ := term.Size()
	a.handleResizeTo(w, h)
}

// handleResizeTo applies a terminal geometry change as one atomic renderer
// boundary. A resize invalidates every cell because the terminal's old screen
// geometry is no longer a valid front-buffer coordinate system.
//
// The wake is important: SIGWINCH is delivered through the event loop, not
// through the normal invalidation path. When the app is idle there is no frame
// ticker, so merely setting dirty=true would leave the resized UI invisible
// until some later input event.
func (a *App) handleResizeTo(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if w == a.width && h == a.height {
		return
	}
	a.width, a.height = w, h
	if a.buf == nil {
		a.buf = buffer.New(w, h)
	} else {
		a.buf.Resize(w, h)
	}

	// Preserve the documented ordering: callbacks observe the new terminal
	// size and the current layout before they run. A callback is allowed to
	// change responsive/layout state, so reflow once more afterwards. Resize is
	// an infrequent boundary, making this extra pass preferable to stale
	// geometry and much safer than trying to infer what the callback changed.
	a.relayout()
	if a.OnResize != nil {
		a.OnResize(w, h)
		a.relayout()
	}

	a.dirty.Store(true)
	a.damageMu.Lock()
	a.damageCount = 0
	a.damageFull = true
	a.damageMu.Unlock()
	a.wakeRender()
}

func (a *App) reflow() {
	area := geometry.Rect{X: 0, Y: 0, W: a.width, H: a.height}
	if a.computedEpoch == a.layoutEpoch && a.hasLayoutCache && a.lastLayoutArea == area {
		if a.metricsEnabled.Load() {
			a.metrics.layoutCacheHits.Add(1)
		}
		return
	}
	start := time.Now()
	a.placementsMu.Lock()
	old := a.placementState
	a.placements = layout.ComputeInto(a.Root, area, a.placements)
	a.spatial.rebuild(a.width, a.height, a.placements)
	idx := a.widgetIndex
	if idx == nil {
		idx = make(map[widget.Widget]widgetPlacements, len(a.placements))
	}
	clear(idx)
	oldByID := a.oldStateByID
	if oldByID == nil {
		oldByID = make(map[uint64]placementState, len(old))
	}
	clear(oldByID)
	for _, st := range old {
		if st.id != 0 {
			oldByID[st.id] = st
		}
	}
	state := a.placementState[:0]
	captureFound := false
	var captureArea geometry.Rect
	for i, p := range a.placements {
		if v := reflect.ValueOf(p.Widget); v.IsValid() && v.Type().Comparable() {
			if refs, ok := idx[p.Widget]; ok {
				refs.extra = append(refs.extra, i)
				idx[p.Widget] = refs
			} else {
				idx[p.Widget] = widgetPlacements{first: i}
			}
		}
		if a.mouseCapture != nil && sameWidget(p.Widget, a.mouseCapture) {
			captureFound = true
			captureArea = p.Area
		}
		id := a.nodeID(p.Widget)
		flags := DirtyLayout | DirtyPaint
		if prior, ok := oldByID[id]; ok && prior.area == p.Area {
			flags = prior.flags
			if flags == DirtyNone {
				flags = DirtyPaint
			}
		}
		state = append(state, placementState{id: id, flags: flags, area: p.Area})
	}
	a.widgetIndex = idx
	a.oldStateByID = oldByID
	a.placementState = state
	a.placementsMu.Unlock()

	// A terminal resize can happen in the middle of a mouse gesture. Keep the
	// captured widget, but refresh its hit/drag geometry to the new placement.
	// If responsive layout removed it entirely, release capture instead of
	// continuing to feed coordinates into stale geometry.
	if a.mouseCapture != nil {
		if captureFound {
			a.mouseCaptureArea = captureArea
		} else {
			a.mouseCapture = nil
			a.SetInteractive(false)
		}
	}
	a.layoutDirty.Store(false)
	a.lastLayoutArea = area
	a.lastLayoutRoot = a.Root
	a.computedEpoch = a.layoutEpoch
	a.hasLayoutCache = true
	if a.metricsEnabled.Load() {
		a.metrics.layoutPasses.Add(1)
		a.metrics.lastLayoutNS.Store(uint64(time.Since(start).Nanoseconds()))
	}
}

func (a *App) nodeID(w widget.Widget) uint64 {
	if w == nil {
		return 0
	}
	if a.widgetIDs == nil {
		a.widgetIDs = make(map[widget.Widget]uint64)
	}
	v := reflect.ValueOf(w)
	if !v.IsValid() || !v.Type().Comparable() {
		return 0
	}
	if id, ok := a.widgetIDs[w]; ok {
		return id
	}
	a.nextNodeID++
	a.widgetIDs[w] = a.nextNodeID
	return a.nextNodeID
}

// Invalidate requests a full-frame repaint. Prefer InvalidateRect for a data
// widget whose geometry is known; the renderer will then repaint only that
// damaged region.
func (a *App) Invalidate() {
	a.damageMu.Lock()
	a.damageCount = 0
	a.damageFull = true
	a.damageMu.Unlock()
	a.requestRender()
}

// InvalidateRect requests a coalesced repaint of a screen-space rectangle.
// Calls are safe from producer goroutines and do not render synchronously.
// Overlapping/touching rectangles are merged up to the fixed 32-region budget;
// on overflow the app intentionally falls back to a full-frame repaint.
func (a *App) InvalidateRect(r geometry.Rect) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	a.damageMu.Lock()
	if !a.damageFull {
		if a.damageCount >= len(a.damage) {
			a.damageCount = 0
			a.damageFull = true
		} else {
			a.addDamageLocked(r)
		}
	}
	a.damageMu.Unlock()
	a.requestRender()
}

// InvalidateWidgets invalidates the current screen regions occupied by the
// supplied widgets. This is ideal for high-frequency market-data producers: a
// Sparkline, price ticker, order book, and KPI can update independently without
// forcing a full dashboard repaint. The lookup is intentionally off the render
// hot path and is concurrency-safe with terminal resize/reflow.
func (a *App) InvalidateWidgets(widgets ...widget.Widget) {
	if len(widgets) == 0 {
		return
	}
	a.placementsMu.RLock()
	a.damageMu.Lock()
	if !a.damageFull {
		for i := range widgets {
			target := widgets[i]
			if target == nil {
				continue
			}
			if v := reflect.ValueOf(target); v.IsValid() && v.Type().Comparable() {
				if refs, ok := a.widgetIndex[target]; ok {
					a.addPlacementDamageLocked(refs.first, a.damageRegionScratch[:])
					for _, pi := range refs.extra {
						a.addPlacementDamageLocked(pi, a.damageRegionScratch[:])
						if a.damageFull {
							break
						}
					}
				}
			} else {
				for pi := range a.placements {
					if sameWidget(a.placements[pi].Widget, target) {
						a.addPlacementDamageLocked(pi, a.damageRegionScratch[:])
					}
					if a.damageFull {
						break
					}
				}
			}
			if a.damageFull {
				break
			}
		}
	}
	a.damageMu.Unlock()
	a.placementsMu.RUnlock()
	a.requestRender()
}

func (a *App) addPlacementDamageLocked(pi int, regionScratch []geometry.Rect) {
	if pi < 0 || pi >= len(a.placements) || a.damageFull {
		return
	}
	if pi < len(a.placementState) {
		a.placementState[pi].flags |= DirtyPaint
	}
	p := a.placements[pi]
	if provider, ok := p.Widget.(widget.DirtyRegionProvider); ok {
		regions := provider.DirtyRegions(p.Area, regionScratch[:0])
		if len(regions) > 0 {
			for _, rr := range regions {
				if a.damageCount >= len(a.damage) {
					a.damageCount = 0
					a.damageFull = true
					return
				}
				a.addDamageLocked(rr)
				if a.damageFull {
					return
				}
			}
			return
		}
	}
	a.addDamageLocked(p.Area)
}

func intersectRect(a, b geometry.Rect) geometry.Rect {
	x1, y1 := a.X, a.Y
	if b.X > x1 {
		x1 = b.X
	}
	if b.Y > y1 {
		y1 = b.Y
	}
	x2, y2 := a.X+a.W, a.Y+a.H
	if b.X+b.W < x2 {
		x2 = b.X + b.W
	}
	if b.Y+b.H < y2 {
		y2 = b.Y + b.H
	}
	if x2 <= x1 || y2 <= y1 {
		return geometry.Rect{}
	}
	return geometry.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

func sameWidget(a, b widget.Widget) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if !va.IsValid() || !vb.IsValid() || va.Type() != vb.Type() || !va.Type().Comparable() {
		return false
	}
	return va.Interface() == vb.Interface()
}

func (a *App) addDamageLocked(r geometry.Rect) {
	r = a.normalizeDamageRect(r)
	if r.W <= 0 || r.H <= 0 || a.damageFull {
		return
	}

	// Coalesce transitively. A new rectangle can bridge two existing regions,
	// and the union created by one merge can touch a region that appeared
	// earlier in the fixed array. Restarting the scan after each removal keeps
	// the invariant that stored damage regions are pairwise non-touching. The
	// maximum is only 32 entries and this runs under damageMu, never in Draw.
	merged := r
	for i := 0; i < a.damageCount; {
		if !rectsTouch(a.damage[i], merged) {
			i++
			continue
		}
		merged = unionRect(merged, a.damage[i])
		last := a.damageCount - 1
		a.damage[i] = a.damage[last]
		a.damage[last] = geometry.Rect{}
		a.damageCount--
	}

	if a.damageCount >= len(a.damage) {
		// Only promote after attempting all coalescing. A 33rd disjoint region
		// needs a full repaint; a region that joins an existing entry does not.
		a.damageCount = 0
		a.damageFull = true
		return
	}
	a.damage[a.damageCount] = merged
	a.damageCount++
	if a.shouldPromoteDamageLocked() {
		a.damageCount = 0
		a.damageFull = true
	}
}

func (a *App) normalizeDamageRect(r geometry.Rect) geometry.Rect {
	if a.width <= 0 || a.height <= 0 || r.W <= 0 || r.H <= 0 {
		return geometry.Rect{}
	}
	// Clamp using subtraction rather than X+W/Y+H so hostile or accidental
	// integer overflow cannot turn an out-of-bounds rectangle into a huge
	// positive damage area.
	if r.X < 0 {
		r.W += r.X
		r.X = 0
	}
	if r.Y < 0 {
		r.H += r.Y
		r.Y = 0
	}
	if r.X >= a.width || r.Y >= a.height || r.W <= 0 || r.H <= 0 {
		return geometry.Rect{}
	}
	if r.W > a.width-r.X {
		r.W = a.width - r.X
	}
	if r.H > a.height-r.Y {
		r.H = a.height - r.Y
	}
	if r.W <= 0 || r.H <= 0 {
		return geometry.Rect{}
	}
	return r
}

func (a *App) shouldPromoteDamageLocked() bool {
	if a.width <= 0 || a.height <= 0 || a.damageCount == 0 {
		return false
	}
	// Stored damage regions are pairwise non-touching, so their area sum is a
	// conservative approximation of actual coverage without requiring a union
	// structure. int64 keeps the calculation correct even for very large virtual
	// terminal dimensions.
	var area int64
	for i := 0; i < a.damageCount; i++ {
		area += int64(a.damage[i].W) * int64(a.damage[i].H)
	}
	terminalArea := int64(a.width) * int64(a.height)
	// Once the damaged coverage reaches most of the terminal, many clipped
	// paints cost more than one linear full-frame traversal. Keep the decision
	// cheap and deterministic: 55%% coverage is the promotion threshold.
	return area >= terminalArea*55/100
}

// InvalidateLayout requests a coalesced relayout. Layout changes can move many
// widgets, so the next frame intentionally uses a full repaint after reflow.
func (a *App) InvalidateLayout() {
	a.layoutEpoch++
	a.layoutDirty.Store(true)
	a.damageMu.Lock()
	a.damageCount = 0
	a.damageFull = true
	a.damageMu.Unlock()
	a.requestRender()
	// During a pointer gesture the interaction frame clock owns redraw pacing.
	// This coalesces a burst of mouse reports into at most 60–120 layout passes.
	if !a.interactive.Load() {
		a.wakeRender()
	}
}

func rectsTouch(a, b geometry.Rect) bool {
	return a.X <= b.X+b.W && b.X <= a.X+a.W && a.Y <= b.Y+b.H && b.Y <= a.Y+a.H
}

func unionRect(a, b geometry.Rect) geometry.Rect {
	x1, y1 := a.X, a.Y
	x2, y2 := a.X+a.W, a.Y+a.H
	if b.X < x1 {
		x1 = b.X
	}
	if b.Y < y1 {
		y1 = b.Y
	}
	if b.X+b.W > x2 {
		x2 = b.X + b.W
	}
	if b.Y+b.H > y2 {
		y2 = b.Y + b.H
	}
	return geometry.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

// Relayout immediately rebuilds placement geometry and the focus ring. It is
// intended for structural state changes such as opening or closing a modal.
func (a *App) Relayout() {
	a.layoutEpoch++
	a.relayout()
	a.Invalidate()
}

// SetInteractive marks the app as actively manipulated. It is intentionally
// separate from TargetFPS so callers can use a higher frame rate only while a
// pointer gesture is in flight.
func (a *App) SetInteractive(v bool) {
	a.interactive.Store(v)
	if v {
		a.wakeRender()
	}
}

// Focus moves keyboard focus to a specific focusable widget when it is present
// in the current placement tree. It is useful for command palettes and modal
// editors that temporarily own keyboard input.
func (a *App) Focus(target widget.Focusable) bool {
	if target == nil {
		return false
	}
	for i, it := range a.focus.items {
		if it == target {
			a.focus.idx = i
			a.focus.applyFocus()
			a.Invalidate()
			return true
		}
	}
	return false
}

func (a *App) relayout() {
	a.reflow()

	a.focusScratch = a.focusScratch[:0]
	for _, p := range a.placements {
		f, ok := p.Widget.(widget.Focusable)
		if !ok || f == nil {
			continue
		}
		// A widget can legitimately appear more than once in a layout tree.
		// Focus is a widget-level state, so duplicate placements must not create
		// duplicate Tab stops or make one widget receive Focus(true) and then
		// Focus(false) in the same rebuild.
		seen := false
		for _, existing := range a.focusScratch {
			if existing == f {
				seen = true
				break
			}
		}
		if !seen {
			a.focusScratch = append(a.focusScratch, f)
		}
	}
	a.focus.rebuild(a.focusScratch)
}

func (a *App) draw() error { return a.drawTo(os.Stdout) }

func (a *App) drawTo(out io.Writer) error {
	requested := a.dirty.Swap(false)
	if !requested && a.live.Load() <= 0 {
		return nil
	}
	a.drawLock.Lock()
	defer a.drawLock.Unlock()
	metricsOn := a.metricsEnabled.Load()
	var frameStart time.Time
	if metricsOn {
		frameStart = time.Now()
	}

	full := !a.hasPainted || a.layoutDirty.Load()
	a.damageMu.Lock()
	if a.damageFull {
		full = true
	}
	n := a.damageCount
	if n > len(a.damageScratch) {
		n = len(a.damageScratch)
	}
	partialPainted := 0
	partialCandidates := 0
	for i := 0; i < n; i++ {
		r := a.damage[i]
		a.damageScratch[i] = buffer.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
	}
	a.damageCount = 0
	a.damageFull = false
	for i := 0; i < n; i++ {
	}
	a.damageMu.Unlock()

	if a.layoutDirty.Load() {
		a.reflow()
		full = true
	}

	// A localized live animation should never implicitly promote to a full
	// dashboard repaint. Global RequestLive still uses the full path, while
	// RequestLiveRect contributes only its registered damage rectangles.
	if !full && n == 0 && a.live.Load() > 0 {
		a.liveRegionMu.Lock()
		if a.liveRegionCount > 0 {
			n = a.liveRegionCount
			if n > len(a.damageScratch) {
				n = len(a.damageScratch)
			}
			for i := 0; i < n; i++ {
				r := a.liveRegions[i]
				a.damageScratch[i] = buffer.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
			}
		} else {
			full = true
		}
		a.liveRegionMu.Unlock()
	}

	if full {
		a.buf.Clear(a.Theme.Background)
		for i, p := range a.placements {
			// Every placement owns a rectangular paint region. Keep widget drawing
			// inside that region so fixed-width content (tables, lists, labels,
			// etc.) cannot spill into a neighbouring panel when the terminal is
			// resized or a responsive layout becomes compact.
			a.buf.SetClip(buffer.Rect{X: p.Area.X, Y: p.Area.Y, W: p.Area.W, H: p.Area.H})
			p.Widget.Draw(a.buf, p.Area, a.Theme)
			a.buf.ClearClip()
			if provider, ok := p.Widget.(widget.DirtyRegionProvider); ok {
				// A full frame supersedes any provider-local pending damage. Consume
				// it now so the next market-data tick can stay row-local.
				provider.DirtyRegions(p.Area, a.damageRegionScratch[:0])
			}
			if i < len(a.placementState) {
				a.placementState[i].flags = DirtyNone
			}
		}
		nbytes, err := a.renderBuffer(out)
		if err != nil {
			return err
		}
		a.hasPainted = true
		if metricsOn {
			a.metrics.frames.Add(1)
			a.metrics.fullFrames.Add(1)
			a.metrics.paintedWidgets.Add(uint64(len(a.placements)))
			a.metrics.bytesWritten.Add(uint64(nbytes))
			a.metrics.lastFrameNS.Store(uint64(time.Since(frameStart).Nanoseconds()))
		}
		return nil
	}

	for i := 0; i < n; i++ {
		r := a.damageScratch[i]
		gr := geometry.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
		a.buf.ClearRegion(r, a.Theme.Background)
		a.buf.SetClip(r)
		candidates := a.spatial.candidateIndices(gr, a.placements)
		partialCandidates += len(candidates)
		// ClearRegion destroys the previously composited pixels in the damage
		// rectangle. Reconstructing them safely requires replaying every placement
		// that intersects the region in original paint order. This is deliberately
		// independent of opaque-background metadata: modal dim backdrops and other
		// compositors may transform existing pixels without owning a background.
		for ci := 0; ci < len(candidates); ci++ {
			pi := candidates[ci]
			p := a.placements[pi]
			// Partial redraws clip every replay to the intersection of the damage
			// region and placement. A widget therefore receives its original area
			// for layout calculations while writes remain confined to the damaged
			// cells.
			clip := intersectRect(gr, p.Area)
			if clip.W <= 0 || clip.H <= 0 {
				continue
			}
			a.buf.SetClip(buffer.Rect{X: clip.X, Y: clip.Y, W: clip.W, H: clip.H})
			p.Widget.Draw(a.buf, p.Area, a.Theme)
			a.buf.ClearClip()
			partialPainted++
			if pi < len(a.placementState) {
				a.placementState[pi].flags = DirtyNone
			}
		}
		a.buf.ClearClip()
	}
	nbytes, err := a.renderBufferRegions(out, a.damageScratch[:n])
	if err != nil {
		return err
	}
	a.hasPainted = true
	if metricsOn {
		a.metrics.frames.Add(1)
		a.metrics.partialFrames.Add(1)
		a.metrics.paintedWidgets.Add(uint64(partialPainted))
		a.metrics.candidateWidgets.Add(uint64(partialCandidates))
		if partialCandidates > partialPainted {
			a.metrics.skippedWidgets.Add(uint64(partialCandidates - partialPainted))
		}
		a.metrics.damageRegions.Add(uint64(n))
		a.metrics.bytesWritten.Add(uint64(nbytes))
		a.metrics.lastFrameNS.Store(uint64(time.Since(frameStart).Nanoseconds()))
	}
	return nil
}

func (a *App) renderBuffer(out io.Writer) (int, error) {
	if a.SynchronizedOutput {
		return a.buf.RenderSynchronized(out)
	}
	return a.buf.Render(out)
}

func (a *App) renderBufferRegions(out io.Writer, regions []buffer.Rect) (int, error) {
	if a.SynchronizedOutput {
		return a.buf.RenderRegionsSynchronized(out, regions)
	}
	return a.buf.RenderRegions(out, regions)
}

func (a *App) focusedArea(target widget.Focusable) geometry.Rect {
	for _, p := range a.placements {
		if f, ok := p.Widget.(widget.Focusable); ok && f == target {
			return p.Area
		}
	}
	return geometry.Rect{X: 0, Y: 0, W: a.width, H: a.height}
}

// RetainedState returns a compact snapshot of how many retained placements are
// currently dirty. It is intended for diagnostics/benchmarks and does not expose
// internal widget identity storage.
func (a *App) RetainedState() (total, dirty int) {
	a.placementsMu.RLock()
	defer a.placementsMu.RUnlock()
	total = len(a.placementState)
	for _, st := range a.placementState {
		if st.flags != DirtyNone {
			dirty++
		}
	}
	return total, dirty
}

// dispatch routes one input event; returns true if the app should quit.
func (a *App) dispatch(ev input.Event) bool {
	if ev.Resize {
		a.handleResizeTo(ev.Width, ev.Height)
		return false
	}
	if ev.Paste != "" {
		if cur := a.focus.current(); cur != nil {
			if p, ok := cur.(widget.PasteHandler); ok && p.HandlePaste(ev.Paste) {
				a.Invalidate()
				widget.ResetCursorBlink()
			}
		}
		return false
	}
	if ev.IsMouse {
		a.dispatchMouse(ev.Mouse)
		return false
	}

	k := ev.Key
	if k.Type == input.KeyCtrlC {
		// Editable widgets may legitimately use Ctrl+C for copy. Give the
		// focused widget first refusal; if it does not consume the key, retain
		// the application's traditional Ctrl+C-to-quit behavior.
		if cur := a.focus.current(); cur != nil && cur.HandleKey(k) {
			a.Invalidate()
			return false
		}
		return true
	}
	if k.Type == input.KeyRune {
		for _, q := range a.QuitKeys {
			if k.Rune == q {
				return true
			}
		}
	}
	if a.OnKey != nil && a.OnKey(k) {
		a.Invalidate()
		return false
	}
	if k.Type == input.KeyTab || k.Type == input.KeyShiftTab {
		// Give the focused editor/input control first refusal so Tab can mean
		// indentation inside an editor. Widgets that do not consume it retain
		// the normal focus-navigation behavior.
		if cur := a.focus.current(); cur != nil && cur.HandleKey(k) {
			a.Invalidate()
			return false
		}
		if k.Type == input.KeyTab {
			a.focus.next()
		} else {
			a.focus.prev()
		}
		a.Invalidate()
		return false
	}
	if cur := a.focus.current(); cur != nil {
		if cur.HandleKey(k) {
			widget.ResetCursorBlink()
			// Interactive callbacks are allowed to mutate widgets other than the
			// focused control (Tabs changes its sibling List; a Button can update
			// a status Label, etc.). A focused-region-only repaint can therefore
			// leave stale cells behind. Keyboard input is human-rate, so a full
			// repaint here is a deliberate correctness/performance trade-off.
			a.Invalidate()
		}
	}
	return false
}

func (a *App) dispatchMouse(m input.MouseEvent) {
	// Preserve drag capture even when the pointer leaves the original cell
	// range. This is important for both sliders and resizable split handles.
	if a.mouseCapture != nil {
		if a.mouseCapture.HandleMouse(m, a.mouseCaptureArea) {
			if m.Action == input.MouseRelease {
				a.mouseCapture = nil
				a.SetInteractive(false)
			}
			// Coalesce drag packets: the next frame performs the pending reflow.
			if m.Action == input.MouseDrag || m.Action == input.MousePress {
				a.InvalidateLayout()
			} else {
				a.InvalidateRect(a.mouseCaptureArea)
			}
			return
		}
		if m.Action == input.MouseRelease {
			a.mouseCapture = nil
			a.SetInteractive(false)
		}
	}
	// Iterate in reverse so widgets drawn last (visually on top / inside nested panels drawn after their siblings) win hit-testing ties.
	for i := len(a.placements) - 1; i >= 0; i-- {
		p := a.placements[i]
		if !p.Area.Contains(m.X, m.Y) {
			continue
		}
		if f, ok := p.Widget.(widget.Focusable); ok {
			if f.HandleMouse(m, p.Area) {
				if m.Action == input.MousePress {
					a.mouseCapture = f
					a.mouseCaptureArea = p.Area
					a.SetInteractive(true)
					a.wakeRender()
				}
				if m.Action == input.MousePress {
					widget.ResetCursorBlink()
				}
				for idx, it := range a.focus.items {
					if it == f {
						a.focus.idx = idx
						a.focus.applyFocus()
						break
					}
				}
				if m.Action == input.MousePress || m.Action == input.MouseRelease {
					// A mouse callback may update a different widget. Human-rate
					// clicks are cheap compared with risking stale retained cells.
					a.Invalidate()
				} else {
					a.InvalidateRect(p.Area)
				}
				return
			}
			continue
		}
		if h, ok := p.Widget.(widget.MouseHandler); ok {
			if h.HandleMouse(m, p.Area) {
				if m.Action == input.MousePress {
					a.mouseCapture = h
					a.mouseCaptureArea = p.Area
					a.SetInteractive(true)
					a.wakeRender()
				}
				// Layout handles modify shared split state. Defer the actual tree
				// reflow to the next frame so a burst of mouse reports coalesces.
				if m.Action == input.MouseDrag || m.Action == input.MousePress {
					a.InvalidateLayout()
				} else {
					a.InvalidateRect(p.Area)
				}
				return
			}
		}
	}
}
