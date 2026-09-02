# ZeroTUI Benchmark Suite

This is the canonical **full ZeroTUI-only performance suite**.
It covers the library's major runtime layers rather than only the Order Book:

- geometry
- color and style
- number formatting
- retained buffer and renderer
- every major layout family
- core widgets and modern widgets
- keyboard/mouse widget handling
- application invalidation/focus/live-render paths
- concurrent Sparkline and OrderBook workloads
- representative end-to-end trading/dashboard scenarios

The suite intentionally does **not** make cross-framework speed claims. Its purpose is to provide reproducible ZeroTUI baselines and to catch performance regressions inside the project.

## Quick start

From the repository root:

```bash
chmod +x benchmarks/run.sh
./benchmarks/run.sh
```

The default run executes every benchmark five times with allocation reporting:

```bash
go test ./benchmarks -run '^$' -bench . -benchmem -count=5
```

Useful focused runs:

```bash
# Order Book and renderer
BENCH='OrderBook|Renderer' ./benchmarks/run.sh

# All widgets
BENCH='Widget' ./benchmarks/run.sh

# Layout subsystem
BENCH='Layout|Responsive' ./benchmarks/run.sh

# Application machinery
BENCH='App' ./benchmarks/run.sh

# End-to-end scenarios
BENCH='Scenario' ./benchmarks/run.sh

# Fast development pass
COUNT=1 ./benchmarks/run.sh
```

## Coverage matrix

### Geometry

| Benchmark | Measures |
|---|---|
| `BenchmarkGeometryContains` | Point containment |
| `BenchmarkGeometryInset` | Uniform rectangle inset |
| `BenchmarkGeometryInsetXY` | Independent X/Y inset |
| `BenchmarkGeometryRow` | One-cell row construction |
| `BenchmarkGeometrySplitH` | Horizontal split |
| `BenchmarkGeometrySplitV` | Vertical split |
| `BenchmarkGeometryBatch` | Repeated rectangle transformations |
| `BenchmarkGeometryContainsBatch` | Repeated hit testing |

### Color and style

| Benchmark | Measures |
|---|---|
| `BenchmarkColorRGB` | RGB packing |
| `BenchmarkColorComponents` | RGB unpacking |
| `BenchmarkColorLerp` | Truecolor interpolation |
| `BenchmarkStyleComposition` | Foreground/background style composition |
| `BenchmarkStyleAttributes` | Attribute composition/query |
| `BenchmarkThemeConstruction` | Theme creation |

### Number formatting

| Benchmark | Measures |
|---|---|
| `BenchmarkNumfmtAppendUint` | Allocation-free unsigned formatting |
| `BenchmarkNumfmtAppendInt` | Allocation-free signed formatting |
| `BenchmarkNumfmtAppendFixed` | Fixed-point formatting |
| `BenchmarkNumfmtAppendFixedPrec` | Reduced-precision fixed-point formatting |
| `BenchmarkNumfmtPadLeft` | In-place numeric padding |
| `BenchmarkNumfmtStrconvReference` | `strconv.AppendUint` reference workload |

### Buffer and renderer

| Benchmark | Measures |
|---|---|
| `BenchmarkBufferSetString` | String placement |
| `BenchmarkBufferSetPaddedString` | Aligned/padded string placement |
| `BenchmarkBufferFillRect` | Rectangular fill |
| `BenchmarkBufferRenderRegions` | Sparse retained rendering |
| `BenchmarkBufferRenderFull` | Full-buffer rendering |
| `BenchmarkBufferRenderSynchronized` | Synchronized sparse rendering |
| `BenchmarkBufferResize` | Backing-buffer resize |
| `BenchmarkRendererSparseCell` | One-cell damage/render path |
| `BenchmarkRendererFullFrame` | Full 120×40 repaint |
| `BenchmarkRendererSynchronizedSparse` | One-cell synchronized output |

### Layout

The suite exercises all of the shipped layout families:

- Flex horizontal and vertical
- Split
- Grid
- Stack
- Centered layout
- Responsive breakpoint layout
- Overlay
- Bordered
- Closable
- deeply nested layout trees
- large 256-node trees
- resize/reflow with placement-buffer reuse

Representative benchmarks include:

`BenchmarkLayoutFlexHorizontal`, `BenchmarkLayoutFlexVertical`, `BenchmarkLayoutSplit`, `BenchmarkLayoutGrid`, `BenchmarkLayoutStack`, `BenchmarkLayoutCenter`, `BenchmarkLayoutResponsive`, `BenchmarkLayoutOverlay`, `BenchmarkLayoutBordered`, `BenchmarkLayoutClosable`, `BenchmarkLayoutNested`, `BenchmarkLayoutLargeTree`, and `BenchmarkLayoutResizeReflow`.

### Widgets

The widget suite covers the shipped widget families:

- Label
- Button
- Toggle
- Slider
- TextInput
- Gauge
- Tabs
- List
- Table
- VirtualList
- VirtualTable
- PriceTicker
- Sparkline
- OrderBook
- FastLogView
- Panel
- Badge
- Divider
- Stat
- Spinner
- ScrollBar
- GradientBar
- CloseButton
- ResizeHandle
- CommandPalette

