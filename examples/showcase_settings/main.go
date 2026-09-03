package main

import (
	"fmt"
	"sync/atomic"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.VaporwaveTheme()
	var notifications uint32 = 1
	var volume uint32 = 72
	var refresh uint32 = 30

	toggle := widget.NewToggle("Desktop notifications", &notifications)
	toggle.OnFlag, toggle.OffFlag = "ON", "OFF"

	volumeSlider := widget.NewSlider("Volume", &volume, 0, 100, 1, widget.FormatInt("%"))
	refreshSlider := widget.NewSlider("Refresh", &refresh, 5, 120, 5, widget.FormatInt("s"))

	input := widget.NewTextInput("Workspace name")
	input.Border = true
	input.SetValue("production")

	save := widget.NewButton("SAVE CHANGES", func() {})
	reset := widget.NewButton("RESET", func() {
		input.SetValue("production")
		atomic.StoreUint32(&volume, 72)
		atomic.StoreUint32(&refresh, 30)
	})

	preview := widget.NewStat("PREVIEW", "production")
	preview.Delta = "READY"
	capacity := widget.NewGradientBar(color.Cyan, color.Magenta, color.DimGray)
	capacity.Value = .72

	form := layout.Padding(layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("WORKSPACE")), 1),
		layout.Fix(layout.Wrap(input), 3),
		layout.Fix(layout.Wrap(toggle), 1),
		layout.Fix(layout.Wrap(volumeSlider), 1),
		layout.Fix(layout.Wrap(refreshSlider), 1),
		layout.Fix(layout.Wrap(widget.NewDivider(true)), 1),
		layout.Fix(layout.Wrap(save), 1),
		layout.Fix(layout.Wrap(reset), 1),
	), 2, 1, 2, 1)

	previewBox := layout.BorderedRounded("PREVIEW", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewBadge("CUSTOM THEME")), 1),
		layout.Fix(layout.Wrap(preview), 3),
		layout.Fix(layout.Wrap(capacity), 1),
		layout.Flex1(layout.Wrap(widget.NewLabel("Everything here is a normal widget.\nThe layout controls spacing, size and grouping."))),
	), nil)

	root := layout.Center(layout.NewSplit(layout.Horizontal,
		layout.BorderedRounded("SETTINGS", form, func() bool {
			return toggle.IsFocused() || volumeSlider.IsFocused() || refreshSlider.IsFocused() || input.IsFocused() || save.IsFocused() || reset.IsFocused()
		}),
		previewBox,
		.52,
	), .86, .82)

	if err := app.New(root, theme).Run(); err != nil {
		fmt.Println(err)
	}
}
