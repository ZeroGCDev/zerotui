package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

const (
	heatWorkers = 6
	heatBuckets = 36
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

	// A rolling per-worker load matrix: one row per worker, one column per
	// 700ms tick over the last heatBuckets ticks. Cool-to-hot coloring makes
	// a hot worker jump out without reading any numbers.
	heat := widget.NewHeatmap(heatBuckets, heatWorkers, 0, 100)
	heat.Low, heat.High = color.NordCyan, color.NordRed
	heat.CellWidth = 2
	history := make([][]float64, heatWorkers)
	for w := range history {
		history[w] = make([]float64, heatBuckets)
		for b := range history[w] {
			history[w][b] = 30
		}
		heat.SetRow(w, history[w])
	}
	heatPanel := layout.BorderedRounded("WORKER LOAD", layout.Wrap(heat), nil)

	side := layout.BorderedRounded("SERVICE STATUS", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewBadge("PRODUCTION")), 1),
		layout.Fix(layout.Wrap(spinner), 2),
		layout.Fix(layout.Wrap(status), 3),
		layout.Fix(layout.Wrap(gauge), 2),
		layout.Flex1(layout.Wrap(widget.NewLabel("API      healthy\nWORKER   healthy\nDB       healthy\nQUEUE    63%"))),
	), nil)

	main := layout.BorderedRounded("LIVE LOGS", layout.Wrap(logs), func() bool { return logs.IsFocused() })
	body := layout.NewFlex(layout.Vertical,
		layout.Flex1(main),
		layout.Fix(heatPanel, heatWorkers+2), // +2 for the panel's own border
	)
	paletteOverlay := layout.NewOverlay(
		func() bool { return paletteVisible },
		layout.Center(layout.Wrap(palette), .62, .72),
	)
	stack := layout.NewStack(body, paletteOverlay)

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

			// Scroll the load matrix left by one tick and append a fresh
			// reading per worker, then republish only the changed rows.
			for w := range history {
				copy(history[w], history[w][1:])
				next := history[w][heatBuckets-2] + float64(rng.Intn(21)-10)
				if next < 5 {
					next = 5
				} else if next > 98 {
					next = 98
				}
				history[w][heatBuckets-1] = next
				heat.SetRow(w, history[w])
			}

			a.BeginBatch()
			a.InvalidateWidgets(logs, gauge, status, heat)
			a.EndBatch()
		}
	}()

	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
}
