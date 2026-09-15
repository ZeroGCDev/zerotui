// Package editor provides a small terminal editor surface:
// a collapsible file tree, a resizable split, and a syntax-highlighted,
// editable code/text viewer with tabs, selection, folding, search and saving.
// It deliberately uses only the Go standard library for filesystem access and syntax scanning.
package editor

/*
 * editor.Editor
     │
     └── CodeViewer
           │
           └── widget.CodeEditor
                 │
                 └── widget.TextEditor
*/

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"strings"
	"unicode/utf8"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

const maxFileSize = 64 << 20

// Editor composes FileTree and CodeViewer into one resizable editor widget.
type EditorTab struct {
	Path   string
	Viewer *CodeViewer
}

type Editor struct {
	widget.FocusMixin

	ThemeOverride *style.EditorTheme

	// OnThemeChange lets an embedding application keep its outer chrome in sync
	// when the built-in settings picker changes the editor palette.
	OnThemeChange  func(*style.EditorTheme)
	themeOptions   []style.EditorThemeOption
	themePreviews  []*style.EditorTheme
	themeIndex     int
	settingsScroll int

	// Terminal instances are disposable because they own PTY sessions.
	Terminal     *widget.Terminal
	TerminalOpen bool

	TerminalRatio float64
	terminalMinH  int
	terminalDrag  bool
	terminalDragY int

	OnInvalidate func()

	// OnExit is called when the user presses the Exit button in the
	// activity rail. The application owns the actual shutdown operation.
	OnExit func()

	Tree   *FileTree
	Viewer *CodeViewer

	Tabs           []*EditorTab
	TabBar         *widget.Tabs
	ActiveTab      int
	tabScroll      int
	Ratio          float64
	MinTree        int
	MinCode        int
	focusPane      int
	dragging       bool
	dragStartX     int
	dragStartRatio float64
	area           geometry.Rect

	// 0 = Explorer
	// 1 = Search
	// 2 = Terminal
	// 3 = Settings

	activity int

	OnLoad  func(path string)
	OnClose func(path string)

	closePrompt    bool
	closePromptTab int
}

func New(root string) *Editor {
	e := &Editor{
		Tree:      NewFileTree(root),
		Viewer:    NewCodeViewer(),
		ActiveTab: -1,

		Ratio:   .27,
		MinTree: 18,
		MinCode: 30,

		// Terminal is created lazily.
		Terminal:     nil,
		TerminalOpen: false,

		TerminalRatio: .22,
		terminalMinH:  7,

		TabBar:         widget.NewTabs(nil),
		themeOptions:   style.BuiltInEditorThemes(),
		themeIndex:     0,
		settingsScroll: 0,
	}

	e.themePreviews = make([]*style.EditorTheme, len(e.themeOptions))
	for i, option := range e.themeOptions {
		if option.Factory != nil {
			e.themePreviews[i] = option.Factory()
		}
	}

	e.TabBar.ShowClose = true

	e.TabBar.OnChange = func(i int) {
		e.activateTab(i)
	}

	e.TabBar.OnClose = func(i int) bool {
		return e.CloseTab(i)
	}

	e.Tree.OnOpen = func(path string) {
		e.LoadFile(path)
	}

	e.Tree.OnRename = func(oldPath, newPath string) {
		e.handleTreeRename(oldPath, newPath)
	}

	e.Tree.OnDelete = func(path string) {
		e.handleTreeDelete(path)
	}

	if n := e.Tree.firstFile(); n != nil {
		e.LoadFile(n.path)
	}

	return e
}

// newTerminal creates a completely fresh terminal instance.
//
// This is deliberately the only place where Editor creates a terminal.
// Every terminal gets the same callbacks. When its shell exits, the Editor
// drops the pointer so the instance cannot be reused.
func (e *Editor) newTerminal() *widget.Terminal {
	root := ""

	if e.Tree != nil {
		root = e.Tree.Root
	}

	t := widget.NewTerminal(root)

	t.OnChange = func() {
		if e.OnInvalidate != nil {
			e.OnInvalidate()
		}
	}

	t.OnExit(func(_ error) {
		// Ignore callbacks from old terminal instances.
		if e.Terminal != t {
			return
		}

		e.Terminal = nil
		e.TerminalOpen = false
		e.terminalDrag = false
		e.activity = 0

		if e.focusPane == 2 {
			e.setFocusPane(1)
		}

		if e.OnInvalidate != nil {
			e.OnInvalidate()
		}
	})

	return t
}

func (e *Editor) Focus(f bool) {
	e.FocusMixin.Focus(f)

	if e.Tree != nil {
		e.Tree.Focus(f && e.focusPane == 0)
	}

	if e.Viewer != nil {
		e.Viewer.Focus(f && e.focusPane == 1)
	}

	if e.Terminal != nil {
		e.Terminal.Focus(f && e.focusPane == 2)
	}

	if f {
		widget.ResetCursorBlink()
	}
}

func (e *Editor) IsFocused() bool {
	return e.FocusMixin.IsFocused()
}

func (e *Editor) SetTheme(theme *style.EditorTheme) {
	if theme == nil {
		return
	}

	e.ThemeOverride = theme
	if theme.Name != "" {
		for i, option := range e.themeOptions {
			if strings.EqualFold(option.Name, theme.Name) {
				e.themeIndex = i
				break
			}
		}
	}

	for _, tab := range e.Tabs {
		if tab != nil && tab.Viewer != nil {
			tab.Viewer.ThemeOverride = theme
		}
	}

	if e.Viewer != nil {
		e.Viewer.ThemeOverride = theme
	}

	if e.OnThemeChange != nil {
		e.OnThemeChange(theme)
	}

	if e.OnInvalidate != nil {
		e.OnInvalidate()
	}
}

