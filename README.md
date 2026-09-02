# ZeroTUI is a retained-mode, low-allocation terminal UI library for Go.

ZeroTUI is designed for applications where terminal rendering is part of a real-time workload: market-data terminals, trading dashboards, operations consoles, telemetry viewers, risk monitors, and dense interactive tools.

The core idea is simple:
Update data frequently, repaint only what changed, and keep the terminal renderer out of the allocation-heavy path.

Many terminal applications spend far more work rebuilding text than the user can actually see changing. ZeroTUI takes a different approach: widgets draw into a retained cell buffer, the compositor tracks damage, and the terminal writer emits only changed cells.

This makes ZeroTUI a particularly good fit when:

- values change many times per second;
- only small regions of the screen change between frames;
- predictable memory behavior matters;
- dashboards contain large tables or virtualized data sets;
- terminal resizing and mouse interaction must stay responsive;
- you want a small, Go-native layout and widget stack rather than a browser or CGO dependency.

### Installation

```bash
go get github.com/ZeroGCDev/zerotui
```

## Widgets and layout

Included widgets include:

`Label` · `Button` · `Toggle` · `Slider` · `Gauge` · `Sparkline` · `Table` · `VirtualTable` · `List` · `VirtualList` · `Tabs` · `TextInput` · `Panel` · `PriceTicker` · `OrderBook` · `FastLogView` · `CommandPalette` · `Spinner` · `Stat` · `GradientBar` · `ResizeHandle` · `CloseButton`

Layout primitives include:

`Flex` · `Grid` · `Split` · `Stack` · `Responsive` · `Center` · `Bordered` · `ClosablePanel` · `Retained`

Interactive widgets implement the focus and mouse contracts used by `app.App`, so applications do not need to build a separate routing layer for every component.

### Simple Example:

```go
package main

import (
    "fmt"

    "github.com/ZeroGCDev/zerotui/app"
    "github.com/ZeroGCDev/zerotui/layout"
    "github.com/ZeroGCDev/zerotui/style"
    "github.com/ZeroGCDev/zerotui/widget"
)

func main() {
    var enabled uint32

    status := widget.NewLabel("Notifications are OFF")
    toggle := widget.NewToggle("Notifications", &enabled)
    button := widget.NewButton("SHOW STATUS", func() {
        if enabled == 1 {
            status.SetText("Notifications are ON")
        } else {
            status.SetText("Notifications are OFF")
        }
    })

    root := layout.NewFlex(layout.Vertical,
        layout.Fix(layout.Wrap(widget.NewLabel("ZeroTUI")), 1),
        layout.Fix(layout.Wrap(toggle), 1),
        layout.Fix(layout.Wrap(button), 1),
        layout.Fix(layout.Wrap(status), 1),
    )

    if err := app.New(root, style.TokyoNightTheme()).Run(); err != nil {
        fmt.Println(err)
    }
}
```

<img width="1000" height="563" alt="Image" src="https://github.com/user-attachments/assets/212c333f-dc5b-47ad-85de-74ee5fe0e603" />

### Examples:
```bash
go run ./examples/dashboard
go run ./examples/controls
```
 • Click `x` of each panel to close  • Use Mouse to Scroll •  Press `1` Controls  `2` Market  `3` Table to Reopening •  Drag the dividers to resize each Panel  • Press `q` Quit",
 
### ThemeOverride

ZeroTUI lets each rendered component opt into its own `*style.Theme` through `ThemeOverride`. A component theme controls the complete visual palette for that component: text/foreground colours, backgrounds, borders, focus state, selection, positive/negative/warning colours, titles and scrollbar roles. The application theme remains the default, so you only override the components that need a custom appearance.

```go
base := style.TokyoNightTheme()
buttonTheme := *base
buttonTheme.Positive = buttonTheme.Positive.WithFg(color.TokyoGreen)
buttonTheme.Selected = buttonTheme.Selected.WithBg(color.TokyoBlue)

button := widget.NewButton("RUN", run)
button.ThemeOverride = &buttonTheme
```
Component typography can be controlled through the theme's foreground/background and terminal attributes such as `Bold`, `Dim`, `Underline`, `Reverse` and `Blink`.


### Runtime theme switching

```go
a.SetTheme(style.NordTheme())
```

This is a visual change only. Existing component `ThemeOverride` values continue to take precedence for components that intentionally use their own palette.

### Exact component dimensions

Layout controls widget dimensions. `Flex` already supports fixed main-axis sizes with `Fix(...)` and flexible sizes with `Flex1(...)`/`FlexN(...)`. It now also supports optional `Item.Width` and `Item.Height` cross-axis constraints. For an explicit width and height, use `layout.FixedSize(...)`:

```go
layout.Fix(layout.FixedSize(layout.Wrap(button), 32, 3), 3)
```

`FixedSize` clamps to the available terminal area and centers the component. It is a layout-time operation and therefore does not add render-loop allocations.

### Bounded component geometry

```go
root := layout.Padding(
    layout.SizeBounds(layout.Wrap(card), 30, 60, 6, 12),
    2, 1, 2, 1,
)
```

