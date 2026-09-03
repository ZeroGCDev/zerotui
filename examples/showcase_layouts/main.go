package main

import (
	"fmt"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.DeepAbyssTheme()

	card := func(title, body string) layout.Node {
		label := widget.NewLabel(body)
		label.Background = &color.AbyssPanel
		return layout.BorderedRounded(title, layout.Padding(layout.Wrap(label), 2, 1, 2, 1), nil)
	}

	panelWidget := widget.NewPanel("WIDGET PANEL", widget.NewLabel("Panel is a widget too.\nUse it when you want a titled\nchild surface without building\na larger layout tree."))

	overview := layout.NewGrid(3, 2,
		card("OVERVIEW", "CPU   42%\nRAM   68%\nDISK  31%"),
		card("NETWORK", "RX   18.2 MB/s\nTX    7.4 MB/s\nRTT      3 ms"),
		card("SERVICES", "gateway   ●\nworker    ●\ndatabase  ●"),
		card("RELEASE", "v2.7.0\nstable\n14 minutes ago"),
		layout.Wrap(panelWidget),
	)

	compact := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("COMPACT VIEW")), 1),
		layout.Flex1(overview),
	)

	expanded := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("EXPANDED VIEW")), 1),
		layout.Flex1(layout.NewSplit(layout.Horizontal,
			overview,
			card("DETAIL", "A layout can be composed\nfrom other layouts.\n\nFlex + Grid + Split +\nPadding + Bordered + Center."),
			.70,
		)),
	)

	responsive := layout.Responsive(100, compact, expanded)

	root := layout.NewStack(
		responsive,
		layout.Center(layout.Wrap(widget.NewLabel("ZERO TUI  •  COMPOSITION SHOWCASE")), .55, .08),
	)

	// Retained demonstrates that a composed subtree can be cached and explicitly invalidated.
	retained := layout.NewRetained(root)
	retained.Invalidate()

	final := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("  LAYOUT LAB")), 1),
		layout.Flex1(retained),
		layout.Fix(layout.Wrap(widget.NewLabel("  Resize the terminal to see Responsive() switch views • [q] quit")), 1),
	)

	if err := app.New(final, theme).Run(); err != nil {
		fmt.Println(err)
	}
}
