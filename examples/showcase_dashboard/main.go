package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

const PriceScale uint64 = 1_000_000_000

func main() {
	theme := style.NordTheme()
	tokyoTheme := style.TokyoNightTheme()

	var lastPrice uint64 = 78_900 * PriceScale
	var simActive uint32 = 1
	var leverage uint32 = 10
	var updateInterval uint32 = 33

	// -------------------------------------------------------------------------
	// Panel 1: controls
	// -------------------------------------------------------------------------

	simToggle := widget.NewToggle("Sim Generator", &simActive)
	simToggle.OnFlag = "RUNNING"
	simToggle.OffFlag = "STOPPED"

	levSlider := widget.NewSlider(
		"Leverage",
		&leverage,
		1,
		50,
		1,
		widget.FormatInt("x"),
	)

	speedSlider := widget.NewSlider(
		"Interval",
		&updateInterval,
		10,
		200,
		10,
		widget.FormatInt("ms"),
	)

	symbolInput := widget.NewTextInput("Symbol Name: ")
	symbolInput.Border = true
	symbolInput.SetValue("")

	statusLabel := widget.NewLabel("Status: READY")
	resultLabel := widget.NewLabel("Last applied: —")

	actionButton := widget.NewButton("APPLY SIGNAL", func() {
		symbol := symbolInput.String()
		if symbol == "" {
			symbol = "(empty)"
		}
		resultLabel.SetText("Last applied: " + symbol)
		statusLabel.SetText("Status: SIGNAL QUEUED")
	})

	controlsBody := layout.Padding(layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(simToggle), 1),
		layout.Fix(layout.Wrap(levSlider), 1),
		layout.Fix(layout.Wrap(speedSlider), 1),
		layout.Fix(layout.Wrap(symbolInput), 3),
		layout.Fix(layout.Wrap(actionButton), 1),
		layout.Fix(layout.Wrap(resultLabel), 1),
		layout.Fix(layout.Wrap(statusLabel), 1),
	), 1, 0, 1, 0)

	controlsFocused := func() bool {
		return simToggle.IsFocused() ||
			levSlider.IsFocused() ||
			speedSlider.IsFocused() ||
			symbolInput.IsFocused() ||
			actionButton.IsFocused()
	}

	panel1 := layout.ClosableRounded(
		"1: CONTROLS & PARAMS",
		controlsBody,
		controlsFocused,
		nil,
	)

	// -------------------------------------------------------------------------
	// Panel 2: live market widgets
	// -------------------------------------------------------------------------

	ticker := widget.NewPriceTicker(
		"BTC-PERP",
		&lastPrice,
		9,
		2,
	)

	spark := widget.NewSparkline(200)
	// Seed the chart so the dashboard shows a useful sparkline immediately,
	// before the first live market tick arrives.
	for i := 0; i < 80; i++ {
		phase := float64(i % 16)
		spark.Push(78900 + phase*2.5)
	}

	book := widget.NewOrderBook(
		9, // price decimals
		3, // size decimals
		2, // displayed decimals
	)

	metricsBody := layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(ticker), 2),
		layout.Fix(layout.Wrap(spark), 3),
		layout.Flex1(layout.Wrap(book)),
	)

	panel2 := layout.ClosableRounded(
		"2: LIVE MARKET & ORDER BOOK",
		metricsBody,
		nil,
		nil,
	)

	// -------------------------------------------------------------------------
	// Panel 3: large scrollable table
	// -------------------------------------------------------------------------

	symbols := [...]string{
		"BTC-PERP",
		"ETH-PERP",
		"SOL-PERP",
		"AVAX-PERP",
		"LINK-PERP",
	}

	columns := []widget.Column{
		{Title: "INDEX", Width: 8},
		{Title: "SYMBOL", Width: 12},
		{Title: "SIM PRICE ($)", Width: 16, Align: widget.AlignRight},
		{Title: "VOLUME", Width: 12, Align: widget.AlignRight},
		{Title: "STATUS", Width: 10},
	}

	// Precompute the immutable demo rows once. VirtualTable will return these
	// existing strings from its callbacks, so scrolling/selection never formats
	// numbers or allocates strings on the render path.
	const rowCount = 1000
	data := make([][5]string, rowCount)
	for row := 0; row < rowCount; row++ {
		data[row][0] = "#" + strconv.Itoa(10000 + row + 1)[1:]
		data[row][1] = symbols[row%len(symbols)]
		data[row][2] = strconv.FormatFloat(100.0+float64((row*37)%2500), 'f', 2, 64)
		data[row][3] = strconv.Itoa((row + 1) * 89 % 15000)
		switch row % 3 {
		case 0:
			data[row][4] = "FILLED"
		case 1:
			data[row][4] = "PENDING"
		default:
			data[row][4] = "ACTIVE"
		}
	}

	// Keep semantic table colors tied to the active application theme. The
	// pointer changes only when [t] is pressed, so CellStyle remains allocation-free.
	activeTheme := theme
	dataTable := widget.NewVirtualTable(columns, rowCount, func(row, col int) string {
		if row < 0 || row >= rowCount || col < 0 || col >= 5 {
			return ""
		}
		return data[row][col]
	})
	dataTable.CellStyle = func(row, col int) *style.Style {
		if col != 4 || row < 0 || row >= rowCount {
			return nil
		}
		switch row % 3 {
		case 0:
			return &activeTheme.Positive
		case 1:
			return &activeTheme.Warning
		default:
			return &activeTheme.Info
		}
	}

	dataTable.ShowScrollBar = true
	dataTable.Zebra = true

	panel3 := layout.ClosableRounded(
		"3: SCROLLABLE DATA TABLE",
		layout.Wrap(dataTable),
		func() bool {
			return dataTable.IsFocused()
		},
		nil,
	)

	// -------------------------------------------------------------------------
	// Workspace
	// -------------------------------------------------------------------------

	topSplit := layout.NewSplit(
		layout.Horizontal,
		panel1,
		panel2,
		0.40,
	)

	mainWorkspace := layout.NewSplit(
		layout.Vertical,
		topSplit,
		panel3,
		0.55,
	)

	header := widget.NewLabel(
		"  ZeroTUI  • LIVE MARKET SIMULATOR",
	)

	footer := widget.NewLabel(
		"Click [x] of each panel to close  • Use Mouse to Scroll • " +
			" Press [1] Controls  [2] Market  [3] Table to Reopening  [t] Theme (Nord/Tokyo) •  Drag the dividers to resize each Panel  •  [q] Quit",
	)

	root := layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(header), 1),
		layout.Flex1(mainWorkspace),
		layout.Fix(layout.Wrap(footer), 1),
	)

	// -------------------------------------------------------------------------
	// Create the application BEFORE starting the producer.
	// -------------------------------------------------------------------------

	a := app.New(root, theme)
	a.QuitKeys = []rune{'q'}

	// -------------------------------------------------------------------------
	// Panel reopening
	// -------------------------------------------------------------------------

	a.OnKey = func(k input.Key) bool {
		if k.Type != input.KeyRune {
			return false
		}

		switch k.Rune {
		case '1':
			panel1.Show()
			a.Relayout()
			return true

		case '2':
			panel2.Show()
			a.Relayout()
			return true

		case '3':
			panel3.Show()
			a.Relayout()
			return true
		case 't':
			// Keep the table's semantic CellStyle in lock-step with the application theme.
			if activeTheme == theme {
				activeTheme = tokyoTheme
				a.SetTheme(tokyoTheme)
			} else {
				activeTheme = theme
				a.SetTheme(theme)
			}
			return true
		}

		return false
	}

	// -------------------------------------------------------------------------
	// Live market-data generator
	// -------------------------------------------------------------------------

	go func() {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		const levelCount = 8

		bids := make([]widget.Level, levelCount)
		asks := make([]widget.Level, levelCount)

		for {
			interval := time.Duration(
				atomic.LoadUint32(&updateInterval),
			) * time.Millisecond

			if interval < 10*time.Millisecond {
				interval = 10 * time.Millisecond
			}

			time.Sleep(interval)

			if atomic.LoadUint32(&simActive) == 0 {
				continue
			}

			// -------------------------------------------------------------
			// Price random walk
			// -------------------------------------------------------------

			current := atomic.LoadUint64(&lastPrice)

			// ±$0.50 per tick.
			delta := int64(rng.Intn(101)-50) * (int64(PriceScale) / 100)

			var next uint64

			if delta < 0 {
				down := uint64(-delta)

				if down >= current {
					next = 78_900 * PriceScale
				} else {
					next = current - down
				}
			} else {
				next = current + uint64(delta)
			}

			atomic.StoreUint64(&lastPrice, next)

			// -------------------------------------------------------------
			// Sparkline
			// -------------------------------------------------------------

			spark.Push(float64(next) / float64(PriceScale))

			// -------------------------------------------------------------
			// Order book
			// -------------------------------------------------------------

			mid := next

			for i := 0; i < levelCount; i++ {
				offset := uint64(i+1) * PriceScale / 10

				// Independent size movement for each side.
				bidSize := uint64(50+rng.Intn(500)) * 1000
				askSize := uint64(50+rng.Intn(500)) * 1000

				// Give both sides visibly different depth profiles.
				if i%3 == 0 {
					bidSize += uint64(150+rng.Intn(250)) * 1000
				}

				if i%2 == 0 {
					askSize += uint64(100+rng.Intn(300)) * 1000
				}

				var bidPrice uint64
				if offset >= mid {
					bidPrice = 1
				} else {
					bidPrice = mid - offset
				}

				asks[i] = widget.Level{
					Price: mid + offset,
					Size:  askSize,
				}

				bids[i] = widget.Level{
					Price: bidPrice,
					Size:  bidSize,
				}
			}

			book.SetLevels(bids, asks)

			// One logical market tick updates several widgets. Batch the
			// invalidations so the scheduler sees one wakeup instead of three.
			a.BeginBatch()
			a.InvalidateWidgets(
				ticker,
				spark,
				book,
			)
			a.EndBatch()
		}
	}()

	// -------------------------------------------------------------------------
	// Run
	// -------------------------------------------------------------------------

	if err := a.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