// SelectTheme applies one of the themes exposed by the IDE settings panel.
// The selected factory creates a fresh palette, preventing accidental sharing
// of mutable theme state between editor instances.
func (e *Editor) SelectTheme(index int) bool {
	if index < 0 || index >= len(e.themeOptions) {
		return false
	}
	theme := e.themeOptions[index].Factory()
	if theme == nil {
		return false
	}
	theme.Name = e.themeOptions[index].Name
	e.themeIndex = index
	e.SetTheme(theme)
	visible := e.settingsVisibleRows()
	e.settingsScroll = e.clampThemeScroll(e.settingsScroll, visible)
	return true
}

func (e *Editor) settingsVisibleRows() int {
	mainH := e.area.H
	if mainH > 0 {
		termH := e.terminalHeight(e.area)
		if termH > 0 {
			mainH -= termH + 1
		}
	}
	visible := mainH - 6
	if visible < 1 {
		visible = 1
	}
	return visible
}

func (e *Editor) clampThemeScroll(scroll, visible int) int {
	if visible < 1 {
		visible = 1
	}
	maxScroll := len(e.themeOptions) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if e.themeIndex < scroll {
		scroll = e.themeIndex
	}
	if e.themeIndex >= scroll+visible {
		scroll = e.themeIndex - visible + 1
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

func (e *Editor) moveThemeSelection(delta int, visible int) bool {
	if len(e.themeOptions) == 0 {
		return false
	}
	next := e.themeIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(e.themeOptions) {
		next = len(e.themeOptions) - 1
	}
	e.settingsScroll = e.clampThemeScroll(e.settingsScroll, visible)
	if next == e.themeIndex {
		return true
	}
	e.themeIndex = next
	e.settingsScroll = e.clampThemeScroll(e.settingsScroll, visible)
	return e.SelectTheme(next)
}

func (e *Editor) setFocusPane(pane int) {
	if pane < 0 || pane > 2 {
		pane = 1
	}

	e.focusPane = pane

	if !e.IsFocused() {
		return
	}

	if e.Tree != nil {
		e.Tree.Focus(pane == 0)
	}

	if e.Viewer != nil {
		e.Viewer.Focus(pane == 1)
	}

	if e.Terminal != nil {
		e.Terminal.Focus(pane == 2)
	}

	widget.ResetCursorBlink()
}

// LoadFile opens path in an editor tab.
func (e *Editor) LoadFile(path string) bool {
	path, err := filepath.Abs(path)

	if err != nil {
		return false
	}

	for i, tab := range e.Tabs {
		if samePath(tab.Path, path) {
			e.activateTab(i)
			return true
		}
	}

	info, err := os.Stat(path)

	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Size() > maxFileSize {
		return false
	}

	data, err := os.ReadFile(path)

	if err != nil ||
		len(data) > maxFileSize ||
		!isTextData(data) ||
		!isTextFile(path) {
		return false
	}

	v := NewCodeViewer()
	v.ThemeOverride = e.ThemeOverride
	v.Load(path, data)

	e.Tabs = append(
		e.Tabs,
		&EditorTab{
			Path:   path,
			Viewer: v,
		},
	)

	e.syncTabBar()
	e.activateTab(len(e.Tabs) - 1)

	return true
}

func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)

	return filepath.Clean(a) == filepath.Clean(b)
}

func (e *Editor) activateTab(i int) bool {
	if i < 0 || i >= len(e.Tabs) {
		return false
	}

	e.ActiveTab = i

	if e.TabBar != nil {
		e.TabBar.Active = i
	}

	e.Viewer = e.Tabs[i].Viewer

	e.setFocusPane(1)

	if e.OnLoad != nil {
		e.OnLoad(e.Tabs[i].Path)
	}

	e.ensureActiveTabVisible()

	return true
}

func (e *Editor) CloseTab(i int) bool {
	if i < 0 || i >= len(e.Tabs) {
		return false
	}

	if e.Tabs[i].Viewer.Modified {
		e.closePrompt = true
		e.closePromptTab = i

		if e.OnInvalidate != nil {
			e.OnInvalidate()
		}

		return true
	}

	return e.removeTab(i)
}

func (e *Editor) removeTab(i int) bool {
	path := e.Tabs[i].Path
	wasActive := i == e.ActiveTab
	oldActive := e.ActiveTab

	e.Tabs = append(
		e.Tabs[:i],
		e.Tabs[i+1:]...,
	)

	e.syncTabBar()

	if len(e.Tabs) == 0 {
		e.ActiveTab = -1
		e.Viewer = NewCodeViewer()

		if e.OnClose != nil {
			e.OnClose(path)
		}

		return true
	}

	if !wasActive {
		if i < oldActive {
			oldActive--
		}

		e.ActiveTab = -1
		e.activateTab(oldActive)
	} else {
		target := i - 1

		if target < 0 {
			target = 0
		}

		e.ActiveTab = -1
		e.activateTab(target)
	}

	if e.OnClose != nil {
		e.OnClose(path)
	}

	return true
}

func (e *Editor) handleTreeRename(oldPath, newPath string) {
	oldPath, _ = filepath.Abs(oldPath)
	newPath, _ = filepath.Abs(newPath)

	for i, tab := range e.Tabs {
		path, _ := filepath.Abs(tab.Path)

		if samePath(path, oldPath) {
			tab.Path = newPath
			tab.Viewer.Path = newPath

		} else if rel, err := filepath.Rel(oldPath, path); err == nil &&
			rel != "." &&
			rel != ".." &&
			!strings.HasPrefix(
				rel,
				".."+string(os.PathSeparator),
			) {

			updated := filepath.Join(newPath, rel)
			tab.Path = updated
			tab.Viewer.Path = updated
		}

		if e.ActiveTab == i {
			e.Viewer = tab.Viewer
		}
	}
}

func (e *Editor) handleTreeDelete(path string) {
	for i := len(e.Tabs) - 1; i >= 0; i-- {
		if samePath(e.Tabs[i].Path, path) {
			if e.Tabs[i].Viewer.Modified {
				continue
			}

			e.removeTab(i)
		}
	}
}

