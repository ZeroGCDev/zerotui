# ZeroTUI

**A retained-mode, low-allocation terminal UI library for Go.**

Update data frequently, repaint only what changed, and keep the terminal renderer out of the allocation-heavy path.

Most terminal applications spend far more CPU time rebuilding text the user cannot even see change than they spend actually changing it — a whole frame's worth of `fmt.Sprintf` calls just to update one number in the corner of the screen. ZeroTUI takes a different approach: widgets draw into a retained cell buffer, a compositor tracks exactly which cells got dirty, and the terminal writer emits only the runs that actually changed. The rest of the screen is simply left alone, because it didn't need anything.

### Latest recorded HFT stress results

The cross-framework results below are from the latest supplied 30-second run, using the same deterministic workload for ZeroTUI, Bubble Tea v2 + Lip Gloss v2, and Ratatui. The target was **1,000 logical ticks/sec** with rendering capped at **60 FPS**.

**Benchmark host:** Linux `amd64`, **Intel Xeon E312xx (Sandy Bridge, IBRS update)**, `root@ubuntu`. Sustained container limits: **4 CPUs, 2 GB RAM**.

```text
go version go1.27.1 linux/amd64
rustc 1.98.0 (88d9e12ae 2026-08-18)
cargo 1.98.0 (797e8a9bc 2026-08-05)
github.com/ZeroGCDev/zerotui
charm.land/bubbletea/v2 v2.0.9
charm.land/lipgloss/v2 v2.0.6
ratatui v0.30.2
```

**Scope of these numbers (read this first):**

- The sustained comparison is an **idiomatic cross-framework comparison**, not a controlled equivalent-code comparison. Each dashboard is written in its own library's natural style, so per-framework formatting strategy is a real part of the result.
- Allocation accounting differs by runtime: Go targets report Go runtime `TotalAlloc` deltas; Ratatui reports a custom `CountingAllocator`'s `layout.size()` sum. These are not the same definition. "No GC" does not mean "no allocation" — Rust frees deterministically via `drop()`, but `format!` still calls the allocator.
- Bubble Tea's "worst density" tick counts are a **counter artifact** (see footnote), not a throughput measurement.
- No framework recorded actual frames rendered, so "strict 60 FPS" is a requested cadence, not a verified one.

| Metric | ZeroTUI | Bubble Tea v2 | Ratatui |
| --- | ---: | ---: | ---: |
| **Realistic sustained ticks/sec** (20 rows) | **973.5** | 910.0 | **~1,000** |
| **Realistic sustained ticks/sec** (500 rows) | **979.9** | 900.2 | **~1,000** |
| **Realistic allocation rate** (20 rows) | **35.5 KB/s** | 26.3 MB/s | 7.6 MB/s |
| **Realistic allocation rate** (500 rows) | **35.3 KB/s** | 25.5 MB/s | 7.5 MB/s |
| **Full-frame render** (20-row headless microbench) | **~79.5 µs** | ~96.8 µs | ~253.9 µs |
| **Full-frame render** (500-row headless microbench) | **~79.6 µs** | ~88.0 µs | ~258.5 µs |
| **Go GC cycles** (realistic sustained) | **0** | 369 / 120 | N/A |

* **Throughput:** Ratatui is the fastest raw sustained logical-tick processor in the recorded run. ZeroTUI stays close to the 1 kHz target in realistic mode and ahead of Bubble Tea on both realistic row counts. Bubble Tea's worst-density tick counters over-count by a factor of `rows`; corrected logical throughput is ~910 ticks/sec (20 rows) and ~894 ticks/sec (500 rows).
* **Allocation (definition caveat applies):** ZeroTUI's realistic allocation traffic is roughly **99.87% lower than Bubble Tea** and roughly **99.5% lower than Ratatui** in these measurements. The ZeroTUI figure reflects its zero-allocation `numfmt` render path; the other two format with `fmt.Sprintf` / `format!` per tick. Ratatui's allocation is transient — freed deterministically by `drop()` — which is why it shows allocation traffic but zero GC pauses.
* **Rendering (headless microbench):** ZeroTUI's full-frame render was about **18% faster than Bubble Tea at 20 rows** and about **10% faster at 500 rows**. Ratatui's measured frame time was substantially higher in this particular workload.
* **Worst-case 500-row allocation:** ZeroTUI measured **15.3 MB/s**, the same order as Ratatui's **14.3 MB/s**, and about **71% below Bubble Tea's 52.7 MB/s**.

## Table of contents

