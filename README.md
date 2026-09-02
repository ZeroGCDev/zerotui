# ZeroTUI

**A retained-mode, low-allocation terminal UI library for Go.**

ZeroTUI is designed for applications where terminal rendering is part of a real-time workload: market-data terminals, trading dashboards, operations consoles, telemetry viewers, risk monitors, and dense interactive tools.

The core idea is simple: update data frequently, repaint only what changed, and keep the terminal renderer out of the allocation-heavy path.

Many terminal applications spend far more work rebuilding text than the user can actually see changing. ZeroTUI takes a different approach: widgets draw into a retained cell buffer, the compositor tracks damage, and the terminal writer emits only changed cells.

## Table of contents

- [Features](#features)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Widgets & layout primitives](#widgets--layout-primitives)
- [Examples](#examples)
- [Documentation](#documentation)
  - [Colors, styles & theming](#colors-styles--theming)
  - [Layout system](#layout-system)
  - [Widgets reference](#widgets-reference)
  - [Application, focus & input](#application-focus--input)
- [Rendering architecture](#rendering-architecture)
- [Input and terminal behavior](#input-and-terminal-behavior)
- [Performance](#performance)
- [License](#license)

## Features

ZeroTUI is a particularly good fit when:

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
<summary><strong>Interactive example (toggle, button, dynamic label)</strong></summary>

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

</details>


## Widgets & layout primitives

| Widgets | Layout primitives |
|---|---|
| `Label` · `Button` · `Toggle` · `Slider` · `Gauge` · `Sparkline` · `Table` · `VirtualTable` · `List` · `VirtualList` · `Tabs` · `TextInput` · `Panel` · `PriceTicker` · `OrderBook` · `FastLogView` · `CommandPalette` · `Spinner` · `Stat` · `GradientBar` · `ResizeHandle` · `CloseButton` | `Flex` · `Grid` · `Split` · `Stack` · `Responsive` · `Center` · `Bordered` · `ClosablePanel` · `Retained` |

Interactive widgets implement the focus and mouse contracts used by `app.App`, so applications do not need to build a separate routing layer for every component.

## Examples

```bash
go run ./examples/dashboard
go run ./examples/controls
```

<img width="1000" height="563" alt="Image" src="https://github.com/user-attachments/assets/212c333f-dc5b-47ad-85de-74ee5fe0e603" />

- Click `x` on a panel to close it
- Use the mouse to scroll
- Press `1` / `2` / `3` to reopen Controls / Market / Table
- Drag the dividers to resize each panel
- Press `q` to quit

## Documentation

The reference below covers every widget, layout primitive, and application-level API in ZeroTUI. Expand only the sections you need.

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

Every standard visual widget supports:

``` go
ThemeOverride *style.Theme
```

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

#### Padding

Padding reserves space around a child.

``` go
content := layout.Padding(
    layout.Wrap(widget.NewLabel("Hello")),
    2, // left
    1, // top
    2, // right
    1, // bottom
)
```

#### Exact component dimensions

Layout controls widget dimensions. `Flex` already supports fixed main-axis sizes with `Fix(...)` and flexible sizes with `Flex1(...)`/`FlexN(...)`. It now also supports optional `Item.Width` and `Item.Height` cross-axis constraints. For an explicit width and height, use `layout.FixedSize(...)`:

```go
layout.Fix(layout.FixedSize(layout.Wrap(button), 32, 3), 3)
```
##### Fixed size
`FixedSize` clamps to the available terminal area and centers the component. It is a layout-time operation and therefore does not add render-loop allocations.
Use `FixedSize` when a component should have a specific width and/or height.

``` go
box := layout.FixedSize(
    layout.Wrap(widget.NewLabel("Settings")),
    40,
    5,
)
```
Examples:

``` go
layout.FixedSize(child, 40, 0) // width 40, available height
layout.FixedSize(child, 0, 5)  // available width, height 5
layout.FixedSize(child, 40, 5) // exact 40x5 when space permits
```
##### SizeBounds
Use `SizeBounds` when a component needs minimum and maximum dimensions.

``` go
box := layout.SizeBounds(
    layout.Wrap(widget.NewLabel("Responsive")),
    20, // min width
    60, // max width
    3,  // min height
    8,  // max height
)
```

A zero maximum means unlimited.

``` go
layout.SizeBounds(child, 20, 0, 3, 0)
```

means:

``` text
minimum width  = 20
maximum width  = unlimited
minimum height = 3
maximum height = unlimited
```
##### Flex
`Flex` is the most useful general-purpose layout.

Create a vertical layout:

``` go
root := layout.NewFlex(
    layout.Vertical,
    layout.Fix(layout.Wrap(title), 1),
    layout.Flex1(layout.Wrap(body)),
)
```

Create a horizontal layout:

``` go
row := layout.NewFlex(
    layout.Horizontal,
    layout.Flex1(layout.Wrap(left)),
    layout.Flex1(layout.Wrap(right)),
)
```
##### Fixed Flex items
``` go
layout.Fix(node, 10)
```

The `10` is the size on the Flex main axis.

Horizontal:

``` text
┌──────────┬─────────────────────────┐
│ fixed 10 │ flexible                │
└──────────┴─────────────────────────┘
```

Vertical:

``` text
┌─────────────────────────────┐
│ fixed 3                     │
├─────────────────────────────┤
│ flexible                    │
│                             │
└─────────────────────────────┘
```
##### Flexible Flex items
One equal share:

``` go
layout.Flex1(node)
```

Weighted share:

``` go
layout.FlexN(node, 2)
```

Example:

``` go
row := layout.NewFlex(
    layout.Horizontal,
    layout.FlexN(layout.Wrap(left), 1),
    layout.FlexN(layout.Wrap(middle), 2),
    layout.FlexN(layout.Wrap(right), 1),
)
```

The remaining width is divided approximately:

``` text
left    = 25%
middle  = 50%
right   = 25%
```
##### Flex gaps
`NewFlex` defaults to a 1-cell gap.

``` go
form := layout.NewFlex(
    layout.Vertical,
    layout.Fix(layout.Wrap(a), 1),
    layout.Fix(layout.Wrap(b), 1),
)
```

Disable the gap:

``` go
form.Gap = 0
```

Use a larger gap:

``` go
form.Gap = 2
```

For compact terminal forms, `Gap = 0` is often useful when the containing panel already provides grouping.
##### Width and height overrides inside Flex
A `layout.Item` can also have:

``` go
Width
Height
```

For example:

``` go
item := layout.Flex1(layout.Wrap(input))
item.Width = 40
item.Height = 3
```

This is useful when a flexible child should remain centered at a particular cross-axis size.
##### Grid layout
`Grid` creates equal-sized row/column cells.

``` go
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

This produces:

``` text
┌──────┬──────┬──────┐
│  A   │  B   │  C   │
├──────┼──────┼──────┤
│  D   │  E   │  F   │
└──────┴──────┴──────┘
```

Use Grid for:

-   KPI cards
-   dashboards
-   control panels
-   fixed tile layouts
##### Split layout
`Split` creates two resizable panes.

``` go
split := layout.NewSplit(
    layout.Horizontal,
    layout.Wrap(left),
    layout.Wrap(right),
    0.5,
)
```

The final argument is the initial ratio for the first pane.

``` text
0.25 → first pane ~25%
0.50 → first pane ~50%
0.75 → first pane ~75%
```

The divider is mouse draggable.
##### Vertical split
Use:

``` go
layout.Vertical
```

for a top/bottom split:

``` go
split := layout.NewSplit(
    layout.Vertical,
    layout.Wrap(top),
    layout.Wrap(bottom),
    0.60,
)
```

##### Split minimum sizes
A `Split` exposes:

``` go
MinFirst
MinSecond
```

Example:

``` go
split.MinFirst = 20
split.MinSecond = 30
```

This prevents either pane becoming unusably narrow.

The default minimums are already conservative.
##### Stack
`Stack` places children on top of one another in the same area.

``` go
stack := layout.NewStack(
    layout.Wrap(background),
    layout.Wrap(content),
)
```

Later children are rendered above earlier children.

Use Stack for:

-   overlays
-   layered indicators
-   custom decorations
-   background + foreground compositions
##### Overlay
An `Overlay` displays a child conditionally.

``` go
overlay := layout.NewOverlay(
    func() bool {
        return modalVisible
    },
    layout.Wrap(modal),
)
```

Use it for:

-   popups
-   command palettes
-   temporary dialogs
-   contextual UI

##### Modal overlay
`NewModal` is a convenient centered overlay:

``` go
modal := layout.NewModal(
    func() bool { return modalVisible },
    layout.Wrap(dialog),
    60,
    15,
)
```

The child is displayed at the requested size with a dimming backdrop.
##### Centering
Use:

``` go
centered := layout.Center(
    layout.Wrap(widget.NewLabel("Centered")),
    40,
    5,
)
```

This creates a centered 40x5 area when the parent has enough space.
##### Bordered layout
`Bordered` is the layout-level way to put a titled border around a whole
child layout.

``` go
panel := layout.Bordered(
    "Controls",
    layout.Padding(form, 2, 1, 2, 1),
    func() bool {
        return input.IsFocused()
    },
)
```

The third argument tells the border whether it should use the focused
border style.

This is ideal when the contents are multiple widgets:

``` text
┌─ Controls ─────────────────────┐
│                                │
│  Toggle                        │
│  Slider                        │
│  Input                         │
│  [ APPLY ]                     │
│                                │
└────────────────────────────────┘
```
##### Rounded borders
Use:

``` go
layout.BorderedRounded(...)
```

when you want rounded corners.

Similarly:

``` go
layout.ClosableRounded(...)
```

creates a rounded closable panel.
##### Closable panels
A closable panel is useful for dashboards.

``` go
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

Close from code:

``` go
panel.Close()
```

Show again:

``` go
panel.Show()
```

Check state:

``` go
if panel.Visible() {
    // visible
}
```

When a panel closes, Flex/Split layouts can reclaim its space.

##### Responsive layouts
Use `Responsive` to choose between compact and expanded layouts.

``` go
root := layout.Responsive(
    100,
    compactLayout,
    expandedLayout,
)
```

Meaning:

``` text
terminal width < 100
    → compactLayout

terminal width >= 100
    → expandedLayout
```

This is useful for applications that should behave differently on:

-   laptop terminals
-   large monitors
-   SSH sessions
-   split terminal windows

##### Retained layout
For a large stable dashboard, use:

``` go
retained := layout.NewRetained(root)
```

A retained subtree caches its flattened placements until:

-   its geometry changes, or
-   it is explicitly invalidated.

Invalidate it when its layout structure changes:

``` go
retained.Invalidate()
```

Use retained layout for large stable scene trees where one hot widget changes frequently but the surrounding layout does not.

</details>

<a id="widgets-reference"></a>
<details>
<summary><strong>🧩 Widgets reference</strong></summary>

#### Label

The simplest text widget:

``` go
label := widget.NewLabel("Hello")
```

Change it:

``` go
label.SetText("New text")
```

Style it:

``` go
label.Bold = true
```

Custom style:

``` go
s := style.Style{
  Fg: color.NordCyan,
  Bg: color.NordPanel,
  Attr: style.Bold,
}

label.Style = &s
```

Custom background:

``` go
bg := color.NordBackground
label.Background = &bg
```

------------------------------------------------------------------------

#### Label with dynamic text

`TextFn` can provide text during rendering:

``` go
label.TextFn = func() string {
  return currentText
}
```

For high-frequency/concurrent data, the callback should read data from
your own safe state.

Do not casually read ordinary mutable strings from another goroutine.

For a simple UI-owned value, `SetText` is usually clearer.

------------------------------------------------------------------------

#### Button

Create:

``` go
button := widget.NewButton(
  "SAVE",
  func() {
      // action
  },
)
```

The callback runs when the button is activated by keyboard or mouse.

Danger button:

``` go
button.Danger = true
```

This uses the theme's negative semantic role.

------------------------------------------------------------------------

#### Button background

``` go
bg := color.NordPanel
button.Background = &bg
```

For a custom button appearance, prefer a component theme:

``` go
buttonTheme := style.NordTheme().Clone()
buttonTheme.Positive.Fg = color.NordCyan
button.ThemeOverride = buttonTheme
```

------------------------------------------------------------------------

#### Toggle

Create a toggle using an atomic `uint32`:

``` go
var enabled uint32

toggle := widget.NewToggle(
  "Notifications",
  &enabled,
)
```

The value convention is:

``` text
0 = off
1 = on
```

Customize displayed flags:

``` go
toggle.OnFlag = "ON"
toggle.OffFlag = "OFF"
```

Read it safely:

``` go
if atomic.LoadUint32(&enabled) == 1 {
  // enabled
}
```

The widget uses atomic operations internally.

This makes Toggle useful when a simple on/off state may also be read by
another goroutine.

------------------------------------------------------------------------

#### Slider

Create a slider:

``` go
var level uint32 = 50

slider := widget.NewSlider(
  "Alert",
  &level,
  0,
  100,
  1,
  widget.FormatInt("%"),
)
```

Arguments:

``` text
label
value pointer
minimum
maximum
step
formatter
```

The value is stored in an atomic `uint32`.

Keyboard and mouse interaction are supported.

------------------------------------------------------------------------

#### Slider formatting

Integer format:

``` go
widget.FormatInt("x")
```

Example:

``` text
10x
20x
30x
```

Basis-point percentage:

``` go
widget.FormatBasisPointsPct('+')
```

This is intended for values such as:

``` text
+2.00%
+5.50%
```

The formatter uses a caller-owned scratch buffer so the steady-state
render path can remain allocation-free.

------------------------------------------------------------------------

#### Slider width

``` go
slider.TrackWidth = 24
```

Use this when you want a consistent control width.

------------------------------------------------------------------------

#### Gauge

Gauge is a non-interactive progress/utilization bar.

``` go
gauge := widget.NewGauge("CPU")
gauge.Value = 0.42
```

`Value` is expected in:

``` text
0.0 → 1.0
```

Example:

``` text
CPU █████████░░░░░ 42%
```

------------------------------------------------------------------------

#### Gauge thresholds

``` go
gauge.WarnAt = 0.70
gauge.DangerAt = 0.90
```

The gauge automatically switches semantic colors:

``` text
< 70%  → positive
>= 70% → warning
>= 90% → negative
```

You can also supply a custom style:

``` go
s := style.Style{
  Fg: color.NordCyan,
  Bg: color.NordPanel,
}

gauge.Style = &s
```

------------------------------------------------------------------------

#### Gauge with concurrent data

If another goroutine updates the measurement, use your own atomic
representation and `ValueFn`.

Conceptually:

``` go
var bits atomic.Uint64

gauge.ValueFn = func() float64 {
  return math.Float64frombits(bits.Load())
}
```

Then a producer can write:

``` go
bits.Store(math.Float64bits(value))
```

This avoids a data race.

------------------------------------------------------------------------

#### Sparkline

Create:

``` go
spark := widget.NewSparkline(120)
```

Push values:

``` go
spark.Push(price)
```

The sparkline is a fixed-capacity ring buffer.

`Push` is O(1).

It is safe to push from another goroutine while the widget is rendering
because Sparkline uses a mutex around its ring-buffer state.

------------------------------------------------------------------------

#### Sparkline colors

By default it uses:

``` text
up   → theme.Positive
down → theme.Negative
```

Override them:

``` go
up := style.Style{
  Fg: color.NordGreen,
  Bg: color.NordPanel,
}

down := style.Style{
  Fg: color.NordRed,
  Bg: color.NordPanel,
}

spark.UpStyle = &up
spark.DownStyle = &down
```

------------------------------------------------------------------------

#### PriceTicker

PriceTicker is designed for atomic fixed-point prices.

Suppose:

``` text
price = 12345
decimals = 2
```

means:

``` text
123.45
```

Create:

``` go
var price uint64 = 12345

ticker := widget.NewPriceTicker(
  "BTC-PERP",
  &price,
  2,
  2,
)
```

The price pointer is read atomically.

The widget detects price direction and uses:

``` text
Positive
Negative
```

styles when the value changes.

------------------------------------------------------------------------

#### OrderBook

Create:

``` go
book := widget.NewOrderBook(
  2, // price decimals
  2, // size decimals
  2, // displayed decimals
)
```

Populate:

``` go
bids := []widget.Level{
  {Price: 10000, Size: 500},
  {Price: 9998, Size: 250},
}

asks := []widget.Level{
  {Price: 10002, Size: 400},
  {Price: 10004, Size: 700},
}

book.SetLevels(bids, asks)
```

`Price` and `Size` are fixed-point integers.

The exact scale is determined by:

``` text
Decimals
SizeDecimals
```

This avoids floating-point formatting in the hot render path.

------------------------------------------------------------------------

#### Why OrderBook is optimized

OrderBook is intended for frequently changing market data.

`SetLevels`:

-   reuses backing storage where possible,
-   compares old/new data,
-   records dirty ranges,
-   recalculates side maxima,
-   avoids per-update snapshot allocations in the steady state.

The renderer then repaints the necessary regions rather than rebuilding
the entire dashboard.

------------------------------------------------------------------------

#### Table

For a normal in-memory table:

``` go
columns := []widget.Column{
  {Title: "SYMBOL", Width: 12},
  {Title: "PRICE", Width: 12, Align: widget.AlignRight},
  {Title: "STATUS", Width: 12},
}

table := widget.NewTable(columns)

table.Rows = [][]string{
  {"BTC-PERP", "78900.00", "ACTIVE"},
  {"ETH-PERP", "4200.00", "PENDING"},
}
```

Wrap it:

``` go
root := layout.Wrap(table)
```

------------------------------------------------------------------------

#### Table column alignment

Columns support:

``` go
widget.AlignLeft
widget.AlignCenter
widget.AlignRight
```

For prices and quantities:

``` go
{Title: "PRICE", Width: 12, Align: widget.AlignRight}
```

For symbols:

``` go
{Title: "SYMBOL", Width: 12, Align: widget.AlignLeft}
```

------------------------------------------------------------------------

#### Flexible table columns

A column with:

``` go
Width: 0
```

is flexible.

Use `Weight`:

``` go
columns := []widget.Column{
  {Title: "SYMBOL", Width: 12},
  {Title: "DESCRIPTION", Weight: 2},
  {Title: "STATUS", Weight: 1},
}
```

Fixed columns are allocated first; flexible columns share remaining
space by weight.

For terminal dashboards, avoid using flexible weights for every column
if the result creates huge empty spaces.

A common good design is:

``` text
fixed identity columns
+
fixed numeric columns
+
one flexible description column
```

------------------------------------------------------------------------

#### Table selection

Tables support:

``` go
table.Selected
```

The selected row is visually highlighted when the table has focus.

Customize selection foreground:

``` go
fg := color.NordWhite
table.SelectionForeground = &fg
```

Customize selection background:

``` go
bg := color.NordBlue
table.SelectionBackground = &bg
```

Selection is applied across the complete row.

------------------------------------------------------------------------

#### Zebra tables

Enable:

``` go
table.Zebra = true
```

This gives alternating row surfaces for dense data.

Use zebra styling carefully. A subtle difference is usually easier to
read than strong alternating colors.

------------------------------------------------------------------------

#### Per-row styling

Use:

``` go
table.RowStyle = func(row int) *style.Style {
  if row == 3 {
      s := style.Style{
          Fg: color.NordGreen,
          Bg: color.NordPanel,
      }
      return &s
  }
  return nil
}
```

For high-frequency tables, avoid constructing a new `style.Style` every
callback.

Prefer prebuilt styles:

``` go
positive := style.Style{
  Fg: color.NordGreen,
  Bg: color.NordPanel,
}

table.RowStyle = func(row int) *style.Style {
  if row == 3 {
      return &positive
  }
  return nil
}
```

------------------------------------------------------------------------

#### Per-cell styling

Use:

``` go
table.CellStyle = func(row, col int) *style.Style {
  if col == 2 {
      return &positive
  }
  return nil
}
```

This is useful for:

-   P&L
-   status
-   risk
-   warnings
-   semantic values

Selection styling is applied after cell styling so selected rows remain
visually continuous.

------------------------------------------------------------------------

#### VirtualTable

Use `VirtualTable` for large datasets.

``` go
table := widget.NewVirtualTable(
  columns,
  1_000_000,
  func(row, col int) string {
      return getCell(row, col)
  },
)
```

The table does **not** render one million rows.

It only asks for cells that intersect the visible viewport/damage
region.

This is the right widget for:

-   order flow
-   large transaction history
-   log-like data
-   millions of records
-   database viewers
-   telemetry streams

------------------------------------------------------------------------

#### VirtualTable callbacks

The callback:

``` go
Cell func(row, col int) string
```

should ideally return an existing string.

For maximum performance, avoid doing expensive work inside it.

Bad:

``` go
Cell: func(row, col int) string {
  return fmt.Sprintf("%.8f", databaseValue(row))
}
```

Better:

``` text
format data when it changes
or
use an allocation-free formatter
or
keep already formatted strings
```

The renderer should stay simple.

------------------------------------------------------------------------

#### VirtualTable RowStyle and CellStyle

Same APIs as `Table`:

``` go
table.RowStyle = ...
table.CellStyle = ...
```

Only painted/visible cells are requested during clipped drawing.

This is important when the underlying table has hundreds of thousands or
millions of rows.

------------------------------------------------------------------------

#### VirtualTable scrollbar

Enable:

``` go
table.ShowScrollBar = true
```

Customize:

``` go
track := color.NordDimGray
thumb := color.NordCyan

table.ScrollTrack = &track
table.ScrollThumb = &thumb
```

Selection colors:

``` go
table.SelectionForeground = &fg
table.SelectionBackground = &bg
```

The selected scrollbar cell retains the scrollbar thumb glyph so the
thumb stays visually continuous.

------------------------------------------------------------------------

#### VirtualList

For a large one-column list:

``` go
list := widget.NewVirtualList(
  1_000_000,
  func(index int) string {
      return itemAt(index)
  },
)
```

This is preferable to building:

``` go
[]string
```

for a huge dataset when only a small viewport is visible.

------------------------------------------------------------------------

#### VirtualList selection

``` go
list.Selected = 42
```

Handle selection:

``` go
list.OnSelect = func(index int) {
  // selected index
}
```

Customize:

``` go
list.ShowScrollBar = true
list.SelectionBackground = &selectionBg
list.SelectionForeground = &selectionFg
```

------------------------------------------------------------------------

#### Normal List

Use `List` when the complete item collection is reasonably small:

``` go
items := []string{
  "Market",
  "Orders",
  "Positions",
  "Risk",
}

list := widget.NewList(items)
```

Keyboard:

``` text
Up / Down
j / k
Enter
```

Mouse selection is also supported.

------------------------------------------------------------------------

#### Tabs

Create:

``` go
tabs := widget.NewTabs([]string{
  "POSITIONS",
  "ORDERS",
  "RISK",
})
```

Set the active tab:

``` go
tabs.Active = 1
```

React to changes:

``` go
tabs.OnChange = func(index int) {
  // switch content
}
```

Tabs draw the tab strip; your application decides what content belongs
to the active tab.

------------------------------------------------------------------------

#### TextInput

Create:

``` go
input := widget.NewTextInput("Symbol")
```

Enable a border:

``` go
input.Border = true
```

Set an initial value:

``` go
input.SetValue("BTC-PERP")
```

Read it:

``` go
value := input.String()
```

Submit callback:

``` go
input.OnSubmit = func(value string) {
  // use submitted text
}
```

------------------------------------------------------------------------

#### Numeric TextInput

Set:

``` go
input.Numeric = true
```

This restricts input to digits and `.`.

Useful for:

-   quantities
-   prices
-   percentages
-   numeric parameters

------------------------------------------------------------------------

#### TextInput styling

Background:

``` go
bg := color.NordBackground
input.Background = &bg
```

Theme:

``` go
input.ThemeOverride = style.NordTheme()
```

For forms, it is often best to use the panel surface as the normal
background and a stronger border/focus style.

------------------------------------------------------------------------

#### Panel widget

There are two ways to create a panel.

##### Widget-level Panel

``` go
panel := widget.NewPanel(
  "Status",
  label,
)
```

A `widget.Panel` contains one `Widget`.

##### Layout-level Bordered

For multiple children, prefer:

``` go
panel := layout.Bordered(
  "Status",
  formLayout,
  func() bool {
      return input.IsFocused()
  },
)
```

This distinction is useful:

``` text
widget.Panel
  → one child widget

layout.Bordered
  → arbitrary layout tree
```

------------------------------------------------------------------------

#### Panel styling

``` go
panel.Background = &bg
panel.Rounded = true
panel.Focused = true
```

Or use:

``` go
panel.ThemeOverride = customTheme
```

For complex panels, layout-level `Bordered` is generally more flexible.

------------------------------------------------------------------------

#### FastLogView

Use `FastLogView` for a high-volume log display.

``` go
log := widget.NewFastLogView(10_000)
```

Append:

``` go
log.Append("connected to exchange")
log.Append("received market update")
```

Follow tail:

``` go
log.FollowTail = true
```

The widget stores logs in a bounded ring.

This avoids unbounded memory growth.

------------------------------------------------------------------------

#### CommandPalette

Create:

``` go
palette := widget.NewCommandPalette([]widget.Command{
  {
      Name: "Open Orders",
      Key:  "O",
      Execute: func() {
          // action
      },
  },
  {
      Name: "Quit",
      Key:  "Q",
      Execute: func() {
          // action
      },
  },
})
```

Update query:

``` go
palette.SetQuery("ord")
```

It performs subsequence-style fuzzy matching.

Use it as an overlay/modal when you want a command launcher.

------------------------------------------------------------------------

#### Badge

Create:

``` go
badge := widget.NewBadge("LIVE")
```

Semantic mode:

``` go
badge.Positive = true
```

or:

``` go
badge.Negative = true
```

or:

``` go
badge.Info = true
```

Explicit colors:

``` go
fg := color.NordCyan
bg := color.NordPanel

badge.Foreground = &fg
badge.Background = &bg
```

------------------------------------------------------------------------

#### Divider

Create a horizontal divider:

``` go
divider := widget.NewDivider(true)
```

Vertical:

``` go
divider := widget.NewDivider(false)
```

Use Divider instead of creating one-off strings of box-drawing
characters when you want a reusable structural separator.

------------------------------------------------------------------------

#### Stat

Create a KPI:

``` go
stat := widget.NewStat(
  "Latency",
  "1.2ms",
)
```

Optional delta:

``` go
stat.Delta = "-0.3ms"
stat.Down = true
```

Or:

``` go
stat.Delta = "+12%"
stat.Up = true
```

Use Stat for compact dashboard metrics.

------------------------------------------------------------------------

#### Spinner

Create:

``` go
spinner := widget.NewSpinner("Loading")
```

Advance it:

``` go
spinner.Tick()
```

A spinner is useful for small local progress indications.

Do not create a new goroutine for every spinner. A single application
update mechanism is usually better.

------------------------------------------------------------------------

#### GradientBar

Create:

``` go
bar := widget.NewGradientBar(
  color.NordCyan,
  color.NordBlue,
  color.NordDimGray,
)
```

Set:

``` go
bar.Value = 0.65
```

It is useful for:

-   utilization
-   intensity
-   health
-   confidence
-   capacity

------------------------------------------------------------------------

#### ScrollBar

`ScrollBar` is a lower-level visual component.

``` go
scroll := widget.ScrollBar{
  Total:    1000,
  Offset:   200,
  Viewport: 30,
}
```

Customize:

``` go
scroll.Track = &track
scroll.Thumb = &thumb
scroll.Background = &bg
```

It is primarily useful when implementing custom scrolling components.

Most users should use the scrollbar built into:

-   `VirtualList`
-   `VirtualTable`

------------------------------------------------------------------------

#### ResizeHandle

`ResizeHandle` is normally created by `Split`.

You usually do not need to construct it manually.

If you do:

``` go
ratio := 0.5

handle := widget.NewResizeHandle(
  widget.ResizeVertical,
  &ratio,
)
```

You can control:

``` go
handle.MinRatio = 0.20
handle.MaxRatio = 0.80
```

It is pointer-oriented rather than keyboard-focus oriented.

------------------------------------------------------------------------

#### CloseButton

`CloseButton` is also normally managed by `layout.ClosableRounded`.

Create directly:

``` go
close := widget.NewCloseButton(func() {
  // close
})
```

It is intentionally not part of the keyboard focus ring.

This prevents decorative panel controls from increasing focus
complexity.

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

You can inspect:

``` go
input.IsFocused()
```

and explicitly focus:

``` go
a.Focus(input)
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

ZeroTUI uses four cooperating layers:

1. **Layout** computes widget placement and can reuse retained placement storage.
2. **Widgets** paint cells into a reusable back buffer.
3. **Damage tracking** identifies the smallest screen regions that need repainting.
4. **The renderer** diffs those cells against the front buffer and emits changed terminal runs.

A normal live update therefore does not need to clear and redraw the entire terminal.

## Input and terminal behavior

The input parser keeps a fast ASCII path while supporting UTF-8 runes, SGR mouse events, and Kitty/progressive keyboard reports.

The renderer can optionally wrap changed frames in DEC synchronized-output mode 2026. It is enabled by default by `app.New` and can be disabled with the application's `SynchronizedOutput` field when terminal compatibility or application policy requires it.

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

</details>

## License

ZeroTUI is released under the GNU General Public License v3.0. See [`LICENSE`](LICENSE).