func (e *Editor) resolveClosePrompt(action int) bool {
	if !e.closePrompt {
		return false
	}

	i := e.closePromptTab

	if i < 0 || i >= len(e.Tabs) {
		e.closePrompt = false
		return true
	}

	var ok bool

	switch action {
	case 1:
		ok = e.SaveAndCloseTab(i)

	case 2:
		ok = e.DiscardAndCloseTab(i)

	case 0:
		e.closePrompt = false
		ok = true
	}

	if ok && action != 0 {
		e.closePrompt = false
	}

	if e.OnInvalidate != nil {
		e.OnInvalidate()
	}

	return true
}

func (e *Editor) SaveActive() bool {
	if e.Viewer == nil {
		return false
	}

	return e.Viewer.Save()
}

func (e *Editor) SaveAll() bool {
	ok := true

	for _, t := range e.Tabs {
		if t.Viewer.Modified && !t.Viewer.Save() {
			ok = false
		}
	}

	return ok
}

func (e *Editor) SaveAndCloseTab(i int) bool {
	if i < 0 || i >= len(e.Tabs) {
		return false
	}

	if e.Tabs[i].Viewer.Modified &&
		!e.Tabs[i].Viewer.Save() {
		return false
	}

	return e.removeTab(i)
}

func (e *Editor) DiscardAndCloseTab(i int) bool {
	if i < 0 || i >= len(e.Tabs) {
		return false
	}

	return e.removeTab(i)
}

func (e *Editor) CloseActiveTab() bool {
	return e.CloseTab(e.ActiveTab)
}

func (e *Editor) NextTab() bool {
	if len(e.Tabs) == 0 {
		return false
	}

	i := e.ActiveTab + 1

	if i >= len(e.Tabs) {
		i = 0
	}

	return e.activateTab(i)
}

func (e *Editor) PreviousTab() bool {
	if len(e.Tabs) == 0 {
		return false
	}

	i := e.ActiveTab - 1

	if i < 0 {
		i = len(e.Tabs) - 1
	}

	return e.activateTab(i)
}

func (e *Editor) ensureActiveTabVisible() {
	if e.ActiveTab < 0 ||
		e.ActiveTab >= len(e.Tabs) {
		return
	}
}

// ToggleTerminal opens/closes the integrated IDE terminal panel.
func (e *Editor) ToggleTerminal() {
	if e.TerminalOpen {
		e.CloseTerminal()
		return
	}

	e.OpenTerminal()
}

func (e *Editor) CloseTerminal() {
	t := e.Terminal

	// Drop the reference before Close() so any asynchronous callback from
	// the old PTY cannot mistake it for the active terminal.
	e.Terminal = nil
	e.TerminalOpen = false
	e.terminalDrag = false

	if t != nil {
		t.Close()
	}

	e.activity = 0
	e.setFocusPane(0)

	if e.OnInvalidate != nil {
		e.OnInvalidate()
	}
}

func (e *Editor) OpenTerminal() {
	// If the old terminal was closed or its shell exited, create a new PTY.
	if e.Terminal == nil {
		e.Terminal = e.newTerminal()
	}

	// Keep a newly created terminal aligned with the directory currently
	// being browsed.
	if e.Tree != nil {
		e.Terminal.SetCWD(e.Tree.Root)
	}

	e.TerminalOpen = true
	e.activity = 2
	e.setFocusPane(2)

	// Start immediately so the shell prompt appears without requiring
	// the user to type first.
	if err := e.Terminal.Start(); err != nil {
		e.Terminal.Append("error: " + err.Error())
	}

	if e.OnInvalidate != nil {
		e.OnInvalidate()
	}
}

// RunActiveFile runs the active file in a live terminal.
func (e *Editor) RunActiveFile() bool {
	if e.Viewer == nil ||
		e.Viewer.Path == "" {
		return false
	}

	if e.Viewer.Modified &&
		!e.Viewer.Save() {
		return false
	}

	if e.Terminal == nil {
		e.Terminal = e.newTerminal()
	}

	e.Terminal.SetCWD(
		filepath.Dir(e.Viewer.Path),
	)

	if err := e.Terminal.RunFile(e.Viewer.Path); err != nil {
		e.Terminal.Append(
			"error: " + err.Error(),
		)

		return false
	}

	e.TerminalOpen = true
	e.activity = 2
	e.setFocusPane(2)

	if e.OnInvalidate != nil {
		e.OnInvalidate()
	}

	return true
}

func (e *Editor) terminalHeight(area geometry.Rect) int {
	if !e.TerminalOpen ||
		area.H < 10 {
		return 0
	}

	minH := e.terminalMinH

	if minH < 7 {
		minH = 7
	}

	maxH := int(
		float64(area.H) * 0.48,
	)

	if maxH < minH {
		maxH = minH
	}

	h := int(
		float64(area.H) * e.TerminalRatio,
	)

	if h < minH {
		h = minH
	}

	if h > maxH {
		h = maxH
	}

	if h > area.H-6 {
		h = area.H - 6
	}

	return h
}

func (e *Editor) drawSearchPanel(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	if area.W <= 0 ||
		area.H <= 0 {
		return
	}

	buf.FillRect(
		area.X,
		area.Y,
		area.W,
		area.H,
		' ',
		theme.Panel,
	)

	buf.SetString(
		area.X+1,
		area.Y,
		"SEARCH",
		theme.Title.WithAttr(style.Bold),
	)

	buf.SetString(
		area.X+1,
		area.Y+1,
		"Find",
		theme.TextMuted,
	)

	q := ""

	if e.Viewer != nil {
		q = e.Viewer.Search
	}

	fieldW := area.W - 2

	if fieldW > 0 {
		field := "> " + q

		if e.Viewer != nil &&
			e.Viewer.Searching {
			field += "▏"
		}

		if utf8.RuneCountInString(field) > fieldW {
			r := []rune(field)
			field = string(
				r[len(r)-fieldW:],
			)
		}

		buf.FillRect(
			area.X+1,
			area.Y+2,
			fieldW,
			1,
			' ',
			theme.Background,
		)

		buf.SetString(
			area.X+1,
			area.Y+2,
			field,
			theme.Text,
		)
	}

	if area.H > 5 {
		buf.SetString(
			area.X+1,
			area.Y+4,
			"Enter  next result",
			theme.TextMuted,
		)

		buf.SetString(
			area.X+1,
			area.Y+5,
			"Esc     close search",
			theme.TextMuted,
		)
	}
}

