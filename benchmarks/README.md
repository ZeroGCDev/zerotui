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

The default run executes the canonical dashboard/core benchmark package five times with allocation reporting. The repository also has editor-specific benchmarks in `widget` and `editor` so the large-document/code-editing paths are tracked independently:

```bash
go test ./benchmarks -run '^$' -bench . -benchmem -count=5
go test ./widget ./editor -run '^$' -bench 'Benchmark(CodeEditor|EditorDraw|TextEditor)' -benchmem -count=5
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

### Theme stress testing

The theme stress suite exercises every built-in editor theme rather than sampling four palettes. It covers repeated factory construction, clone/normalization, name lookup, and steady-state text-editor drawing across all 14 themes. The suite is allocation-aware and is intended to catch regressions where a theme switch or preview accidentally parses assets repeatedly or paints through per-frame allocations.

| Benchmark | Measures |
|---|---|
| `BenchmarkThemeStress_ConstructionAll` | Repeated construction across all built-in themes |
| `BenchmarkThemeStress_CloneAndNormalizeAll` | Clone + editor-background normalization |
| `BenchmarkThemeStress_LookupAll` | Case-insensitive named theme lookup |
| `BenchmarkThemeStress_TextEditorDrawAll` | Steady-state editor rendering while cycling all themes |
| `BenchmarkThemeStress_EachFactory` | Per-theme factory cost and allocations |

The built-in Zed theme is cached after its first parse, so repeated theme previews do not re-parse the embedded JSON. Returned theme values remain independent and safe for callers to mutate.

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
- Terminal
- TextEditor
- CodeEditor
- TreeView
- FilePicker
- ShortcutHelpBar
- TimeAndSales
- PositionList / PositionPanel
- Orders
- PnL
- LatencyMonitor
- RiskMonitor
- MarketStatus
- OrderEntry

Editor and terminal benchmarks live in the `widget`/`editor` packages rather than the canonical `benchmarks` package because their fixtures are substantially larger or require PTY/editor state.

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

### Editor and developer-tooling benchmarks

The newer editor stack is intentionally benchmarked separately because it exercises different workloads from a market dashboard:

| Benchmark | Measures |
|---|---|
| `BenchmarkCodeEditorViewportDraw` | Allocation-free syntax-aware viewport rendering on a 10,000-line source |
| `BenchmarkCodeEditorSearch10000` | Search-hit progression through a 10,000-line source |
| `BenchmarkCodeEditorEditAndHighlight` | Interactive edit plus document/syntax update work |
| `BenchmarkEditorDraw1000Lines` | Complete editor composition: file tree + tabs + code view + status UI |
| `BenchmarkTextEditorDraw500K` | Large plain-text viewport rendering over 500,000 lines |
| `BenchmarkTextEditorSearch500K` | Search path over a 500,000-line text document |

The built-in code editor uses range/viewport-oriented syntax work and a large-document token cache. These benchmarks therefore distinguish steady-state viewport repaint from content-changing edits, where syntax and fold state may legitimately be invalidated.

The editor benchmarks were also run during the latest review on Linux `amd64` with an AMD EPYC 9V74 80-Core Processor. The runner had Go 1.23.2 available, so the module version was lowered only in a temporary working copy for compatibility; the repository's committed `go.mod` remains at Go 1.27.0. The focused review pass used a 200 ms measurement window and `-benchmem`.

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
