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