func (e *Editor) drawSettings(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	if area.W <= 0 || area.H <= 0 {
		return
	}

	buf.FillRect(area.X, area.Y, area.W, area.H, ' ', theme.Panel)
	content := area
	if content.W < 12 || content.H < 5 {
		return
	}

	buf.SetString(content.X+1, content.Y, "Appearance", theme.Info.WithAttr(style.Bold))
	current := "Custom"
	if e.ThemeOverride != nil && e.ThemeOverride.Name != "" {
		current = e.ThemeOverride.Name
	}
	buf.SetString(content.X+1, content.Y+1, "Editor theme", theme.TextMuted)
	buf.SetString(content.X+15, content.Y+1, current, theme.Title.WithAttr(style.Bold))

	listY := content.Y + 3
	visible := content.H - 4
	if visible < 1 {
		visible = 1
	}
	e.settingsScroll = e.clampThemeScroll(e.settingsScroll, visible)

	for row := 0; row < visible; row++ {
		i := e.settingsScroll + row
		if i >= len(e.themeOptions) {
			break
		}
		y := listY + row
		option := e.themeOptions[i]
		var optionTheme *style.EditorTheme
		if i >= 0 && i < len(e.themePreviews) {
			optionTheme = e.themePreviews[i]
		}
		selected := i == e.themeIndex

		st := theme.Text
		if selected {
			st = theme.Selected
			buf.FillRect(content.X, y, content.W, 1, ' ', st)
		}

		// A compact theme swatch previews the accent without leaking the
		// preview theme's own background into the settings surface. The swatch
		// always uses the current row background, so it never appears as a
		// black square/halo on light or colorful themes.
		swatch := theme.TextMuted.WithBg(st.Bg)
		if optionTheme != nil {
			swatch = optionTheme.Title.WithBg(st.Bg).WithAttr(style.Bold)
		}
		buf.SetString(content.X+1, y, "●", swatch)

		nameSt := st
		if selected {
			nameSt = st.WithAttr(style.Bold)
		}
		buf.SetString(content.X+3, y, option.Name, nameSt)

		if selected && content.W > 30 {
			hint := "active"
			x := content.X + content.W - utf8.RuneCountInString(hint) - 2
			buf.SetString(x, y, hint, st.WithoutAttr(style.Bold).WithAttr(style.Dim))
		}
	}

	footerY := content.Y + content.H - 1
	if footerY >= content.Y {
		buf.SetString(content.X+1, footerY, "↑/↓ select   Enter apply", theme.TextMuted)
	}
}

func (e *Editor) Draw(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	var editorTheme *style.EditorTheme

	if e.ThemeOverride != nil {
		editorTheme = e.ThemeOverride
		theme = &editorTheme.Theme

		for _, tab := range e.Tabs {
			if tab != nil &&
				tab.Viewer != nil {
				tab.Viewer.ThemeOverride = editorTheme
			}
		}

		if e.Viewer != nil {
			e.Viewer.ThemeOverride = editorTheme
		}
	}

	e.area = area

	if area.W < 10 ||
		area.H < 5 {
		return
	}

	termH := e.terminalHeight(area)

	mainH := area.H

	if termH > 0 {
		mainH = area.H - termH - 1
	}

	main := geometry.Rect{
		X: area.X,
		Y: area.Y,
		W: area.W,
		H: mainH,
	}

	// Activity rail.
	railW := 5

	if main.W < 60 {
		railW = 4
	}

	if railW >= main.W-20 {
		railW = 0
	}

	if railW > 0 {
		buf.FillRect(
			main.X,
			main.Y,
			railW,
			main.H,
			' ',
			theme.Background,
		)

		// 0 Explorer
		// 1 Search
		// 2 Terminal
		// 3 Settings

		icons := []string{
			"▣",
			"S",
			"T",
			"⚙",
		}

		for i, icon := range icons {
			y := main.Y + 1 + i*3

			if y >= main.Y+main.H {
				break
			}

			st := theme.TextMuted

			if i == e.activity {
				if railW >= 4 && y > main.Y && y+1 < main.Y+main.H {
					buf.FillRect(main.X+1, y-1, railW-2, 3, ' ', theme.Panel)
					buf.SetString(main.X+1, y-1, "▌", theme.BorderFocus)
				}
				st = theme.Info.WithAttr(style.Bold)
			}

			buf.SetString(
				main.X+(railW-1)/2,
				y,
				icon,
				st,
			)
		}

		buf.FillRect(
			main.X+railW-1,
			main.Y,
			1,
			main.H,
			'│',
			theme.Border,
		)
	}

	work := geometry.Rect{
		X: main.X + railW,
		Y: main.Y,
		W: main.W - railW,
		H: main.H,
	}

	if work.W < 8 {
		work = main
	}

	first := e.splitWidth(work)

	left := geometry.Rect{
		X: work.X,
		Y: work.Y,
		W: first,
		H: work.H,
	}

	right := geometry.Rect{
		X: work.X + first + 1,
		Y: work.Y,
		W: work.W - first - 1,
		H: work.H,
	}

	leftContent := e.panelContent(left)
	rightContent := e.panelContent(right)

	if left.W >= 10 && left.H >= 4 {
		buffer.DrawBorder(buf, left.X, left.Y, left.W, left.H, "", theme.Border, theme.Title, theme.Panel, true)
	}
	if right.W >= 18 && right.H >= 4 {
		buffer.DrawBorder(buf, right.X, right.Y, right.W, right.H, "", theme.Border, theme.Title, theme.Panel, true)
	}

	switch e.activity {
	case 1:
		e.drawSearchPanel(
			buf,
			leftContent,
			theme,
		)

	case 3:
		e.drawSettings(
			buf,
			leftContent,
			theme,
		)

	default:
		e.Tree.Draw(
			buf,
			leftContent,
			theme,
		)
	}

	buf.FillRect(
		work.X+first,
		work.Y,
		1,
		work.H,
		'│',
		theme.Border,
	)

	e.drawTabs(
		buf,
		rightContent,
		theme,
	)

	code := e.codeArea(rightContent)

	if code.H > 0 &&
		e.Viewer != nil {
		e.Viewer.ShowStatusBar = false

		e.Viewer.Draw(
			buf,
			code,
			theme,
		)
	}

	if rightContent.H >= 2 &&
		rightContent.W >= 24 {
		sy := rightContent.Y + rightContent.H - 1

		buf.FillRect(
			rightContent.X,
			sy,
			rightContent.W,
			1,
			' ',
			theme.Panel,
		)

		mode := "NORMAL"

		if e.Viewer != nil &&
			e.Viewer.Modified {
			mode = "MODIFIED"
		}

		buf.SetString(
			rightContent.X+1,
			sy,
			mode,
			theme.Info,
		)

		if e.Viewer != nil {
			pos := fmt.Sprintf(
				"Ln %d, Col %d",
				e.Viewer.Cursor+1,
				e.Viewer.Col+1,
			)

			w := utf8.RuneCountInString(pos)

			if w+2 < rightContent.W {
				buf.SetString(
					rightContent.X+rightContent.W-w-1,
					sy,
					pos,
					theme.TextMuted,
				)
			}
		}
	}

	if termH > 0 &&
		e.Terminal != nil {
		dividerY := area.Y + mainH

		buf.FillRect(
			area.X,
			dividerY,
			area.W,
			1,
			'─',
			theme.BorderFocus,
		)

		termArea := geometry.Rect{
			X: area.X,
			Y: dividerY + 1,
			W: area.W,
			H: termH,
		}

		e.Terminal.Draw(
			buf,
			termArea,
			theme,
		)
	}

	if e.closePrompt {
		e.drawClosePrompt(
			buf,
			area,
			theme,
		)
	}
}