Every widget benchmark measures its `Draw` path where the widget exposes drawing, while focusable/pointer-driven widgets additionally have keyboard or mouse handling benchmarks where meaningful.

The virtual widgets deliberately use million-row datasets with a small viewport to verify that the benchmark exercises the visible-window model rather than materializing the dataset.

### Input and interaction

Interaction coverage includes:

- Toggle keyboard handling
- Slider keyboard handling
- TextInput keyboard handling
- List keyboard handling
- VirtualList keyboard handling
- Table keyboard handling
- VirtualTable keyboard handling
- generic widget mouse handling
- VirtualList wheel handling
- VirtualTable wheel handling
- ResizeHandle pointer handling

The parser itself is an I/O boundary and is covered by the package's correctness tests; the benchmark suite concentrates on the allocation-sensitive interaction/routing work that occurs after an event has been decoded.

### Application

The public application machinery is benchmarked through:

- `Relayout`
- `Invalidate`
- `InvalidateRect`
- targeted `InvalidateWidgets`
- focus targeting
- bounded live-render regions
- reference-counted live rendering
- interactive-mode switching
- retained-state inspection

These are intentionally separated from widget drawing so regressions in application scheduling/invalidation do not disappear inside an end-to-end number.

### Concurrency

The concurrent suite uses `testing.B.RunParallel` to exercise:

- concurrent Sparkline pushes
- concurrent Sparkline push + draw
- concurrent OrderBook updates
- concurrent OrderBook update + draw

Run the benchmark suite with the race detector separately when validating synchronization changes:

```bash
go test -race ./widget ./app ./layout ./buffer ./input
```

The race detector is a correctness tool, not a normal performance benchmark, so its timings should not be compared with ordinary `-bench` results.

### End-to-end scenarios

| Benchmark | Measures |
|---|---|
| `BenchmarkScenarioTradingDashboardLayoutDraw` | Layout + widget draw + full buffer render for a representative market dashboard |
| `BenchmarkScenarioTradingDashboardResize` | Repeated responsive dashboard reflow |
| `BenchmarkScenarioLargeVirtualTable` | Million-row virtual table viewport |
| `BenchmarkScenarioNestedLayout` | Deep mixed layout tree |
| `BenchmarkScenarioPartialWidgetUpdate` | Localized ticker update + regional render |
| `BenchmarkScenarioFullDashboardRedraw` | Full dashboard repaint path |
| `BenchmarkScenarioHighFrequencyMarketUpdate` | Ticker + Sparkline + OrderBook update/render loop |

These scenarios are the closest thing in the suite to application-level workload measurements.

## Order Book coverage

The Order Book retains the original performance-focused benchmarks because it is a primary ZeroTUI workload:

- `BenchmarkOrderBookTick` at 10/25/50/100 levels
- `BenchmarkOrderBookTenLevelUpdate`
- `BenchmarkOrderBookBestBidAsk`
- `BenchmarkOrderBookFullRefresh`
- `BenchmarkOrderBookResize`
- `BenchmarkOrderBookThemeChange`
- `BenchmarkOrderBookExplicitBackground`
- `BenchmarkWidgetOrderBookDraw`
- concurrent update/draw benchmarks
- high-frequency end-to-end market updates

The tick benchmarks use a clipped row region and `RenderRegions`, matching the retained/damage-oriented update path. Full-refresh benchmarks intentionally repaint everything so the cost of the larger workload remains visible.

## Allocation policy

All benchmarks use `-benchmem` and call `ReportAllocs()` around the measured section.

ZeroTUI's design target is allocation-free steady-state rendering for its hot paths. A benchmark reporting `0 B/op` and `0 allocs/op` is therefore meaningful and should be protected against regressions.

Not every operation is expected to be zero-allocation. Examples include application/layout construction, resize boundaries, or an input operation whose API necessarily constructs a string. Those should be monitored rather than artificially hidden.

## Reproducibility

Record the environment with every published benchmark result:

```bash
go version
uname -a
nproc
lscpu | head -25
```

Then run:

```bash
COUNT=5 ./benchmarks/run.sh
```

For a before/after comparison, keep the machine, Go version, compiler settings, benchmark count, and background workload consistent:

```bash
COUNT=5 ./benchmarks/run.sh > /tmp/zerotui-before.txt
# make the change
COUNT=5 ./benchmarks/run.sh > /tmp/zerotui-after.txt
diff -u /tmp/zerotui-before.txt /tmp/zerotui-after.txt
```

For a release report, publish the actual output from the release machine rather than copying reference numbers from another CPU.

## Correctness before publishing numbers

Run the relevant tests first:

```bash
go test ./app ./buffer ./input ./layout ./widget ./geometry ./style ./numfmt
```

Then run the benchmark suite:

```bash
./benchmarks/run.sh
```

Finally, for a release candidate:

```bash
go test ./...
go test -race ./...
go vet ./...
```
