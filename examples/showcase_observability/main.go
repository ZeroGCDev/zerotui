package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.NordTheme()

	logs := widget.NewFastLogView(1000)
	logs.FollowTail = true
	for i := 1; i <= 18; i++ {
		logs.Append(fmt.Sprintf("14:2%d:%02d  worker=%02d  request completed  status=200", i%6, i*3%60, i))
	}

	spinner := widget.NewSpinner("Deploying")
	spinner.Frame = 3
	gauge := widget.NewGauge("Queue depth")
	gauge.Value = .63

	var paletteVisible bool
	palette := widget.NewCommandPalette([]widget.Command{
		{Name: "Restart service", Key: "r"},
		{Name: "Open metrics", Key: "m"},
		{Name: "Clear logs", Key: "c"},
		{Name: "Deploy latest", Key: "d"},
	})

	status := widget.NewStat("DEPLOYMENT", "RUNNING")
	status.Delta = "2m 14s"

	side := layout.BorderedRounded("SERVICE STATUS", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewBadge("PRODUCTION")), 1),
		layout.Fix(layout.Wrap(spinner), 2),
		layout.Fix(layout.Wrap(status), 3),
		layout.Fix(layout.Wrap(gauge), 2),
		layout.Flex1(layout.Wrap(widget.NewLabel("API      healthy\nWORKER   healthy\nDB       healthy\nQUEUE    63%"))),
	), nil)

	main := layout.BorderedRounded("LIVE LOGS", layout.Wrap(logs), func() bool { return logs.IsFocused() })
	paletteOverlay := layout.NewOverlay(
		func() bool { return paletteVisible },
		layout.Center(layout.Wrap(palette), .62, .72),
	)
	stack := layout.NewStack(main, paletteOverlay)

	root := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("  OBSERVABILITY • CONTROL ROOM")), 1),
		layout.Flex1(layout.NewSplit(layout.Horizontal, side, stack, .25)),
		layout.Fix(layout.Wrap(widget.NewLabel("  Press [p] to focus command palette • [q] quit")), 1),
	)

	var a *app.App
	palette.OnClose = func() {
		paletteVisible = false
		a.Relayout()
	}
	a = app.New(root, theme)
	a.QuitKeys = []rune{'q'}
	a.OnKey = func(k input.Key) bool {
		if k.Type == input.KeyRune && k.Rune == 'p' {
			paletteVisible = true
			palette.SetQuery("")
			a.Relayout()
			a.Focus(palette)
			return true
		}
		return false
	}

	go func() {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; ; i++ {
			time.Sleep(700 * time.Millisecond)
			queue := 35 + rng.Intn(55)
			gauge.Value = float64(queue) / 100
			status.Delta = fmt.Sprintf("%ds", i+1)
			msg := []string{"request completed", "cache refreshed", "worker heartbeat", "metrics batch flushed", "health check passed"}[rng.Intn(5)]
			code := 200
			if rng.Intn(20) == 0 {
				code = 503
			}
			logs.Append(fmt.Sprintf("live event %04d  %s  status=%d  queue=%02d%%", i, msg, code, queue))
			a.BeginBatch()
			a.InvalidateWidgets(logs, gauge, status)
			a.EndBatch()
		}
	}()

	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
}