func (e *Editor) drawClosePrompt(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	if !e.closePrompt ||
		e.closePromptTab < 0 ||
		e.closePromptTab >= len(e.Tabs) {
		return
	}

	w, h := 52, 7

	if w > area.W-2 {
		w = area.W - 2
	}

	if h > area.H-2 {
		h = area.H - 2
	}

	if w < 20 ||
		h < 5 {
		return
	}

	x := area.X + (area.W-w)/2
	y := area.Y + (area.H-h)/2

	buffer.DrawBorder(
		buf,
		x,
		y,
		w,
		h,
		"",
		theme.BorderFocus,
		theme.Title,
		theme.Panel,
		true,
	)

	name := filepath.Base(
		e.Tabs[e.closePromptTab].Path,
	)

	buf.SetString(
		x+2,
		y,
		" Unsaved Changes ",
		theme.Title.WithAttr(style.Bold),
	)

	buf.SetString(
		x+2,
		y+2,
		"Save changes to "+name+"?",
		theme.Text,
	)

	buf.SetString(
		x+2,
		y+4,
		"[S] Save   [D] Discard   [Esc] Cancel",
		theme.Warning,
	)
}

func (e *Editor) syncTabBar() {
	if e.TabBar == nil {
		return
	}

	if len(e.TabBar.Titles) == len(e.Tabs) &&
		len(e.TabBar.Modified) == len(e.Tabs) {

		changed := false

		for i, tab := range e.Tabs {
			name := ""
			modified := false

			if tab != nil {
				name = filepath.Base(tab.Path)

				modified = tab.Viewer != nil &&
					tab.Viewer.Modified
			}

			if e.TabBar.Titles[i] != name ||
				e.TabBar.Modified[i] != modified {
				changed = true
				break
			}
		}

		if !changed {
			e.TabBar.Active = e.ActiveTab
			return
		}
	}

	titles := make([]string, len(e.Tabs))
	modified := make([]bool, len(e.Tabs))

	for i, tab := range e.Tabs {
		if tab == nil {
			continue
		}

		titles[i] = filepath.Base(tab.Path)

		modified[i] = tab.Viewer != nil &&
			tab.Viewer.Modified
	}

	e.TabBar.Titles = titles
	e.TabBar.Modified = modified
	e.TabBar.Active = e.ActiveTab
}

func (e *Editor) drawTabs(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	if e.TabBar == nil {
		return
	}

	e.syncTabBar()

	e.TabBar.Draw(
		buf,
		geometry.Rect{
			X: area.X,
			Y: area.Y,
			W: area.W,
			H: 1,
		},
		theme,
	)
}

func (e *Editor) normalizeTabScroll(width int) {
	e.syncTabBar()

	if e.TabBar != nil {
		e.TabBar.Active = e.ActiveTab

		e.TabBar.Draw(
			buffer.New(
				maxInt(width, 1),
				1,
			),
			geometry.Rect{
				W: maxInt(width, 1),
				H: 1,
			},
			style.NordTheme(),
		)

		e.tabScroll = e.TabBar.Scroll
	}
}

func (e *Editor) tabAt(
	area geometry.Rect,
	x int,
) (int, bool) {
	if e.TabBar == nil {
		return -1, false
	}

	e.syncTabBar()

	e.TabBar.Active = e.ActiveTab
	e.TabBar.Scroll = e.tabScroll

	cur := area.X

	for i := e.TabBar.Scroll; i < len(e.TabBar.Titles); i++ {

		w := e.TabBar.TabWidth(i)

		if x >= cur &&
			x < cur+w {
			return i,
				e.TabBar.ShowClose &&
					x >= cur+w-3
		}

		cur += w

		if cur >= area.X+area.W {
			break
		}
	}

	return -1, false
}

func (e *Editor) finishSearch() {
	if e.Viewer != nil {
		e.Viewer.Searching = false
		e.Viewer.GotoLine = false
	}

	e.activity = 0
	e.setFocusPane(1)

	if e.OnInvalidate != nil {
		e.OnInvalidate()
	}
}

