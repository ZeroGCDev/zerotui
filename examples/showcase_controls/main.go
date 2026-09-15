package main

import (
	"fmt"
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

// This example is intentionally calm: one surface, one accent, and semantic
// status colors. It demonstrates component-level styling without turning the
// screen into a palette sampler.
func main() {
	var enabled uint32
	var level uint32 = 68

	theme := style.TokyoNightTheme()

	// Component themes are immutable after setup. Draw only follows pointers,
	// so these overrides do not create garbage in the render loop.
	accentTheme := *theme
	inputTheme := *theme
	buttonTheme := *theme
	statusTheme := *theme

	title := widget.NewLabel("ZeroTUI  •  COMPONENT CUSTOMIZATION")
	title.ThemeOverride = &accentTheme

	toggle := widget.NewToggle("Notifications", &enabled)
	toggle.OnFlag = "ON"
	toggle.OffFlag = "OFF"
	toggle.ThemeOverride = &accentTheme

	slider := widget.NewSlider(
		"Alert level",
		&level,
		0, 100, 1,
		widget.FormatInt("%"),
	)
	slider.ThemeOverride = &accentTheme
	slider.TrackWidth = 24

	input := widget.NewTextInput("Symbol")
	input.Border = true
	input.ThemeOverride = &inputTheme
	input.SetValue("BTC-PERP")

	status := widget.NewLabel("Status: READY")
	status.ThemeOverride = &statusTheme
	result := widget.NewLabel("Last applied: —")
	result.ThemeOverride = &accentTheme

	apply := widget.NewButton("APPLY", func() {
		symbol := input.String()
		if symbol == "" {
			symbol = "(empty)"
		}
		result.SetText("Last applied: " + symbol)
		if atomic.LoadUint32(&enabled) == 1 {
			status.SetText("Status: ENABLED")
			return
		}
		status.SetText("Status: APPLIED")
	})
	apply.ThemeOverride = &buttonTheme

	section := widget.NewLabel("CONTROL SURFACE")
	section.ThemeOverride = &accentTheme

	form := layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(section), 1),
		layout.Fix(layout.Wrap(toggle), 1),
		layout.Fix(layout.Wrap(slider), 1),
		layout.Fix(layout.FixedSize(layout.Wrap(input), 52, 3), 3),
		layout.Fix(layout.Wrap(apply), 1),
		layout.Fix(layout.Wrap(result), 1),
		layout.Fix(layout.Wrap(status), 1),
	)
	// Seven compact controls fit cleanly inside the bordered surface. A zero
	// gap here is deliberate: the panel itself provides the visual grouping.
	form.Gap = 0

	panel := layout.BorderedRounded(
		"Controls",
		layout.Padding(form, 2, 1, 2, 1),
		func() bool {
			return toggle.IsFocused() || slider.IsFocused() || input.IsFocused() || apply.IsFocused()
		},
	)

	root := layout.FixedSize(panel, 72, 17)
	if err := app.New(root, theme).Run(); err != nil {
		fmt.Println(err)
	}
}