- [Features](#features)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Widgets & layout primitives](#widgets--layout-primitives)
- [Examples](#examples)
- [Editor & developer tooling](#editor--developer-tooling)
- [Documentation](#documentation)
  - [Colors, styles & theming](#colors-styles--theming)
  - [Layout system](#layout-system)
  - [Widgets reference](#widgets-reference)
  - [Number formatting (numfmt)](#number-formatting-numfmt)
  - [Application, focus & input](#application-focus--input)
- [Rendering architecture](#rendering-architecture)
- [Input and terminal behavior](#input-and-terminal-behavior)
- [Performance](#performance)
- [License](#license)

## Features

None of this means ZeroTUI is the right tool for every terminal app — a settings screen that redraws twice a minute doesn't need any of it. It earns its keep specifically when:

- values change many times per second;
- only small regions of the screen change between frames;
- predictable memory behavior matters;
- dashboards contain large tables or virtualized data sets;
- terminal resizing and mouse interaction must stay responsive;
- you want a small, Go-native layout and widget stack rather than a browser or CGO dependency.

## Installation

```bash
go get github.com/ZeroGCDev/zerotui
```

## Quick start

A ZeroTUI application needs three things:

1. a widget,
2. a layout root,
3. an `app.App`.

<details open>
<summary><strong>Minimal example</strong></summary>

``` go
package main

import (
    "github.com/ZeroGCDev/zerotui/app"
    "github.com/ZeroGCDev/zerotui/layout"
    "github.com/ZeroGCDev/zerotui/style"
    "github.com/ZeroGCDev/zerotui/widget"
)

func main() {
    hello := widget.NewLabel("Hello, ZeroTUI!")

    root := layout.Wrap(hello)

    app.New(root, style.NordTheme()).Run()
}
```

`layout.Wrap` converts a `widget.Widget` into a `layout.Node`. That distinction is important:

- **Widget** — draws and optionally handles input.
- **Layout Node** — decides where one or more widgets are placed.

Once `Run()` starts, ZeroTUI owns terminal input/rendering until the application exits.

A widget paints something:

``` go
label := widget.NewLabel("CPU: 42%")
```

A layout decides where that widget lives:

``` go
root := layout.FixedSize(
    layout.Wrap(label),
    30,
    3,
)
```

</details>

<details open>
<summary><strong>Interactive examples</strong></summary>

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

```go
package main

import (
	"fmt"
	"time"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.CatppuccinMochaTheme()

	revenue := widget.NewStat("REVENUE", "$128.4K")
	revenue.Delta, revenue.Up = "+14.8%", true
	orders := widget.NewStat("ORDERS", "2,481")
	orders.Delta, orders.Up = "+8.2%", true
	latency := widget.NewStat("LATENCY", "3.8 ms")
	latency.Delta, latency.Down = "-0.6 ms", true

	cpu := widget.NewGauge("CPU")
	cpu.ValueFn = func() float64 { return 0.42 + 0.08*float64(time.Now().Unix()%5)/4 }
	cpu.WarnAt, cpu.DangerAt = .70, .90

	memory := widget.NewGradientBar(color.Cyan, color.Magenta, color.DimGray)
	memory.Value = .68

	traffic := widget.NewSparkline(40)
	for i := 0; i < 40; i++ {
		traffic.Push(40 + float64((i*i)%31))
	}

	health := widget.NewList([]string{
		"● API gateway       healthy",
		"● Database          healthy",
		"● Worker queue      healthy",
		"● Object storage    healthy",
		"● Search            healthy",
	})
	health.Background = &color.Panel

	badge := widget.NewBadge("LIVE")
	badge.Positive = true

	stats := layout.NewGrid(1, 3,
		layout.Wrap(revenue), layout.Wrap(orders), layout.Wrap(latency),
	)

	top := layout.BorderedRounded("KEY METRICS", stats, nil)
	chart := layout.BorderedRounded("TRAFFIC • LAST 40 SAMPLES", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(traffic), 4),
		layout.Fix(layout.Wrap(memory), 1),
	), nil)

	right := layout.BorderedRounded("SYSTEM HEALTH", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(badge), 1),
		layout.Flex1(layout.Wrap(health)),
		layout.Fix(layout.Wrap(cpu), 1),
	), nil)

	body := layout.NewSplit(layout.Horizontal, chart, right, .68)
	root := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("  NOVA • OPERATIONS")), 1),
		layout.Fix(top, 4),
		layout.Flex1(body),
		layout.Fix(layout.Wrap(widget.NewLabel("  [q] quit")), 1),
	)

	if err := app.New(root, theme).Run(); err != nil {
		fmt.Println(err)
	}
}
```
</details>


## Widgets & layout primitives

| Widgets | Layout primitives |
|---|---|
| `Label` · `Button` · `Toggle` · `Slider` · `Gauge` · `Sparkline` · `Heatmap` · `PriceTicker` · `OrderBook` · `Table` · `VirtualTable` · `VirtualList` · `List` · `Tabs` · `TextInput` · `FastLogView` · `CommandPalette` · `Badge` · `Divider` · `Stat` · `Spinner` · `GradientBar` · `ScrollBar` · `ResizeHandle` · `CloseButton` · `Terminal` · `TextEditor` · `CodeEditor` · `TreeView` · `FilePicker` · `ShortcutHelpBar` · `TimeAndSales` · `PositionList` · `PositionPanel` · `Orders` · `PnL` · `LatencyMonitor` · `RiskMonitor` · `MarketStatus` · `OrderEntry` | `Flex` · `Grid` · `Split` · `FixedSize` · `SizeBounds` · `Padding` · `Stack` · `Overlay` · `Modal` · `Center` · `Bordered` · `BorderedRounded` · `ClosableRounded` · `Responsive` · `Retained` · `FitHeight` · `Wrap` |

Interactive widgets use the focus and mouse contracts provided by `app.App`. Layout nodes determine placement and sizing without turning layout containers into widgets.

## Examples

The fastest way to get a feel for ZeroTUI isn't reading about it — it's running it. Ten runnable showcases ship inside `examples/`. Each is a real, self-contained `package main`, so `go run` is all you need:

```bash
go run ./examples/showcase_dashboard
```

<img width="1000" height="563" alt="Image" src="https://github.com/user-attachments/assets/212c333f-dc5b-47ad-85de-74ee5fe0e603" />

`showcase_dashboard` is the one pictured above — a live operations console with closable panels, a scrolling order book, a price ticker, and a sparkline all updating at once:

- Click `x` on a panel to close it
- Use the mouse to scroll
- Press `1` / `2` / `3` to reopen Controls / Market / Table
- Press `t` to swap between the Nord and Tokyo Night themes live
- Drag the dividers to resize each panel
- Press `q` to quit

<img width="1080" height="608" alt="Image" src="https://github.com/user-attachments/assets/2509e92c-09c9-4d1b-a3f1-17bd72884ea3" />

The screenshot above is `showcase_ide` — the reference editor build described in the next section. Press `Ctrl+c` to quit.

The rest of the showcases are worth a look too — together they exercise almost every widget in the library:

| Run it | What it shows |
| --- | --- |
| `go run ./examples/showcase_dashboard` | Closable panels, live order book, price ticker, sparkline, theme switching |
| `go run ./examples/showcase_ide` | The full `editor.Editor`: file tree, tabs, syntax highlighting, embedded terminal |
| `go run ./examples/showcase_trading` | Positions, orders, PnL, latency, risk, and market-status widgets side by side |
| `go run ./examples/showcase_terminal` | Order book, price ticker, sparkline, and a live log feed in one operations layout |
| `go run ./examples/showcase_layouts` | Panels and labels composed to demonstrate the layout primitives themselves |
| `go run ./examples/showcase_controls` | A calm, single-accent screen of buttons, toggles, sliders, and text inputs |
| `go run ./examples/showcase_settings` | A settings screen: toggles, sliders, badges, gradient bars, and dividers |
| `go run ./examples/showcase_observability` | Gauges, stats, a spinner, a fast log view, a scrolling worker-load heatmap, and a command palette |
| `go run ./examples/showcase_tasks` | Tables, tabs, a plain list, and a virtualized list working together |
| `go run ./examples/showcase_static` | A quieter, mostly-static dashboard layout for lower-refresh-rate use cases |

Every showcase quits with `q` or `Ctrl+c` unless noted otherwise on screen.

## Editor & developer tooling

ZeroTUI includes an editor surface intended for real terminal-based development workflows, not just a text box. The high-level `editor.Editor` composes the reusable `widget.CodeEditor` with a file tree, tabs, terminal pane, search/settings panes, and resizable splits.

### Code editing capabilities

- **File explorer:** open, create, rename and delete files/folders, with a collapsible tree.
- **Tabs:** multiple open files, active-tab switching, tab scrolling and close handling.
- **Editing:** cursor movement, word movement, selections, cut/copy/paste, insertion/deletion, duplicate line, delete line, comment toggling, indentation/outdent, page navigation and goto-line.
- **History:** document-level undo/redo with bounded history support.
- **Search:** in-editor forward search and search-hit tracking.
- **Folding:** brace-aware/code-aware fold ranges with hidden-line indexing.
- **Syntax highlighting:** lightweight lexical highlighting without an AST; Go has a fused highlighter/fold scanner, while other language profiles use range-based scanning.
- **Large files:** files at or above the large-document threshold use viewport token caching, so steady-state repaint work is proportional to the visible region instead of repeatedly highlighting the whole document.
- **Persistence:** the `editor.CodeViewer` adapter saves through the host filesystem and preserves file permissions when available.
- **Embedded terminal:** the editor can open a disposable PTY terminal beneath the code view, with resize and focus routing.
- **Mouse/keyboard interaction:** the editor participates in ZeroTUI's normal focus and mouse contracts.

> **A clipboard note.** Cut/copy/paste inside `TextEditor`, `CodeEditor`, and `Terminal` go through the `clipboard` package, which is intentionally process-local — it never touches the host OS clipboard, and never emits a clipboard escape sequence like OSC 52. Copying inside ZeroTUI and pasting into another terminal window will not work, and that's by design rather than an oversight. See [Clipboard](#clipboard) in the widgets reference for the two-function API.

### Syntax coverage

The built-in profiles currently cover **Go plus Python, JSON, YAML/YML, TOML, Rust, C/C++ headers and sources, Java, JavaScript/JSX, Bash/Shell, and Markdown**. The syntax layer is intentionally presentation-oriented: it emits tokens for highlighting and folding rather than building an AST.


## Documentation

What follows is the whole library, front to back: every widget, every layout primitive, and every application-level API, with runnable code next to each one rather than a bare function signature. It's organized so you can treat it like a reference, not a novel — expand only the sections you actually need, and skip the rest.

<a id="colors-styles--theming"></a>
<details>
<summary><strong>🎨 Colors, styles &amp; theming</strong></summary>

#### Colors

Create an RGB color:

``` go
red := color.RGB(220, 50, 47)
```

Use a named color:

``` go
blue := color.TokyoBlue
```
#### Styles: foreground, background and attributes

A `style.Style` contains:

``` go
type Style struct {
    Fg   color.Color
    Bg   color.Color
    Attr style.Attr
}
```

Create one:

``` go
s := style.New(color.White, color.Background)
```

Or directly:

``` go
s := style.Style{
    Fg: color.NordWhite,
    Bg: color.NordPanel,
}
```

Change foreground:

``` go
s = s.WithFg(color.NordCyan)
```

Change background:

``` go
s = s.WithBg(color.NordBackground)
```

Add attributes:

``` go
s = s.WithAttr(style.Bold)
```

Remove attributes:

``` go
s = s.WithoutAttr(style.Bold)
```

Supported terminal attributes:

``` go
style.Bold
style.Dim
style.Underline
style.Reverse
style.Blink
style.Italic
```
#### Built-in themes
Use a built-in theme:

``` go
theme := style.NordTheme()
```

Then:

``` go
app.New(root, theme).Run()
```

Other theme constructors:

``` go
style.TokyoNightTheme()
style.MatchaLatteTheme()
style.VaporwaveTheme()
style.MochaEspressoTheme()
style.DeepAbyssTheme()
style.NordTheme()
style.DraculaTheme()
style.CatppuccinMochaTheme()
style.RosePineTheme()
style.CyberpunkTheme()
style.AutumnTheme()
style.SynthwaveTheme()
style.SolarizedLightTheme()
```
#### Creating your own theme

Clone an existing theme:

``` go
theme := style.NordTheme().Clone()
```

Change selected color:

``` go
theme.Selected.Bg = color.RGB(70, 120, 190)
theme.Selected.Fg = color.White
```

Change panel:

``` go
theme.Panel.Bg = color.RGB(30, 34, 42)
```

Change title:

``` go
theme.Title.Fg = color.RGB(100, 200, 220)
theme.Title.Attr = style.Bold
```

Use it:

``` go
app.New(root, theme).Run()
```

#### ThemeOverride

Most standard visual widgets expose:

``` go
ThemeOverride *style.Theme
```

ZeroTUI lets a rendered component opt into its own `*style.Theme` through `ThemeOverride`. A component theme controls the visual palette for that component: text/foreground colours, backgrounds, borders, focus state, selection, semantic colours, titles and scrollbar roles. The application theme remains the default, so you only override the components that need a custom appearance.

`TextEditor` is the exception: it exposes `ThemeOverride *style.EditorTheme`, which embeds the normal `style.Theme` and adds editor/syntax roles. Use `style.NewEditorTheme(...)` or `style.ZedEditorTheme()` when customizing source-editor appearance.

```go
base := style.TokyoNightTheme()
buttonTheme := *base
buttonTheme.Positive = buttonTheme.Positive.WithFg(color.TokyoGreen)
buttonTheme.Selected = buttonTheme.Selected.WithBg(color.TokyoBlue)

button := widget.NewButton("RUN", run)
button.ThemeOverride = &buttonTheme
```
Component typography can be controlled through the theme's foreground/background and terminal attributes such as `Bold`, `Dim`, `Underline`, `Reverse` and `Blink`.


#### Runtime theme switching

```go
a.SetTheme(style.NordTheme())
```
For example:

``` go
a.OnKey = func(k input.Key) bool {
    if k.Type == input.KeyRune && k.Rune == 't' {
        a.SetTheme(style.DraculaTheme())
        return true
    }
    return false
}
```
This is a visual change only. Existing component `ThemeOverride` values continue to take precedence for components that intentionally use their own palette.

#### Widget background overrides

Many widgets expose:

``` go
Background *color.Color
```

`nil` means:

> inherit the surface already behind the widget.

Example:

``` go
bg := color.RGB(30, 35, 45)

label := widget.NewLabel("Custom surface")
label.Background = &bg
```

For a shared color:

``` go
panelBg := color.NordPanel

label.Background = &panelBg
button.Background = &panelBg
```

This is preferable to making every component a different random color.

#### Foreground overrides

Widgets that explicitly support foreground overrides can use:

``` go
Foreground *color.Color
```

For example:

``` go
badge := widget.NewBadge("LIVE")

fg := color.NordCyan
badge.Foreground = &fg
```

For components without a dedicated `Foreground` field, use a `style.Style` where supported, or use a component `ThemeOverride`.

</details>

<a id="layout-system"></a>
<details>
<summary><strong>📐 Layout system</strong></summary>

ZeroTUI layouts operate on `layout.Node` values. A widget becomes a layout node with:

```go
node := layout.Wrap(widget.NewLabel("Hello"))
```

Layouts decide where nodes are placed; widgets remain responsible for drawing and interaction.

#### Flex

`Flex` is the general-purpose layout for rows and columns.

```go
root := layout.NewFlex(
    layout.Vertical,
    layout.Fix(layout.Wrap(title), 1),
    layout.Flex1(layout.Wrap(body)),
)
```

Horizontal:

```go
row := layout.NewFlex(
    layout.Horizontal,
    layout.Flex1(layout.Wrap(left)),
    layout.Flex1(layout.Wrap(right)),
)
```

Use:

```go
layout.Fix(node, 10)     // fixed main-axis size
layout.Flex1(node)       // one flexible share
layout.FlexN(node, 2)    // weighted flexible share
```

A `Flex` also supports an optional cross-axis `Item.Width` and `Item.Height` constraint:

```go
item := layout.Flex1(layout.Wrap(input))
item.Width = 40
item.Height = 3
```

The default gap is one cell. Change it when needed:

```go
root.Gap = 0
root.Gap = 2
```

---

#### Grid

`Grid` divides the available area into equal row/column cells.

```go
grid := layout.NewGrid(
    2,
    3,
    layout.Wrap(a),
    layout.Wrap(b),
    layout.Wrap(c),
    layout.Wrap(d),
    layout.Wrap(e),
    layout.Wrap(f),
)
```

Use it for KPI cards, dashboards, and regular tile arrangements.

---

#### Split

`Split` creates two panes separated by a mouse-draggable divider.

```go
split := layout.NewSplit(
    layout.Horizontal,
    layout.Wrap(left),
    layout.Wrap(right),
    0.5,
)
```

The ratio is the initial share of the first pane.

```text
0.25 → first pane about 25%
0.50 → first pane about 50%
0.75 → first pane about 75%
```

Use `layout.Vertical` for a top/bottom split:

```go
split := layout.NewSplit(
    layout.Vertical,
    layout.Wrap(top),
    layout.Wrap(bottom),
    0.60,
)
```

Minimum pane sizes can be constrained:

```go
split.MinFirst = 20
split.MinSecond = 30
```

---

#### FixedSize

`FixedSize` gives a child an explicit width and/or height when space permits.

```go
box := layout.FixedSize(
    layout.Wrap(widget.NewLabel("Settings")),
    40,
    5,
)
```

Examples:

```go
layout.FixedSize(child, 40, 0) // width 40, available height
layout.FixedSize(child, 0, 5)  // available width, height 5
layout.FixedSize(child, 40, 5)
```

The result is clamped to the available terminal area and centered.

---

#### SizeBounds

`SizeBounds` applies minimum and maximum dimensions.

```go
box := layout.SizeBounds(
    layout.Wrap(widget.NewLabel("Responsive")),
    20, 60,
    3, 8,
)
```

A zero maximum means unlimited:

```go
layout.SizeBounds(child, 20, 0, 3, 0)
```

---

#### Padding

`Padding` reserves space around a child.

```go
content := layout.Padding(
    layout.Wrap(widget.NewLabel("Hello")),
    2, // left
    1, // top
    2, // right
    1, // bottom
)
```

Padding is useful when the containing layout should control spacing without adding another widget.

---

#### Stack

`Stack` places all children in the same area, with later children rendered above earlier ones.

```go
stack := layout.NewStack(
    layout.Wrap(background),
    layout.Wrap(content),
)
```

Use it for layered content, decorations, and custom background/foreground compositions.

---

#### Overlay

`Overlay` conditionally places a child over the current layout.

```go
overlay := layout.NewOverlay(
    func() bool {
        return modalVisible
    },
    layout.Wrap(modal),
)
```

Typical uses include popups, command palettes, and temporary dialogs.

---

#### Modal

`NewModal` is a centered overlay with a dimming backdrop.

```go
modal := layout.NewModal(
    func() bool {
        return modalVisible
    },
    layout.Wrap(dialog),
    60,
    15,
)
```

The width and height are the requested modal dimensions.

---

#### Center

`Center` places a child in a centered box.

```go
centered := layout.Center(
    layout.Wrap(widget.NewLabel("Centered")),
    40,
    5,
)
```

The requested area is centered when the parent has enough space.

---

#### Bordered

`Bordered` adds a titled border around a complete child layout.

```go
panel := layout.Bordered(
    "Controls",
    layout.Padding(form, 2, 1, 2, 1),
    func() bool {
        return textInput.IsFocused()
    },
)
```

The focus callback controls whether the focused border style is used.

---

#### BorderedRounded

`BorderedRounded` is the rounded-corner variant of `Bordered`.

```go
panel := layout.BorderedRounded(
    "Order Book",
    layout.Wrap(orderBook),
    func() bool {
        return orderBook.IsFocused()
    },
)
```

Use it when a dashboard benefits from rounded panel framing.

---

#### ClosableRounded

`ClosableRounded` combines a rounded border, a close button, and visibility state.

```go
panel := layout.ClosableRounded(
    "Order Book",
    layout.Wrap(orderBook),
    func() bool {
        return orderBook.IsFocused()
    },
    func() {
        // optional close callback
    },
)
```

Control it programmatically:

```go
panel.Close()
panel.Show()

if panel.Visible() {
    // visible
}
```

When a closable node becomes hidden, surrounding `Flex`/`Split` layouts can reclaim the available space.

---

#### Responsive

`Responsive` switches between two layout trees according to terminal width.

```go
root := layout.Responsive(
    100,
    compactLayout,
    expandedLayout,
)
```

The compact layout is used below the breakpoint; the expanded layout is used at or above it.

This is useful for laptop terminals, large monitors, SSH sessions, and narrow split-terminal windows.

---

#### Retained

`NewRetained` caches a stable layout subtree's flattened placements.

```go
retained := layout.NewRetained(root)
```

Invalidate it when the structure or geometry needs to be recomputed:

```go
retained.Invalidate()
```

Use retained layout when a large scene tree is structurally stable while individual widgets inside it update frequently.

---

#### FitHeight

`FitHeight` lets a child participate in layouts that honor its preferred height.

```go
node := layout.FitHeight(layout.Wrap(widget.NewLabel("Content")))
```

Use it when a component's natural/preferred height should influence the layout rather than forcing a fixed height.

---

#### Wrap

`Wrap` converts a `widget.Widget` into a `layout.Node`.

```go
node := layout.Wrap(widget.NewButton("RUN", run))
```

It is the normal bridge between the widget and layout packages.

</details>

<a id="widgets-reference"></a>
<details>
<summary><strong>🧩 Widgets reference</strong></summary>

The current ZeroTUI widget package is focused on reusable, terminal-native components. Widgets draw themselves into the retained buffer and, where applicable, handle keyboard/mouse input and report focused or dirty regions.

### Basic display widgets

#### Label

Use `Label` for static or application-owned text.

```go
label := widget.NewLabel("CPU: 42%")
label.SetText("CPU: 48%")
```

For a render-time value:

```go
label.TextFn = func() string {
    return currentText
}
```

A label can also use `Bold`, `Style`, and `Background` where appropriate.

---

#### Button

A keyboard- and mouse-activatable action.

```go
button := widget.NewButton("SAVE", func() {
    save()
})
```

Use `Danger = true` for destructive actions so the theme's negative semantic styling is used.

---

#### Toggle

A two-state control backed by a `uint32`.

```go
var enabled uint32

toggle := widget.NewToggle("Notifications", &enabled)
```

The widget updates the value when activated and participates in normal focus/mouse handling.

---

#### Slider

A bounded numeric control.

```go
var leverage uint32 = 10

slider := widget.NewSlider(
    "Leverage",
    &leverage,
    1, 50, 1,
    widget.FormatInt("x"),
)
```

The slider supports keyboard/mouse interaction and a configurable `SliderFormat`.

---

#### Gauge

A compact progress/utilization indicator.

```go
gauge := widget.NewGauge("CPU")
gauge.ValueFn = func() float64 {
    return cpuUsage()
}
```

Optional thresholds:

```go
gauge.WarnAt = 0.70
gauge.DangerAt = 0.90
```

---

#### Sparkline

A small time-series chart.

```go
spark := widget.NewSparkline(60)

for _, value := range samples {
    spark.Push(value)
}
```

Use it for latency, traffic, prices, utilization, and other compact trends.

---

#### Heatmap

A fixed-size grid of scalar values rendered as solid colored cells — a per-core load matrix, an error-rate grid bucketed by service and minute, or an order-book depth/correlation matrix.

```go
grid := widget.NewHeatmap(24, 8, 0, 100) // 24 cols x 8 rows, values normalized into [0,100]

grid.SetRow(0, latestCoreLoads) // len(latestCoreLoads) == 24
grid.Set(3, 2, 87.5)            // or update a single cell
```

`Set`/`SetRow` copy into preallocated storage under a short lock and are safe to call from a metrics/market-data goroutine while `Draw` runs on the render goroutine. Unlike `Sparkline`, the color domain is a fixed `[min, max]` set at construction (change it deliberately with `SetRange`) rather than auto-ranged, so a value's color stays stable across frames and a single `Set` call only invalidates its own row.

```go
grid.Low, grid.High = color.NordCyan, color.NordRed // cool -> hot; defaults to theme.Info -> theme.Negative
grid.CellWidth = 2                                   // terminal columns per grid cell (default 2, roughly square)
```

---

#### PriceTicker

A compact numeric price display intended for frequently changing market values.

```go
var price uint64 = 78900000000000

ticker := widget.NewPriceTicker(
    "BTC-PERP",
    &price,
    2,
    2,
)
```

The price is stored as an integer and formatted using the configured decimal/display precision.

---

#### Badge

A small semantic status label.

```go
badge := widget.NewBadge("LIVE")
badge.Positive = true
```

Useful for states such as LIVE, READY, ERROR, or ACTIVE.

---

#### Divider

A horizontal or vertical separator.

```go
horizontal := widget.NewDivider(true)
vertical := widget.NewDivider(false)
```

Use it to visually group sections without introducing another panel.

---

#### Stat

A compact label/value metric.

```go
stat := widget.NewStat("REVENUE", "$128.4K")
stat.Delta = "+14.8%"
stat.Up = true
```

Useful for dashboard KPI cards and summary rows.

---

#### Spinner

A small activity indicator.

```go
spinner := widget.NewSpinner("Loading")
spinner.Tick()
```

Advance it from the application's normal update loop rather than creating a goroutine per spinner.

---

#### GradientBar

A horizontal value/intensity bar with a configurable gradient.

```go
bar := widget.NewGradientBar(
    color.NordCyan,
    color.NordBlue,
    color.NordDimGray,
)
bar.Value = 0.65
```

Useful for capacity, utilization, confidence, or health indicators.

### Lists, tables and navigation

#### List

A normal in-memory list.

```go
list := widget.NewList([]string{
    "API gateway",
    "Database",
    "Worker queue",
})
```

Use `List` when the complete set of items is small enough to keep in the widget.

---

#### VirtualList

A viewport-oriented list for larger data sets.

```go
list := widget.NewVirtualList(
    100000,
    func(index int) string {
        return loadItem(index)
    },
)
```

Only the visible portion needs to be rendered. This is the preferred list primitive when the data set is large or frequently changing.

---

#### Table

A regular interactive table with application-owned rows and columns.

```go
table := widget.NewTable([]widget.Column{
    {Title: "Symbol"},
    {Title: "Price"},
    {Title: "Qty"},
})
```

The table supports selection, keyboard navigation, mouse interaction, column sizing/alignment, and row/cell presentation controls exposed by the current API.

---

#### VirtualTable

A viewport-oriented table for large row counts.

```go
table := widget.NewVirtualTable(
    []widget.Column{
        {Title: "Symbol"},
        {Title: "Price"},
        {Title: "Qty"},
    },
    100000,
    func(row, col int) string {
        return cellText(row, col)
    },
)
```

`VirtualTable` renders from callbacks instead of requiring the application to materialize every visible row as a separate object. It also tracks dirty regions and supports keyboard/mouse interaction.

---

#### Tabs

A horizontal tab selector.

```go
tabs := widget.NewTabs([]string{
    "Overview",
    "Orders",
    "Logs",
})
```

Tabs support keyboard and mouse selection and can be placed above the content layout that corresponds to the selected tab.

### Text and command widgets

#### TextInput

A single-line editable field.

```go
input := widget.NewTextInput("Search...")
input.SetValue("zerotui")
```

Read the current value with:

```go
value := input.String()
```

`TextInput` handles editing, cursor movement, paste, focus, and mouse interaction.

---

#### FastLogView

A bounded, high-throughput log view.

```go
logs := widget.NewFastLogView(4096)

logs.Append("connected")
logs.Append("market data ready")
```

Clear it when required:

```go
logs.Clear()
```

It is intended for continuously appended operational/log output where rebuilding a conventional list every update would be unnecessary overhead.

---

#### CommandPalette

A searchable command selector.

```go
palette := widget.NewCommandPalette([]widget.Command{
    {Key: "save", Label: "Save File"},
    {Key: "quit", Label: "Quit"},
})

palette.SetQuery("save")
```

Use it for command search, quick actions, and modal command interfaces.

### Scrolling, resizing and panel controls

#### ScrollBar

`ScrollBar` is a low-level visual scrollbar component.

```go
scroll := widget.ScrollBar{
    Total:    1000,
    Offset:   200,
    Viewport: 30,
}
```

It can be customized through its track, thumb, and background fields. `VirtualList` and `VirtualTable` provide scrolling behavior at the widget level, so applications normally do not need to build a scrollbar separately.

---

#### ResizeHandle

`Split` uses `ResizeHandle` for mouse-driven pane resizing.

```go
ratio := 0.5

handle := widget.NewResizeHandle(
    widget.ResizeVertical,
    &ratio,
)
```

The handle can constrain the ratio with `MinRatio` and `MaxRatio`.

For ordinary two-pane layouts, prefer `layout.NewSplit` rather than constructing the handle directly.

---

#### CloseButton

A small close control used by closable layouts.

```go
closeButton := widget.NewCloseButton(func() {
    // close panel
})
```

Most applications should use `layout.ClosableRounded`, which manages the close button and panel visibility together.

---

#### Terminal

A PTY-backed interactive terminal widget.

```go
terminal := widget.NewTerminal(".")
if err := terminal.Start(); err != nil {
    // handle startup error
}
```

Useful controls include:

```go
terminal.SetCWD("/tmp")
terminal.SetShell("/bin/bash")
terminal.SetMaxScrollback(10000)
terminal.RunCommand("go test ./...")
terminal.Stop()
terminal.Close()
```

`Terminal` handles keyboard input, paste, mouse-wheel scrolling, resize, PTY lifecycle, and terminal emulation.

---

### Editor widgets

#### TextEditor

A reusable plain-text editor with a document model.

```go
editor := widget.NewTextEditor()
editor.SetDocumentName("notes.txt")
editor.SetLanguage("text")
editor.SetText("hello\nworld")

editor.OnSave = func() bool {
    // persist editor.Document.Text()
    return true
}
```

It provides cursor movement, selections, scrolling, undo/redo, search/goto-line support, optional read-only mode, and an editor status bar.

Disable the status row when embedding it in a compact layout:

```go
editor.ShowStatusBar = false
```

---

#### CodeEditor

`CodeEditor` extends the text-editing surface with syntax highlighting and code folding.

```go
editor := widget.NewCodeEditor()
editor.Load("main.go", sourceBytes)
```

The host application owns filesystem I/O. Highlighting and folding providers can be replaced or disabled:

```go
editor.SetHighlighter(myHighlighter)
editor.SetFoldProvider(myFoldProvider)
```

Built-in syntax profiles include Go, Python, JSON, YAML/YML, TOML, Rust, C/C++, Java, JavaScript/JSX, Bash/Shell, and Markdown.

Large documents use viewport-oriented token caching so steady-state syntax work stays focused on visible source lines.

### Tree and file widgets

#### TreeView

A model-backed hierarchy widget.

```go
tree := widget.NewTreeView(model)

tree.OnActivate = func(node widget.TreeNode) {
    // activate node
}
```

The application supplies a `TreeModel`; `TreeView` handles selection, expansion/collapse, keyboard navigation, scrolling, and mouse interaction.

It does not perform filesystem I/O.

---

#### FilePicker

A text/code-oriented file browser rooted at an application-selected directory.

```go
picker := widget.NewFilePicker(".")
picker.OnSelect = func(path string) {
    // open path
}
picker.OnCancel = func() {
    // dismiss
}
picker.SetQuery("main")
```

It skips common dependency/metadata directories such as `.git`, `vendor`, and `node_modules` and is intended for selecting supported text/code/log files.

---

#### ShortcutHelpBar

A compact keyboard shortcut legend.

```go
help := widget.NewShortcutHelpBar(
    widget.Shortcut{Key: "Ctrl+S", Label: "Save"},
    widget.Shortcut{Key: "Ctrl+F", Label: "Search"},
)
```

The separator is configurable through `Separator`.

### Trading and operations widgets

These components are presentation-focused. The application owns the underlying market/order state and supplies updates through the widget APIs.

#### OrderBook

A high-frequency bid/ask depth display.

```go
book := widget.NewOrderBook(
    2, // price decimals
    2, // size decimals
    2, // displayed decimals
)
```

Order-book rendering is designed around localized updates rather than clearing and rebuilding the whole terminal region for every tick. Price cells, quantities, best bid/ask changes, and bar regions can therefore remain cheap under frequent updates.

---

#### TimeAndSales

A scrolling stream of executed trades.

```go
trades := widget.NewTimeAndSales(
    100, // capacity
    2,   // price decimals
    2,   // size decimals
    2,   // displayed decimals
)

trades.AddTrade(widget.Trade{
    Price: 10025,
    Size:  3,
    Buy:   true,
})
```

Use it beside an `OrderBook` for a compact market-data view.

---

#### PositionList

A position-oriented list for trading dashboards.

```go
positions := widget.NewPositionList()

positions.SetItems([]widget.PositionListItem{
    {
        Symbol: "BTC-PERP",
    },
})
```

The widget owns selection and presentation while the application remains responsible for the underlying trading state.

---

#### PositionPanel

A compact position-control panel with optional take-profit and stop-loss controls.

```go
panel := widget.NewPositionPanel(
    &tpEnabled,
    &slEnabled,
    &tpValue,
    &slValue,
)
```

Use `SetData` to update the displayed position information and `SetControl` to select the active control.

---

#### Orders

A compact order list for active/completed orders.

```go
orders := widget.NewOrders(100, 2, 2)

orders.SetRows([]widget.Order{
    {
        ID:     "42",
        Symbol: "BTC-PERP",
        Side:   "BUY",
        Price:  10000,
        Qty:    1,
        Status: "OPEN",
    },
})
```

The widget is designed for dashboard presentation; the application owns order lifecycle and synchronization.

---

#### PnL

A compact profit-and-loss display.

```go
pnl := widget.NewPnL(2, 2)
```

The constructor controls decimal/display precision. Feed the widget from the application's position and realized/unrealized PnL state.

---

#### LatencyMonitor

A compact latency statistics display.

```go
latency := widget.NewLatencyMonitor()
```

Its exported `Stats` contains atomic `Min`, `Max`, `Last`, and `Count` counters, making it suitable for high-frequency telemetry updates.

---

#### RiskMonitor

A compact risk-state display.

```go
risk := widget.NewRiskMonitor(2, 2)
```

Use it to present application-owned risk metrics without moving the risk model itself into the widget.

---

#### MarketStatus

A market/venue status indicator.

```go
status := widget.NewMarketStatus("CME", "BTC-PERP")
status.SetState(widget.MarketLive)
```

Use the widget to show whether a venue/instrument is live or in another supported market state.

---

#### OrderEntry

A compact order-entry control surface.

```go
entry := widget.NewOrderEntry()
entry.Side = "BUY"
```

It is intended to be embedded in a larger trading layout alongside market-data and position widgets.

### Supporting editor/data types

`Document` is the reusable text model used by the editor widgets:

```go
doc := widget.NewDocument("hello\nworld")
doc.SetName("notes.txt")
doc.SetLanguage("text")
doc.SetLine(0, "updated")
```

`Document`, `Column`, `TreeNode`, `TreeModel`, `Shortcut`, `Trade`, `Order`, and the other exported model/support types are APIs used by the widgets; they are not additional standalone widgets.

</details>


<a id="number-formatting-numfmt"></a>
<details>
<summary><strong>🔢 Number formatting (numfmt)</strong></summary>

Every trading widget in this library — `PriceTicker`, `OrderBook`, `Positions`, `Orders`, `PnL` — has the same problem every frame: turn a fixed-point integer into digits on screen without asking the Go allocator for anything. `fmt.Sprintf("%.2f", price)` allocates. `strconv.FormatFloat` allocates. Neither is acceptable on a hot render path ticking many times a second. `numfmt` is the small package those widgets use internally to avoid that entirely, and it is exported so your own custom widgets can use the same trick.

The whole package is five functions that append to a caller-owned `[]byte`, exactly the way `strconv.AppendInt` works — no formatting verbs, no reflection, no allocation once the scratch buffer is warm.

#### Plain integers

``` go
buf := make([]byte, 0, 32)
buf = numfmt.AppendUint(buf, 42)   // "42"
buf = buf[:0]
buf = numfmt.AppendInt(buf, -42)   // "-42"
```

#### Fixed-point values

Trading data is usually stored as an integer scaled by a power of ten (a `uint64` price of `7890012345` at 9 decimals *means* `7.890012345`) specifically so the render path never touches a `float64`. `AppendFixed` renders that scaled integer with its full decimal precision:

``` go
buf = buf[:0]
buf = numfmt.AppendFixed(buf, 7890012345, 9)
// buf == "7.890012345"
```

Most displays don't want all nine digits, though — a compact ticker might store prices at high internal precision but only show 2. `AppendFixedPrec` renders a chosen number of visible decimals instead of the full stored precision:

``` go
buf = buf[:0]
buf = numfmt.AppendFixedPrec(buf, 1234567, 2, 2)
// buf == "12345.67"  (stored at 2 decimals, shown at 2)
```

For signed fixed-point values (P&L, deltas, funding rates), use `AppendSignedFixedPrec`:

``` go
buf = buf[:0]
buf = numfmt.AppendSignedFixedPrec(buf, -1234567, 2, 2)
// buf == "-12345.67"
```

#### Padding a rendered number

`PadLeft` left-pads the digits it just appended, rather than taking a length up front. Record `len(dst)` before appending, then pad after:

``` go
buf = buf[:0]
from := len(buf)
buf = numfmt.AppendUint(buf, 42)
buf = numfmt.PadLeft(buf, from, 5, '0')
// buf == "00042"
```

This order — append, then pad from the recorded start — is what lets `PadLeft` work on any of the append functions above without knowing in advance how many digits a value will produce.

#### The pattern widgets actually use

The idiom every built-in widget follows is a small `[N]byte` scratch array reused across frames:

``` go
type Row struct {
    scratch [32]byte
}

func (r *Row) renderPrice(price uint64) []byte {
    buf := numfmt.AppendFixed(r.scratch[:0], price, 2)
    return buf
}
```

`r.scratch[:0]` reuses the backing array's capacity every call, so a widget redrawing thousands of times a second never asks the garbage collector for a new byte slice just to show a number.

</details>

<a id="application-focus--input"></a>
<details>
<summary><strong>⚙️ Application, focus &amp; input</strong></summary>

#### Focus

Interactive widgets implement the focus contract.

Examples:

-   Button
-   Toggle
-   Slider
-   List
-   VirtualList
-   Table
-   VirtualTable
-   Tabs
-   TextInput
-   CommandPalette

The application routes keyboard input to the focused widget.

A concrete focusable widget exposes:

``` go
btn.IsFocused()
```

and:

``` go
btn.Focus(true)
```

The application can explicitly focus a widget with:

``` go
a.Focus(btn)
```

where appropriate.

------------------------------------------------------------------------

#### Mouse interaction

Mouse-aware widgets implement:

``` go
HandleMouse(...)
```

ZeroTUI supports:

-   mouse clicks
-   wheel scrolling
-   dragging
-   resize handles
-   scrollbar dragging
-   button activation
-   table/list selection

You normally do not need to route mouse events manually.

`app.App` handles the routing.

------------------------------------------------------------------------

#### Global keyboard shortcuts

Use:

``` go
a.OnKey = func(k input.Key) bool {
  if k.Type == input.KeyRune {
      switch k.Rune {
      case 'q':
          // handled by QuitKeys normally
      case 'r':
          // reset
          return true
      }
  }
  return false
}
```

Returning `true` means:

> the event has been consumed.

Global handlers run before normal focus routing.

------------------------------------------------------------------------

#### Quit keys

Default:

``` go
q
```

You can configure:

``` go
a.QuitKeys = []rune{'q', 'x'}
```

Ctrl+C also causes the application to exit.

------------------------------------------------------------------------

#### Application target FPS

`App` exposes:

``` go
TargetFPS int
```

For live/interactive rendering:

``` go
a.TargetFPS = 60
```

or:

``` go
a.TargetFPS = 30
```

The scheduler is event-driven when idle.

This means a static application does not need to wake continuously just
to redraw an unchanged screen.

------------------------------------------------------------------------

#### Live rendering

For continuously changing UI:

``` go
a.RequestLive()
```

This tells the application that a live rendering source exists.

For a local animation region, use:

``` go
a.RequestLiveRect(rect)
```

and later:

``` go
a.DropLiveRect(rect)
```

This is preferable to making the entire screen live when only a small
widget changes.

------------------------------------------------------------------------

#### Invalidation

When data changes and the application needs a redraw:

``` go
a.Invalidate()
```

For a known rectangle:

``` go
a.InvalidateRect(rect)
```

For widgets:

``` go
a.InvalidateWidgets(
  priceTicker,
  orderBook,
)
```

Targeted invalidation is preferable when the change is localized.

------------------------------------------------------------------------

#### Batching updates

If several related values change together:

``` go
a.BeginBatch()

// update several widgets/data sources

a.InvalidateWidgets(price, book, status)

a.EndBatch()
```

This coalesces a burst of updates into one render wakeup.

A good real-time pattern is:

``` text
market update arrives
     ↓
update related state
     ↓
BeginBatch
     ↓
invalidate affected widgets
     ↓
EndBatch
     ↓
one render opportunity
```

------------------------------------------------------------------------

#### UI queue

Use `Queue` when a background worker needs to hand a short widget-state
mutation back to the application's UI/render goroutine:

``` go
ok := a.Queue(func() {
    status.SetText("market data refreshed")
    a.InvalidateWidgets(status)
})
```

The queue is bounded and non-blocking. `Queue` returns `false` when the queue
is full, so producers can coalesce or retry instead of blocking the rendering
path. Queued callbacks are not run concurrently with drawing or input
dispatch.

------------------------------------------------------------------------

#### Explicit relayout and interactive mode

Normal invalidation does not require a full layout pass. When layout structure
or geometry changes, use:

``` go
a.InvalidateLayout()
```

or explicitly:

``` go
a.Relayout()
```

For mouse-drag or other continuous interaction, the application can enter
interactive mode:

``` go
a.SetInteractive(true)
// update geometry while dragging
a.SetInteractive(false)
```

Interactive mode allows the scheduler to keep producing frames while the
interaction is active.

------------------------------------------------------------------------

#### Retained-state inspection

For diagnostics and performance instrumentation:

``` go
total, dirty := a.RetainedState()
```

This reports the current retained placement count and the number of retained
placements marked dirty.

------------------------------------------------------------------------

#### Resize and synchronized output

`OnResize` runs after the application rebuilds geometry:

``` go
a.OnResize = func(width, height int) {
    // update application-owned state
}
```

`SynchronizedOutput` defaults to `true` and wraps changed terminal frames in
DEC synchronized-output mode 2026. Disable it when a terminal compatibility
constraint requires ordinary output:

``` go
a.SynchronizedOutput = false
```

------------------------------------------------------------------------

#### Concurrent data updates

ZeroTUI deliberately uses different synchronization techniques depending
on the data.

Examples:

##### Toggle / Slider / PriceTicker

Use atomic scalar storage.

##### Sparkline / OrderBook

Use internal mutex-protected ring/snapshot state.

##### Your own data

You are responsible for synchronization.

Do not do:

``` go
var price float64

// goroutine A
price = 123.4

// renderer
fmt.Println(price)
```

without synchronization.

Use an atomic value, mutex, channel, or another safe ownership model.

</details>

## Rendering architecture

Every frame passes through four cooperating layers, and only the last one ever talks to the actual terminal:

```text
Layout          →   Widgets          →   Damage tracking     →   Renderer
placement           paint into a         finds the smallest      diffs vs. the front
math only           reusable back        dirty regions            buffer, writes only
                     buffer                                        the changed runs
```

1. **Layout** computes widget placement and can reuse retained placement storage instead of recomputing a tree of rectangles every frame.
2. **Widgets** paint cells into a reusable back buffer — the same backing array, frame after frame, not a fresh one each time.
3. **Damage tracking** identifies the smallest screen regions that actually need repainting, cell by cell.
4. **The renderer** diffs those cells against the front buffer and emits only the terminal escape sequences for runs that changed.

A normal live update therefore never needs to clear and redraw the entire terminal — most frames touch a handful of cells, and a handful of cells is all that gets written.

## Input and terminal behavior

The input parser keeps a fast ASCII path — the common case for keystrokes — while still supporting UTF-8 runes, SGR mouse events, and Kitty/progressive keyboard reports for terminals that offer them.

The renderer can optionally wrap changed frames in DEC synchronized-output mode 2026, which tells a supporting terminal to buffer a batch of writes and present them atomically instead of painting partial frames the eye can catch mid-update. It is enabled by default by `app.New` and can be disabled with the application's `SynchronizedOutput` field when terminal compatibility or application policy requires it.

## Performance

The benchmark suite is part of the project, not an afterthought. Run it on the machine that matters to you:

```bash
chmod +x benchmarks/run.sh
./benchmarks/run.sh
```

Or use the standard Go command directly:

```bash
go test ./benchmarks -run '^$' -bench . -benchmem -count=5
```

Most operations report **0 B/op and 0 allocations/op**, and a targeted widget update (~95 ns) is roughly **1,000× cheaper** than a full dashboard redraw (~92–93 µs) on the test machine below — which is why small changes can stay small in ZeroTUI.

<details open>
<summary><strong>Full benchmark results</strong></summary>

The following values are from the **latest in-repository compatibility run during this review** on **Linux `amd64`, Intel Xeon E312xx (Sandy Bridge, IBRS update)**.

The focused benchmark pass used a 200 ms measurement window and `-benchmem`. The values below are the **mean of those samples**, not the first sample.

| Operation | Latest review-run result |
| --- | ---: |
| `App.Invalidate` | **19.1 ns/op**, 0 B/op, 0 allocs/op |
| `App.InvalidateWidgets` | **28.6 ns/op**, 0 B/op, 0 allocs/op |
| Sparse retained buffer render (20 rows) | **~136.3 ns/op**, 0 B/op, 0 allocs/op |
| Sparse retained buffer render (500 rows) | **~127.7 ns/op**, 0 B/op, 0 allocs/op |
| Full buffer render (20 rows) | **~79.5 µs/op**, 0 B/op, 0 allocs/op |
| Full buffer render (500 rows) | **~79.6 µs/op**, 0 B/op, 0 allocs/op |
| Flex horizontal layout | **~402 ns/op**, 0 B/op, 0 allocs/op |
| Split layout | **~32.9 ns/op**, 0 B/op, 0 allocs/op |
| Grid layout | **~267 ns/op**, 0 B/op, 0 allocs/op |
| Responsive layout | **~60.7 ns/op**, 0 B/op, 0 allocs/op |
| High-frequency market-update scenario | **~24.2 µs/op**, 0 B/op, 0 allocs/op |
| OrderBook tick / 100 levels (single-level tick) | **~1.88 µs/op**, 0 B/op, 0 allocs/op |

The widget pass covered the shipped drawing and interaction benchmarks. Representative results (mean of 5 samples) include `PriceTicker` **~121.7 ns/op**, `Table` **~9.57 µs/op**, `VirtualTable` **~11.88 µs/op**, `OrderBook` **~19.11 µs/op**, and `FastLogView` **~7.93 µs/op**, all at **0 B/op and 0 allocs/op** in this run.

The editor-specific benchmarks measured (mean of 5 samples):

| Editor operation | Latest review-run result |
| --- | ---: |
| `TextEditor` draw / 500k-line source | **~49.2 µs/op**, 0 B/op, 0 allocs/op |
| `TextEditor` search / 500k-line source | **~572.7 ns/op**, 0 B/op, 0 allocs/op |
| `CodeEditor` viewport draw / 10k-line source | **~83.8 µs/op**, 0 B/op, 0 allocs/op |
| `CodeEditor` search / 10k-line source | **~105.4 ns/op**, 0 B/op, 0 allocs/op |
| `CodeEditor` edit + syntax update | **~2.27 ms/op**, 1.28 MB/op, 11 allocs/op |
| Complete `editor.Editor` draw / ~1,000 source lines | **~174.5 µs/op**, 16 B/op, 1 alloc/op |

### HFT sustained benchmark — latest supplied run — **cpu: Intel Xeon E312xx (Sandy Bridge, IBRS update)**

The sustained comparison uses a 120×45 pseudo-terminal, a 19-row visible viewport, 20- and 500-row datasets, a 1 kHz logical tick target, and a strict 60 FPS render ceiling. Updates are queued/batched between frames; realistic density changes one row per logical tick, while worst density changes every row.

All three are run inside the same `podman run --cpus=4 --memory=2g` container, through the same `script -qec` pty. Ratatui uses `CrosstermBackend`; ZeroTUI and Bubble Tea use their own pty writers. `GOMAXPROCS=4` for the two Go targets.

| Framework | Rows | Density | Ticks completed | Effective ticks/sec | Allocation rate | GC cycles | GC pause total |
|---|---:|---|---:|---:|---:|---:|---:|
| **ZeroTUI** | 20 | realistic | 28,252 | **973.5** | **35.5 KB/s** | **0** | **0.00 ms** |
| **ZeroTUI** | 20 | worst | 28,222 | **972.6** | **0.64 MB/s** | 5 | 2.60 ms |
| **ZeroTUI** | 500 | realistic | 28,433 | **979.9** | **35.3 KB/s** | **0** | **0.00 ms** |
| **ZeroTUI** | 500 | worst | 27,774 | **957.0** | **15.3 MB/s** | 36 | 9.30 ms |
| Bubble Tea v2 | 20 | realistic | 26,610 | 910.0 | 26.3 MB/s | 369 | 140.30 ms |
| Bubble Tea v2 | 20 | worst | 534,240 † | 18,214.4 † | 27.5 MB/s | 413 | 141.38 ms |
| Bubble Tea v2 | 500 | realistic | 26,313 | 900.2 | 25.5 MB/s | 120 | 167.68 ms |
| Bubble Tea v2 | 500 | worst | 13,127,500 † | 446,734.3 † | 52.7 MB/s | 121 | 215.23 ms |
| Ratatui | 20 | realistic | 29,012 | **~1,000** | 7.6 MB/s | N/A | N/A |
| Ratatui | 20 | worst | 29,010 | **~1,000** | 7.8 MB/s | N/A | N/A |
| Ratatui | 500 | realistic | 29,015 | **~1,000** | 7.5 MB/s | N/A | N/A |
| Ratatui | 500 | worst | 29,001 | **~1,000** | 14.3 MB/s | N/A | N/A |

**† Bubble Tea worst-density counter is a counting artifact, not a throughput measurement.** Bubble Tea's `applyTick` increments its tick counter once per *fixture application*, and in worst mode one logical tick applies `rows` fixture entries — so the counter over-counts logical ticks by a factor of `rows`. The underlying work Bubble Tea performs is one logical tick per queued token, the same as ZeroTUI and Ratatui; only the reporting differs. Correcting for the artifact:
- Bubble Tea 20 worst: 534,240 ÷ 20 = **26,712 logical ticks** → **~910 ticks/sec**
- Bubble Tea 500 worst: 13,127,500 ÷ 500 = **26,255 logical ticks** → **~894 ticks/sec**

These two rows are retained here only because they appear in the raw run data, not as valid ticks/sec figures.

**Allocation definition caveat.** The Go targets (ZeroTUI, Bubble Tea) report deltas in Go runtime `TotalAlloc` — bytes allocated on the Go heap. Ratatui reports a custom `CountingAllocator` that sums `layout.size()` on every `alloc` call. These are not the same definition and are not directly comparable; cross-framework allocation ratios in this table should be read as order-of-magnitude signals, not precise equivalents. **"No GC" is not "no allocation":** Ratatui allocates because its dashboard calls `format!` per tick, but Rust frees that memory deterministically via `drop()`, so no GC pauses appear. ZeroTUI is the only framework here that is *both* GC-free and essentially allocation-free in the hot render path.

### Layer-1 state mutation and frame rendering — **cpu: Intel Xeon E312xx (Sandy Bridge, IBRS update)**

The supplied headless cross-framework microbenchmarks report the following central measurements (mean of 5 runs for Go targets, mean of criterion samples for Ratatui):

| Framework | State mutation, 20 rows | Full frame, 20 rows | State mutation, 500 rows | Full frame, 500 rows |
|---|---:|---:|---:|---:|
| **ZeroTUI** | **~368 ns** | **~79.5 µs** | **~359 ns** | **~79.6 µs** |
| Bubble Tea v2 | ~362 ns | ~96.8 µs | ~352 ns | ~88.0 µs |
| Ratatui | ~233 ns | ~253.9 µs | ~237 ns | ~258.5 µs |

ZeroTUI does not win the isolated state-mutation test; Ratatui is faster there (233 ns vs 368 ns). ZeroTUI's advantage in this suite is concentrated in low-allocation rendering and retained/sparse update behavior.

**Corrected rendering claim**: ZeroTUI's full-frame render is **~18% faster than Bubble Tea at 20 rows** (96.8 ÷ 79.5 = 1.218) and **~10% faster at 500 rows** (88.0 ÷ 79.6 = 1.105).

### ZeroTUI sparse rendering and trading workloads

The supplied ZeroTUI microbenchmarks measured:

- sparse 60 FPS frame path: **~136.3 ns/op**, 0 B/op, 0 allocs/op at 20 rows; **~127.7 ns/op** at 500 rows;
- full 60 FPS frame path: **~79.5 µs/op** at 20 rows; **~79.6 µs/op** at 500 rows;
- OrderBook full update + draw: approximately **6.05 µs / 10 levels**, **13.85 µs / 25**, **27.28 µs / 50**, and **32.0 µs / 100**, all with **0 B/op and 0 allocs/op** in the supplied runs.

> **Note on OrderBook numbers.** The `BenchmarkOrderBookTick/100` in the main `benchmarks` package reports ~1.88 µs/op, while the HFT target's `BenchmarkZeroTUIOrderBookTick/100` reports ~32.0 µs/op. These measure different operations: the former is a single-level tick, the latter is a full 100-level update plus draw. Do not conflate them.

The current benchmark suite also keeps concurrent Sparkline and OrderBook paths separate from ordinary single-threaded measurements, so synchronization overhead is visible rather than hidden inside a generic widget benchmark.

### Editor benchmark results

An editor-specific benchmark set was added for viewport rendering, search, edit/highlight behavior, and the complete editor composition. A **local compatibility validation** on **Linux `amd64` / Intel Xeon E312xx (Sandy Bridge, IBRS update)** measured (mean of 5 samples):

| Editor operation | Result | Allocations |
| --- | ---: | ---: |
| `CodeEditor` viewport draw, 10,000-line source | **~83.8 µs/op** | **0 B/op, 0 allocs/op** |
| `CodeEditor` search, 10,000-line source | **~105.4 ns/op** | **0 B/op, 0 allocs/op** |
| `CodeEditor` edit + syntax update | **~2.27 ms/op** | 1.28 MB/op, 11 allocs/op |
| Complete `editor.Editor` draw, ~1,000 source lines | **~174.5 µs/op** | 16 B/op, 1 alloc/op |

For code-heavy workloads, the important distinction is between **steady-state viewport work** and **content-changing work**: repainting an already-highlighted viewport is allocation-free in the benchmark, while an edit can legitimately invalidate syntax/fold state and therefore does more work.

### Why retained rendering matters

A market-data producer can receive many updates while the terminal only needs to present the latest state at the next render opportunity. ZeroTUI can coalesce changes and emit only the final damaged cells instead of rebuilding every visible string for every update.

The same design helps the editor: changing one line does not inherently require rebuilding every visible line, and large-document syntax work can be scoped to the viewport. This is why the editor benchmarks distinguish zero-allocation viewport redraw/search from the more expensive edit-and-highlight path.

</details>

## License

[MIT License](LICENSE) © 2026 ZeroGCDev