func (e *Editor) HandleKey(k input.Key) bool {
	if e.closePrompt {
		if k.Type == input.KeyEsc {
			e.closePrompt = false

			if e.OnInvalidate != nil {
				e.OnInvalidate()
			}

			return true
		}

		if k.Type == input.KeyRune {
			switch k.Rune {
			case 's', 'S':
				return e.resolveClosePrompt(1)

			case 'd', 'D':
				return e.resolveClosePrompt(2)
			}
		}

		return true
	}

	if k.Type == input.KeyF6 {
		return e.RunActiveFile()
	}

	if e.activity == 3 {
		visible := e.settingsVisibleRows()
		switch k.Type {
		case input.KeyUp:
			return e.moveThemeSelection(-1, visible)
		case input.KeyDown:
			return e.moveThemeSelection(1, visible)
		case input.KeyEnter:
			return e.SelectTheme(e.themeIndex)
		case input.KeyEsc:
			e.activity = 0
			e.setFocusPane(0)
			if e.OnInvalidate != nil {
				e.OnInvalidate()
			}
			return true
		}
		// Settings is a control surface, not an editable source buffer. Do not
		// let ordinary text input fall through to the hidden CodeViewer.
		if k.Mods&input.ModCtrl == 0 && k.Mods&input.ModAlt == 0 && k.Mods&input.ModSuper == 0 {
			return true
		}
	}

	if e.activity == 1 &&
		e.Viewer != nil &&
		e.focusPane == 1 &&
		(e.Viewer.Searching ||
			e.Viewer.GotoLine) {

		if k.Type == input.KeyEnter {
			if e.Viewer.GotoLine {
				return e.Viewer.HandleKey(k)
			}

			ok := e.Viewer.HandleKey(k)
			e.finishSearch()

			return ok
		}

		if k.Type == input.KeyEsc {
			e.Viewer.Searching = false
			e.Viewer.GotoLine = false
			e.Viewer.SetStatus("")
			e.finishSearch()

			return true
		}
	}

	if k.Type == input.KeyRune &&
		k.Mods&input.ModCtrl != 0 {

		switch k.Rune {
		case '`':
			e.ToggleTerminal()
			return true

		case 'j':
			e.ToggleTerminal()
			return true
		}
	}

	if e.TerminalOpen &&
		e.Terminal != nil &&
		e.focusPane == 2 {

		if k.Type == input.KeyEsc {
			e.setFocusPane(1)
			return true
		}

		return e.Terminal.HandleKey(k)
	}

	if k.Type == input.KeyRune &&
		k.Mods&input.ModCtrl != 0 {

		switch k.Rune {
		case 's':
			return e.SaveActive()

		case 'z', 'y', 'f', 'g', 'a', 'c', 'x', 'v':
			if e.focusPane == 1 {
				return e.Viewer.HandleKey(k)
			}
		}
	}

	if k.Type == input.KeyCtrlW ||
		(k.Type == input.KeyRune &&
			k.Mods&input.ModCtrl != 0 &&
			k.Rune == 'w') {
		return e.CloseActiveTab()
	}

	if k.Type == input.KeyCtrlP ||
		(k.Type == input.KeyRune &&
			k.Mods&input.ModCtrl != 0 &&
			k.Rune == 'p') {
		return e.PreviousTab()
	}

	if k.Type == input.KeyTab &&
		k.Mods&input.ModCtrl != 0 {
		return e.NextTab()
	}

	if k.Type == input.KeyShiftTab &&
		k.Mods&input.ModCtrl != 0 {
		return e.PreviousTab()
	}

	if k.Type == input.KeyRune &&
		k.Mods&input.ModCtrl != 0 &&
		k.Rune == 'q' {

		if e.OnExit != nil {
			e.OnExit()
			return true
		}

		return false
	}

	if e.focusPane == 0 {
		if e.Tree.HandleKey(k) {
			return true
		}

		return false
	}

	return e.Viewer.HandleKey(k)
}

