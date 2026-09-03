package main

import (
	"fmt"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func main() {
	theme := style.TokyoNightTheme()

	// Tabs are wired to the task list so this example demonstrates a real
	// tabbed view rather than a tab strip that only changes its highlight.
	tabs := widget.NewTabs([]string{"Inbox", "Today", "Upcoming", "Done"})
	tabs.Active = 1
	tabItems := [][]string{
		{"Design landing page", "Review pull request #842", "Write release notes", "Fix mobile navigation", "Update API documentation"},
		{"Design landing page", "Review pull request #842", "Write release notes", "Fix mobile navigation", "Update API documentation", "Prepare demo recording", "Clean up old feature flags"},
		{"Prepare Q4 roadmap", "Schedule customer interviews", "Plan architecture review", "Draft launch checklist", "Book release demo"},
		{"Archive old feature flags", "Close completed pull requests", "Publish release notes", "Clean up project boards"},
	}

	virtual := widget.NewVirtualList(200, func(i int) string { return fmt.Sprintf("Generated task %03d", i+1) })
	// Start in the middle of the large queue so the example demonstrates
	// virtualization and selection without filling the screen from item 001.
	virtual.Selected = 148
	virtual.ShowScrollBar = true

	tasks := widget.NewList(tabItems[tabs.Active])
	tasks.Selected = 0
	tabs.OnChange = func(index int) {
		if index < 0 || index >= len(tabItems) {
			return
		}
		tasks.Items = tabItems[index]
		tasks.Selected = 0
	}

	schedule := layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("TODAY  •  WEDNESDAY")), 1),
		layout.Fix(layout.Wrap(widget.NewLabel("09:30    Focus session")), 1),
		layout.Fix(layout.Wrap(widget.NewLabel("11:00    Team review")), 1),
		layout.Fix(layout.Wrap(widget.NewLabel("14:00    Release planning")), 1),
		layout.Fix(layout.Wrap(widget.NewLabel("16:30    Demo recording")), 1),
		layout.Fix(layout.Wrap(widget.NewLabel("NEXT UP")), 1),
		layout.Fix(layout.Wrap(widget.NewLabel("Prepare release notes")), 1),
	)

	columns := []widget.Column{
		{Title: "TASK", Weight: 3},
		{Title: "OWNER", Weight: 1},
		{Title: "STATUS", Weight: 1},
	}
	table := widget.NewTable(columns)
	table.Rows = [][]string{
		{"Landing page", "Maya", "IN REVIEW"},
		{"API docs", "Arun", "DONE"},
		{"Navigation", "Lena", "BLOCKED"},
		{"Release notes", "Maya", "DRAFT"},
	}

	left := layout.BorderedRounded("TASKS", layout.NewFlex(
		layout.Vertical,
		layout.Fix(layout.Wrap(tabs), 2),
		layout.Fix(layout.Wrap(tasks), 7),
		layout.Fix(layout.Wrap(widget.NewLabel("LARGE QUEUE")), 1),
		layout.Fix(layout.Wrap(virtual), 5),
	), func() bool { return tasks.IsFocused() })

	center := layout.BorderedRounded("SCHEDULE", layout.Padding(schedule, 2, 1, 2, 1), nil)
	right := layout.BorderedRounded("TEAM BOARD", layout.Wrap(table), func() bool { return table.IsFocused() })

	workspace := layout.NewGrid(1, 3, left, center, right)
	footer := widget.NewLabel("  ↑↓ select  •  click tabs  •  mouse wheel scroll  •  [q] quit")

	root := layout.Center(layout.NewFlex(layout.Vertical,
		layout.Fix(layout.Wrap(widget.NewLabel("  WORKSPACE / TODAY")), 1),
		layout.Flex1(workspace),
		layout.Fix(layout.Wrap(footer), 1),
	), .96, .92)

	if err := app.New(root, theme).Run(); err != nil {
		fmt.Println(err)
	}
}
