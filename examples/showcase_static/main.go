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