func (e *Editor) HandleMouse(
	ev input.MouseEvent,
	area geometry.Rect,
) bool {
	e.area = area

	if e.closePrompt {
		if ev.Action != input.MousePress {
			return true
		}

		w, h := 52, 7

		if w > area.W-2 {
			w = area.W - 2
		}

		if h > area.H-2 {
			h = area.H - 2
		}

		x := area.X + (area.W-w)/2
		y := area.Y + (area.H-h)/2

		if ev.Y == y+h-2 &&
			ev.X >= x+2 &&
			ev.X < x+w-2 {

			pos := ev.X - (x + 2)
			zone := (w - 4) / 3

			if pos < zone {
				return e.resolveClosePrompt(1)
			}

			if pos < zone*2 {
				return e.resolveClosePrompt(2)
			}

			return e.resolveClosePrompt(0)
		}

		return true
	}

	termH := e.terminalHeight(area)

	mainH := area.H

	if termH > 0 {
		mainH = area.H - termH - 1
	}

	if termH > 0 {
		dividerY := area.Y + mainH

		termArea := geometry.Rect{
			X: area.X,
			Y: dividerY + 1,
			W: area.W,
			H: termH,
		}

		closeZone := geometry.Rect{
			X: termArea.X + termArea.W - 4,
			Y: termArea.Y,
			W: 4,
			H: 1,
		}

		if (ev.Action == input.MousePress ||
			ev.Action == input.MouseRelease) &&
			closeZone.Contains(ev.X, ev.Y) {

			if ev.Action == input.MousePress {
				e.CloseTerminal()
				e.setFocusPane(1)
			}

			return true
		}

		if ev.Action == input.MousePress &&
			ev.Y >= dividerY-2 &&
			ev.Y <= dividerY {

			e.terminalDrag = true
			e.terminalDragY = ev.Y
			e.setFocusPane(2)

			return true
		}

		if ev.Action == input.MouseDrag &&
			e.terminalDrag {

			avail := area.H - 1

			ratio := float64(
				area.Y+area.H-ev.Y,
			) / float64(avail)

			if ratio < 0.15 {
				ratio = 0.15
			}

			if ratio > 0.65 {
				ratio = 0.65
			}

			e.TerminalRatio = ratio

			return true
		}

		if ev.Action == input.MouseRelease &&
			e.terminalDrag {

			e.terminalDrag = false
			return true
		}

		if termArea.Contains(ev.X, ev.Y) &&
			e.Terminal != nil {

			e.setFocusPane(2)

			return e.Terminal.HandleMouse(
				ev,
				termArea,
			)
		}
	}

	main := geometry.Rect{
		X: area.X,
		Y: area.Y,
		W: area.W,
		H: mainH,
	}

	railW := 5

	if main.W < 60 {
		railW = 4
	}

	if railW >= main.W-20 {
		railW = 0
	}

	work := geometry.Rect{
		X: main.X + railW,
		Y: main.Y,
		W: main.W - railW,
		H: main.H,
	}

	if work.W < 8 {
		work = main
	}

	// Activity rail hit testing.
	if railW > 0 &&
		ev.Action == input.MousePress &&
		ev.X >= main.X &&
		ev.X < main.X+railW &&
		ev.Y >= main.Y+1 &&
		ev.Y < main.Y+1+4*3 {

		i := (ev.Y - (main.Y + 1)) / 3

		switch i {
		case 0:
			// Explorer.
			e.activity = 0
			e.setFocusPane(0)

			if e.Viewer != nil {
				e.Viewer.SetStatus("Explorer")
			}

			if e.OnInvalidate != nil {
				e.OnInvalidate()
			}

			return true

		case 1:
			// Search.
			e.activity = 1
			e.setFocusPane(1)

			if e.Viewer != nil {
				e.Viewer.Searching = true
				e.Viewer.GotoLine = false
				e.Viewer.Search = ""
				e.Viewer.SetSearchHit(
					e.Viewer.Cursor,
				)

				e.Viewer.SetStatus(
					"SEARCH — type text, Enter = next, Esc = close",
				)
			}

			if e.OnInvalidate != nil {
				e.OnInvalidate()
			}

			return true

		case 2:
			// Terminal.
			e.OpenTerminal()
			e.setFocusPane(2)

			return true

		case 3:
			// Settings.
			e.activity = 3
			e.setFocusPane(1)

			if e.Viewer != nil {
				e.Viewer.SetStatus("Settings")
			}

			if e.OnInvalidate != nil {
				e.OnInvalidate()
			}

			return true

		}
	}

	first := e.splitWidth(work)
	dividerX := work.X + first

	dividerHit :=
		ev.X >= dividerX-2 &&
			ev.X <= dividerX+2 &&
			ev.Y >= main.Y &&
			ev.Y < work.Y+work.H

	if ev.Action == input.MouseDrag &&
		e.dragging {

		avail := maxInt(
			work.W-1,
			1,
		)

		ratio := float64(
			ev.X-work.X,
		) / float64(avail)

		minRatio := float64(
			maxInt(e.MinTree, 1),
		) / float64(avail)

		maxRatio := float64(
			maxInt(avail-e.MinCode, 1),
		) / float64(avail)

		if minRatio > maxRatio {
			minRatio, maxRatio =
				maxRatio, minRatio
		}

		if ratio < minRatio {
			ratio = minRatio
		}

		if ratio > maxRatio {
			ratio = maxRatio
		}

		e.Ratio = ratio

		return true
	}

	if ev.Action == input.MouseRelease && e.dragging {
		e.dragging = false
		return true
	}

	if !work.Contains(ev.X, ev.Y) {
		return false
	}

	if ev.Action == input.MouseWheelUp ||
		ev.Action == input.MouseWheelDown {

		left := geometry.Rect{
			X: work.X,
			Y: work.Y,
			W: first,
			H: work.H,
		}

		right := geometry.Rect{
			X: work.X + first + 1,
			Y: work.Y,
			W: work.W - first - 1,
			H: work.H,
		}

		leftContent := e.panelContent(left)
		rightContent := e.panelContent(right)

		if left.Contains(ev.X, ev.Y) {
			e.setFocusPane(0)

			return e.Tree.HandleMouse(
				ev,
				leftContent,
			)
		}

		if right.Contains(ev.X, ev.Y) {
			e.setFocusPane(1)

			code := e.codeArea(rightContent)

			if code.W <= 0 ||
				code.H <= 0 ||
				!code.Contains(ev.X, ev.Y) {
				return true
			}

			return e.Viewer.HandleMouse(
				ev,
				code,
			)
		}

		return false
	}

	// The app captures this outer Editor widget after the initial mouse press.
	// Therefore subsequent drag/release events arrive here too. Route them to
	// the active child instead of dropping them; otherwise text selection can
	// never be completed by a real mouse drag.
	if ev.Action == input.MouseDrag || ev.Action == input.MouseRelease {
		if e.terminalDrag {
			if ev.Action == input.MouseDrag {
				avail := area.H - 1
				ratio := float64(area.Y+area.H-ev.Y) / float64(maxInt(avail, 1))
				if ratio < 0.15 {
					ratio = 0.15
				}
				if ratio > 0.65 {
					ratio = 0.65
				}
				e.TerminalRatio = ratio
				return true
			}
			e.terminalDrag = false
			return true
		}
		if e.dragging {
			if ev.Action == input.MouseDrag {
				avail := maxInt(work.W-1, 1)
				ratio := float64(ev.X-work.X) / float64(avail)
				minRatio := float64(maxInt(e.MinTree, 1)) / float64(avail)
				maxRatio := float64(maxInt(avail-e.MinCode, 1)) / float64(avail)
				if minRatio > maxRatio {
					minRatio, maxRatio = maxRatio, minRatio
				}
				if ratio < minRatio {
					ratio = minRatio
				}
				if ratio > maxRatio {
					ratio = maxRatio
				}
				e.Ratio = ratio
				return true
			}
			e.dragging = false
			return true
		}

		if e.focusPane == 1 && e.Viewer != nil {
			right := geometry.Rect{
				X: work.X + first + 1,
				Y: work.Y,
				W: work.W - first - 1,
				H: work.H,
			}
			rightContent := e.panelContent(right)
			code := e.codeArea(rightContent)
			return e.Viewer.HandleMouse(clampEditorMouseToCode(ev, code, e.Viewer), code)
		}
		if e.focusPane == 2 && e.TerminalOpen && e.Terminal != nil {
			termArea := geometry.Rect{
				X: area.X,
				Y: area.Y + mainH + 1,
				W: area.W,
				H: termH,
			}
			return e.Terminal.HandleMouse(ev, termArea)
		}
		return true
	}

	if dividerHit {
		e.dragging = true
		e.dragStartX = ev.X
		e.dragStartRatio = e.Ratio
		e.setFocusPane(0)

		return true
	}

	left := geometry.Rect{
		X: work.X,
		Y: work.Y,
		W: first,
		H: work.H,
	}

	right := geometry.Rect{
		X: work.X + first + 1,
		Y: work.Y,
		W: work.W - first - 1,
		H: work.H,
	}

	leftContent := e.panelContent(left)
	rightContent := e.panelContent(right)

	if left.Contains(ev.X, ev.Y) {
		if e.activity == 1 {
			e.setFocusPane(1)

			if ev.Action == input.MousePress &&
				e.Viewer != nil {
				e.Viewer.Searching = true
				e.Viewer.GotoLine = false
			}

			return true
		}

		if e.activity == 3 {
			e.setFocusPane(1)
			if ev.Action == input.MousePress {
				content := e.panelContent(left)
				listY := content.Y + 3
				visible := content.H - 4
				if visible < 1 {
					visible = 1
				}
				e.settingsScroll = e.clampThemeScroll(e.settingsScroll, visible)
				if ev.X >= content.X && ev.X < content.X+content.W && ev.Y >= listY && ev.Y < listY+visible {
					idx := e.settingsScroll + ev.Y - listY
					if idx >= 0 && idx < len(e.themeOptions) {
						e.SelectTheme(idx)
					}
				}
			}
			return true
		}

		e.setFocusPane(0)

		return e.Tree.HandleMouse(
			ev,
			leftContent,
		)
	}

	if !right.Contains(ev.X, ev.Y) {
		return false
	}

	e.setFocusPane(1)

	if ev.Y == rightContent.Y {
		if e.TabBar != nil {
			e.syncTabBar()

			return e.TabBar.HandleMouse(
				ev,
				geometry.Rect{
					X: rightContent.X,
					Y: rightContent.Y,
					W: rightContent.W,
					H: 1,
				},
			)
		}

		return true
	}

	code := e.codeArea(rightContent)

	if code.W <= 0 ||
		code.H <= 0 {
		return true
	}

	// The one-cell card padding is visual breathing room, but it should not
	// create dead mouse coordinates. Clamp events landing on the card chrome
	// to the nearest editor cell so a selection started beside the code remains
	// capturable when the outer IDE owns the drag sequence.
	ev = clampEditorMouseToCode(ev, code, e.Viewer)
	return e.Viewer.HandleMouse(
		ev,
		code,
	)
}