This keeps sizing concerns in the layout layer instead of making widgets perform geometry work during every draw.


### Rendering architecture

ZeroTUI uses four cooperating layers:

1. **Layout** computes widget placement and can reuse retained placement storage.
2. **Widgets** paint cells into a reusable back buffer.
3. **Damage tracking** identifies the smallest screen regions that need repainting.
4. **The renderer** diffs those cells against the front buffer and emits changed terminal runs.

A normal live update therefore does not need to clear and redraw the entire terminal.

### Input and terminal behavior

The input parser keeps a fast ASCII path while supporting UTF-8 runes, SGR mouse events, and Kitty/progressive keyboard reports.

The renderer can optionally wrap changed frames in DEC synchronized-output mode 2026. It is enabled by default by `app.New` and can be disabled with the application's `SynchronizedOutput` field when terminal compatibility or application policy requires it.

### Performance

The benchmark suite is part of the project, not an afterthought. Run it on the machine that matters to you:

```bash
chmod +x benchmarks/run.sh
./benchmarks/run.sh
```

Or use the standard Go command directly:

```bash
go test ./benchmarks -run '^$' -bench . -benchmem -count=5
```
### Some representative results

| Operation | Typical result |
|---|---:|
| Geometry containment check | ~2.2 ns |
| Geometry inset | ~1.2 ns |
| Style composition | ~0.27 ns |
| Color interpolation | ~4.7 ns |
| Number formatting | ~15–17 ns |
| Buffer string write | ~39 ns |
| Layout split | ~37 ns |
| Layout reflow | ~115 ns |
| Responsive layout calculation | ~64 ns |
| Sparse renderer update | ~100 ns |
| Synchronized sparse render | ~105 ns |
| Full renderer frame | ~28 µs |
| Application invalidation | ~18 ns |
| Targeted widget invalidation | ~30 ns |
| Focus operation | ~24 ns |
| PriceTicker draw | ~80 ns |
| Label draw | ~88 ns |
| Button draw | ~288 ns |
| Toggle draw | ~355 ns |
| TextInput draw | ~240 ns |
| Sparkline draw | ~1.1 µs |
| OrderBook draw | ~22 µs |
| VirtualList draw | ~6.8 µs |
| VirtualTable draw | ~14 µs |
| High-frequency market update scenario | ~21–22 µs |

Most of these operations report **0 B/op and 0 allocations/op**.

These benchmarks were run on:

- **CPU:** Intel Core i5-8265U @ 1.60 GHz
- **OS:** Linux amd64
- **Go:** 1.27.0
- **Benchmark repetitions:** 5

Suppose a market-data producer receives hundreds of updates but the terminal only needs to show the latest state at the next render opportunity. ZeroTUI lets the data plane mutate application state independently from the terminal's human-scale presentation cadence. The renderer can coalesce changes and emit only the final damaged cells.

For a typical application, this means ZeroTUI can handle:

- frequent market/data updates without constantly creating garbage
- partial UI updates without redrawing the entire screen
- large lists and tables without rendering every item
- responsive layouts and window resizing with very small overhead
- keyboard and mouse interaction at extremely low cost
- concurrent data updates while the UI is rendering

ZeroTUI also includes VirtualList and VirtualTable widgets designed for large datasets.

The benchmark suite includes a large virtual-table scenario and measures rendering independently from the size of the underlying dataset. The purpose of virtualization is simple:

> You should not have to draw thousands of terminal rows just because your application contains thousands of records.

This makes ZeroTUI suitable for applications such as:

- trading terminals
- monitoring dashboards
- log viewers
- system administration tools
- database viewers
- developer tools
- real-time telemetry
- command-line business applications

The benchmark suite also exercises concurrent Sparkline and OrderBook updates.

The Sparkline update operation measures around **85–92 ns** per push with no allocations, while the OrderBook level update measures roughly **590–615 ns** with no allocations.

A combined OrderBook update-and-draw workload is measured in the roughly **11–15 µs** range in the supplied benchmark run.

This does not mean every application will achieve the same frame rate. Actual performance depends on terminal size, terminal emulator speed, application logic, data volume, operating system and hardware.

It does show that the UI library itself can keep the cost of common real-time operations very small.

For example, changing a price in an Order Book does not necessarily require rebuilding and redrawing the entire dashboard. ZeroTUI's invalidation and dirty-region system allows updates to remain localized.

The benchmark for a targeted widget update is approximately **95 ns**, while a complete dashboard redraw is around **92–93 µs** on the test machine.

That is roughly a **1,000× difference in the amount of work measured by these two benchmark scenarios**.

This is one of the important reasons to use ZeroTUI for applications with frequently changing data: **small changes can remain small.**

> **Benchmark note:** These numbers are measurements of ZeroTUI on the hardware and software listed above, not guarantees of application-level performance. Use them as an indication of the library's overhead rather than a promise of a specific FPS or latency on every machine.

## License

ZeroTUI is released under the GNU General Public License v3.0. See [`LICENSE`](LICENSE).