func editorDecimalWidth(n int) int {
	if n < 1 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	return w
}

func clampEditorMouseToCode(ev input.MouseEvent, r geometry.Rect, viewer *CodeViewer) input.MouseEvent {
	if r.W <= 0 || r.H <= 0 {
		return ev
	}
	codeX := r.X + 6
	if viewer != nil {
		numWidth := editorDecimalWidth(len(viewer.Lines))
		if numWidth < 2 {
			numWidth = 2
		}
		codeX = r.X + numWidth + 4
	}
	if ev.X < codeX {
		ev.X = codeX
	}
	if ev.X >= r.X+r.W {
		ev.X = r.X + r.W - 1
	}
	firstY := r.Y + 1
	if ev.Y < firstY {
		ev.Y = firstY
	}
	if ev.Y >= r.Y+r.H {
		ev.Y = r.Y + r.H - 1
	}
	return ev
}

// panelContent is the one-cell visual breathing room used by the IDE cards.
// Hit testing uses the same rectangle, so the padding never creates a visual
// coordinate mismatch between rendering and interaction.
func (e *Editor) panelContent(area geometry.Rect) geometry.Rect {
	if area.W >= 10 && area.H >= 4 {
		return area.Inset(1)
	}
	return area
}

// codeArea returns the exact rectangle used by the embedded editor.
func (e *Editor) codeArea(right geometry.Rect) geometry.Rect {
	code := right

	if code.H > 0 {
		code.Y++
		code.H--
	}

	if code.H > 0 &&
		right.W >= 24 {
		code.H--
	}

	return code
}

func (e *Editor) splitWidth(area geometry.Rect) int {
	avail := area.W - 1

	first := int(
		float64(avail) * e.Ratio,
	)

	if first < e.MinTree {
		first = e.MinTree
	}

	if avail-first < e.MinCode {
		first = avail - e.MinCode
	}

	if first < 1 {
		first = 1
	}

	return first
}

func isTextFile(path string) bool {
	ext := strings.ToLower(
		filepath.Ext(path),
	)

	switch ext {
	case ".go",
		".txt",
		".log",
		".md",
		".markdown",
		".json",
		".yaml",
		".yml",
		".toml",
		".xml",
		".html",
		".htm",
		".css",
		".js",
		".jsx",
		".ts",
		".tsx",
		".rs",
		".py",
		".rb",
		".java",
		".c",
		".h",
		".cpp",
		".hpp",
		".cc",
		".sh",
		".bash",
		".zsh",
		".fish",
		".sql",
		".proto",
		".conf",
		".ini",
		".env":
		return true
	}

	return false
}

func isTextData(data []byte) bool {
	if bytes.IndexByte(data, 0) != -1 {
		return false
	}

	if !utf8.Valid(data) {
		return false
	}

	return true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

// Keep bufio in the package's standard-library dependency set and provide a
// tiny helper for callers that want to validate a file without loading it all.
func IsReadableText(path string) bool {
	if !isTextFile(path) {
		return false
	}

	f, err := os.Open(path)

	if err != nil {
		return false
	}

	defer f.Close()

	r := bufio.NewReader(f)

	b, err := r.Peek(4096)

	return err == nil ||
		(len(b) > 0 && err == bufio.ErrBufferFull)
}
