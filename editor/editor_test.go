package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

func TestLoadFileScansGoOnceAndBuildsFolds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	src := "package main\n\nfunc main() {\n\tprintln(42)\n}\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	if !e.LoadFile(path) {
		t.Fatal("LoadFile rejected Go source")
	}
	if e.Viewer.Path != path || len(e.Viewer.Lines) < 5 {
		t.Fatal("viewer was not populated")
	}
	if _, ok := e.Viewer.Folds[2]; !ok {
		t.Fatalf("expected function fold, got %#v", e.Viewer.Folds)
	}
	if len(e.Viewer.Tokens) == 0 || e.Viewer.LineTokenCount() != len(e.Viewer.Lines) {
		t.Fatal("syntax tokens were not indexed")
	}
	e.Viewer.Cursor = 2
	if !e.Viewer.ToggleFold() {
		t.Fatal("expected fold toggle at cursor")
	}
}

func TestLoadFileRejectsBinaryAndUnsupportedFiles(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "image.png")
	if err := os.WriteFile(bin, []byte{0x89, 'P', 'N', 'G', 0}, 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	if e.LoadFile(bin) {
		t.Fatal("binary/unsupported file was accepted")
	}
}

func TestFileTreeContainsOnlySupportedFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package p"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "ok.log"), []byte("hello"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "no.png"), []byte("PNG"), 0644)
	tree := NewFileTree(dir)
	var nodes [32]*node
	visible := tree.visible(nodes[:0])
	for _, n := range visible {
		if !n.dir && filepath.Ext(n.name) == ".png" {
			t.Fatal("unsupported file leaked into tree")
		}
	}
}

func TestCodeViewerExpandsTabsForTerminalCells(t *testing.T) {
	v := NewCodeViewer()
	v.Load("main.go", []byte("package main\nfunc main() {\n\tprintln(42)\n}\n"))
	if len(v.Lines) < 3 {
		t.Fatal("expected source lines")
	}
	if got := v.Lines[2]; got != "    println(42)" {
		t.Fatalf("expanded indentation=%q want four spaces", got)
	}
	if _, ok := v.Folds[1]; !ok {
		t.Fatalf("fold coordinates changed unexpectedly: %#v", v.Folds)
	}
	for _, line := range v.Lines {
		for _, r := range line {
			if r == '\t' {
				t.Fatal("viewer retained a literal tab; terminal-cell output can desynchronize")
			}
		}
	}
}

func TestCodeViewerSurvivesRepeatedResizeGeometry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	src := "package main\n\nfunc main() {\n\tprintln(42)\n}\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	if !e.LoadFile(path) {
		t.Fatal("LoadFile rejected Go source")
	}
	for _, width := range []int{120, 90, 160, 72, 140, 80, 180, 100} {
		b := buffer.New(width, 24)
		e.Draw(b, geometry.Rect{W: width, H: 24}, style.NordTheme())
		// The indented source line must remain contiguous in the cell buffer
		// regardless of how often the terminal geometry changes.
		found := false
		for y := 0; y < 24 && !found; y++ {
			var got []rune
			for x := 0; x < width; x++ {
				got = append(got, b.CellAt(x, y).Ch)
			}
			if strings.Contains(string(got), "    println(42)") {
				found = true
			}
		}
		if !found {
			t.Fatalf("width %d lost/repositioned source after resize", width)
		}
	}
}

func TestEditorOpensFilesInTabsAndCachesParsedViewer(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	if err := os.WriteFile(a, []byte("package a\nfunc A() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("package b\nfunc B() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	if !e.LoadFile(a) || !e.LoadFile(b) {
		t.Fatal("failed to open files")
	}
	if len(e.Tabs) != 2 || e.ActiveTab != 1 || e.Viewer.Path != b {
		t.Fatalf("tabs=%d active=%d path=%q", len(e.Tabs), e.ActiveTab, e.Viewer.Path)
	}
	bViewer := e.Viewer
	if !e.LoadFile(a) {
		t.Fatal("failed to switch to existing tab")
	}
	if len(e.Tabs) != 2 || e.Viewer == bViewer || e.Viewer.Path != a {
		t.Fatal("existing tab was not activated correctly")
	}
	if !e.LoadFile(b) || e.Viewer != bViewer {
		t.Fatal("existing tab was reparsed/replaced")
	}
	if !e.CloseActiveTab() || len(e.Tabs) != 1 || e.Viewer.Path != a {
		t.Fatal("closing active tab did not select left tab")
	}
}

func TestEditorTabNavigationAndCloseShortcut(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go"), filepath.Join(dir, "c.go")}
	for i, p := range paths {
		if err := os.WriteFile(p, []byte("package p\nvar X = "+fmt.Sprint(i)+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	e := New(dir)
	for _, p := range paths {
		e.LoadFile(p)
	}
	if !e.NextTab() || e.ActiveTab != 0 {
		t.Fatalf("next tab wrapped incorrectly: %d", e.ActiveTab)
	}
	if !e.PreviousTab() || e.ActiveTab != 2 {
		t.Fatalf("previous tab wrapped incorrectly: %d", e.ActiveTab)
	}
	if !e.HandleKey(input.Key{Type: input.KeyRune, Rune: 'w', Mods: input.ModCtrl}) {
		t.Fatal("ctrl+w not consumed")
	}
	if len(e.Tabs) != 2 || e.Viewer.Path != paths[1] {
		t.Fatalf("close shortcut state: tabs=%d path=%q", len(e.Tabs), e.Viewer.Path)
	}
}

func TestCodeViewerEditingUndoRedoAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	v := NewCodeViewer()
	v.Load(path, mustRead(t, path))
	v.Focus(true)
	v.Cursor, v.Col = 1, len([]rune(v.Lines[1]))
	v.HandleKey(input.Key{Type: input.KeyRune, Rune: '!'})
	if !v.Modified || !strings.HasSuffix(v.Lines[1], "!") {
		t.Fatalf("edit failed: %#v", v.Lines)
	}
	if !v.Undo() || strings.HasSuffix(v.Lines[1], "!") {
		t.Fatalf("undo failed: %#v", v.Lines)
	}
	if !v.Redo() || !strings.HasSuffix(v.Lines[1], "!") {
		t.Fatalf("redo failed: %#v", v.Lines)
	}
	if !v.Save() {
		t.Fatal("save failed")
	}
	got, _ := os.ReadFile(path)
	if string(got) != strings.Join(v.Lines, "\n") {
		t.Fatalf("saved mismatch: %q", got)
	}
}

func TestCodeViewerSelectionCutPaste(t *testing.T) {
	v := NewCodeViewer()
	v.Lines = []string{"hello world"}
	v.Cursor, v.Col = 0, 11
	v.SetSelectionAnchor(0, 6)
	if !v.CopySelection(true) {
		t.Fatal("cut not handled")
	}
	if v.Lines[0] != "hello " {
		t.Fatalf("cut result=%q", v.Lines[0])
	}
	v.Col = 6
	v.Paste()
	if v.Lines[0] != "hello world" {
		t.Fatalf("paste result=%q", v.Lines[0])
	}
}

func TestEditorDoesNotCloseDirtyTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	e.Viewer.Focus(true)
	e.Viewer.HandleKey(input.Key{Type: input.KeyRune, Rune: 'x'})
	if !e.Viewer.Modified {
		t.Fatal("expected dirty tab")
	}
	if !e.CloseActiveTab() || !e.closePrompt {
		t.Fatal("dirty tab did not open save/discard prompt")
	}
	if len(e.Tabs) != 1 {
		t.Fatal("dirty tab disappeared before prompt decision")
	}
	if !e.resolveClosePrompt(1) || e.closePrompt || len(e.Tabs) != 0 {
		t.Fatal("save-and-close prompt flow failed")
	}
	if len(e.Tabs) == 0 {
		return
	}
	if !e.SaveActive() || !e.CloseActiveTab() {
		t.Fatal("save then close failed")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCodeViewerAutoIndentPairsAndTab(t *testing.T) {
	v := NewCodeViewer()
	v.Lines = []string{"func main() {"}
	v.Cursor, v.Col = 0, len([]rune(v.Lines[0]))
	v.Focus(true)
	v.HandleKey(input.Key{Type: input.KeyEnter})
	if v.Lines[1] != "    " {
		t.Fatalf("auto indent=%q", v.Lines[1])
	}
	v.HandleKey(input.Key{Type: input.KeyRune, Rune: '('})
	if v.Lines[1] != "    ()" || v.Col != 5 {
		t.Fatalf("pair insertion=%q col=%d", v.Lines[1], v.Col)
	}
	v.HandleKey(input.Key{Type: input.KeyTab})
	if v.Lines[1] != "    (    )" {
		t.Fatalf("tab indent=%q", v.Lines[1])
	}
	v.HandleKey(input.Key{Type: input.KeyShiftTab})
	if v.Lines[1] != "(    )" {
		t.Fatalf("outdent=%q", v.Lines[1])
	}
}

func TestCodeViewerMouseDragSelection(t *testing.T) {
	v := NewCodeViewer()
	v.Lines = []string{"hello world"}
	v.Focus(true)
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 10}
	numWidth := 1
	codeX := area.X + numWidth + 4
	v.HandleMouse(input.MouseEvent{X: codeX + 6, Y: area.Y + 1, Action: input.MousePress}, area)
	v.HandleMouse(input.MouseEvent{X: codeX + 11, Y: area.Y + 1, Action: input.MouseDrag}, area)
	if !v.HasSelection() {
		t.Fatal("drag did not create selection")
	}
	if got := func() string {
		sl, sc, el, ec, _ := v.SelectedRange()
		if sl != 0 || el != 0 {
			return "bad"
		}
		return string([]rune(v.Lines[0])[sc:ec])
	}(); got != "world" {
		t.Fatalf("selection=%q", got)
	}
}

func TestEditorClipboardAndCodeEditing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {\n\tprintln(\"hi\")\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	if e.Viewer == nil {
		t.Fatal("missing viewer")
	}
	v := e.Viewer
	v.Cursor, v.Col = 1, 0
	if !v.HandleKey(input.Key{Type: input.KeyRune, Rune: 'x'}) {
		t.Fatal("insert not handled")
	}
	if !v.Modified {
		t.Fatal("edit did not mark document modified")
	}
	if !v.Undo() || !v.Redo() {
		t.Fatal("undo/redo failed")
	}
	v.Cursor, v.Col = 1, 0
	v.ToggleComment()
	if !strings.Contains(v.Lines[1], "//") {
		t.Fatal("comment toggle failed")
	}
	v.ToggleComment()
	if strings.Contains(v.Lines[1], "//") {
		t.Fatal("comment untoggle failed")
	}
	v.DuplicateLine()
	if len(v.Lines) != 6 {
		t.Fatalf("duplicate line: got %d lines", len(v.Lines))
	}
	v.DeleteLine()
	if len(v.Lines) != 5 {
		t.Fatalf("delete line: got %d lines", len(v.Lines))
	}
}

func TestGotoLine(t *testing.T) {
	v := NewCodeViewer()
	v.Path = "x.go"
	v.Lines = []string{"one", "two", "three"}
	if !v.GotoLineNumber(3) || v.Cursor != 2 {
		t.Fatalf("goto line failed: cursor=%d", v.Cursor)
	}
	if v.GotoLineNumber(4) {
		t.Fatal("out-of-range goto should fail")
	}
}

func TestCodeViewerEmptyEditorAcceptsTyping(t *testing.T) {
	v := NewCodeViewer()
	v.HandleKey(input.Key{Type: input.KeyRune, Rune: 'h'})
	if len(v.Lines) != 1 || v.Lines[0] != "h" {
		t.Fatalf("empty editor typing failed: %#v", v.Lines)
	}
}

func TestCodeViewerMouseWheelScrolls(t *testing.T) {
	v := NewCodeViewer()
	v.Lines = make([]string, 40)
	for i := range v.Lines {
		v.Lines[i] = "line"
	}
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 10}
	if !v.HandleMouse(input.MouseEvent{Action: input.MouseWheelDown, X: 10, Y: 5}, area) {
		t.Fatal("wheel down not handled")
	}
	if v.Scroll == 0 {
		t.Fatal("wheel down did not scroll")
	}
	if !v.HandleMouse(input.MouseEvent{Action: input.MouseWheelUp, X: 10, Y: 5}, area) {
		t.Fatal("wheel up not handled")
	}
	if v.Scroll != 0 {
		t.Fatalf("wheel up did not return to top: %d", v.Scroll)
	}
}

func TestEditorMouseWheelReachesViewer(t *testing.T) {
	e := New("")
	e.Viewer.Lines = make([]string, 40)
	for i := range e.Viewer.Lines {
		e.Viewer.Lines[i] = "line"
	}
	area := geometry.Rect{X: 0, Y: 0, W: 100, H: 12}
	if !e.HandleMouse(input.MouseEvent{Action: input.MouseWheelDown, X: 70, Y: 6}, area) {
		t.Fatal("wheel down not handled")
	}
	if e.Viewer.Scroll == 0 {
		t.Fatal("viewer did not scroll")
	}
}

func TestCodeViewerMouseWheelDoesNotSnapBackToCursor(t *testing.T) {
	v := NewCodeViewer()
	v.Lines = make([]string, 100)
	for i := range v.Lines {
		v.Lines[i] = "line"
	}
	area := geometry.Rect{X: 0, Y: 0, W: 80, H: 12}
	v.Cursor = 0
	if !v.HandleMouse(input.MouseEvent{Action: input.MouseWheelDown, X: 20, Y: 6}, area) {
		t.Fatal("wheel down not handled")
	}
	if v.Scroll == 0 {
		t.Fatal("wheel down did not move viewport")
	}
	want := v.Scroll
	// Draw calls ensureScroll; manual wheel scrolling must survive it.
	v.EnsureScroll(area.H-2, 40)
	if v.Scroll != want {
		t.Fatalf("viewport snapped back after draw preparation: got %d want %d", v.Scroll, want)
	}
	for i := 0; i < 20; i++ {
		v.HandleMouse(input.MouseEvent{Action: input.MouseWheelDown, X: 20, Y: 6}, area)
	}
	if v.Scroll <= want {
		t.Fatalf("repeated wheel events stopped advancing viewport: got %d after %d", v.Scroll, want)
	}
}

func TestEditorFocusPropagatesToActiveViewer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	e.Focus(true)
	if !e.IsFocused() || !e.Viewer.IsFocused() || e.Tree.IsFocused() {
		t.Fatalf("focus propagation failed: editor=%v viewer=%v tree=%v", e.IsFocused(), e.Viewer.IsFocused(), e.Tree.IsFocused())
	}

	widget.ResetCursorBlink()
	b := buffer.New(80, 12)
	e.Draw(b, geometry.Rect{W: 80, H: 12}, style.NordTheme())
	found := false
	for y := 1; y < 11 && !found; y++ {
		for x := 20; x < 80; x++ {
			c := b.CellAt(x, y)
			if c.Style == style.NordTheme().Selected {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("focused editor did not render a visible selected cursor cell")
	}
}

func TestEditorTabStripReflowsAfterWidthChanges(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha.go", "beta.go", "gamma.go", "delta.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package p\nfunc X() {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	e := New(dir)
	for _, name := range []string{"alpha.go", "beta.go", "gamma.go", "delta.go"} {
		if !e.LoadFile(filepath.Join(dir, name)) {
			t.Fatalf("failed to load %s", name)
		}
	}
	// Make the last tab active, then repeatedly change terminal width. The
	// active tab must remain reachable in the rendered strip.
	e.ActiveTab = len(e.Tabs) - 1
	e.Viewer = e.Tabs[e.ActiveTab].Viewer
	e.setFocusPane(1)
	for _, width := range []int{140, 80, 55, 40, 120, 65, 150} {
		b := buffer.New(width, 16)
		e.Draw(b, geometry.Rect{W: width, H: 16}, style.NordTheme())
		if e.tabScroll < 0 || e.tabScroll > e.ActiveTab {
			t.Fatalf("width %d produced invalid tabScroll=%d", width, e.tabScroll)
		}
		// tabAt should still locate the active tab somewhere in the visible strip.
		visible := false
		for x := 0; x < width; x++ {
			idx, _ := e.tabAt(geometry.Rect{X: 0, Y: 0, W: width, H: 1}, x)
			if idx == e.ActiveTab {
				visible = true
				break
			}
		}
		if !visible {
			t.Fatalf("width %d hid active tab %d (scroll=%d)", width, e.ActiveTab, e.tabScroll)
		}
	}
}

func TestCodeViewerAutoScrollFollowsCaret(t *testing.T) {
	v := NewCodeViewer()
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	v.Lines = lines
	v.SetViewport(8, 40)
	v.Cursor, v.Col = 0, 0
	v.SetManualScroll(false)
	for i := 0; i < 30; i++ {
		v.MoveLine(1, false)
	}
	if v.Cursor != 30 {
		t.Fatalf("cursor=%d", v.Cursor)
	}
	if v.Scroll > v.Cursor || v.Cursor >= v.Scroll+v.ViewportHeight() {
		t.Fatalf("caret not kept visible: cursor=%d scroll=%d viewport=%d", v.Cursor, v.Scroll, v.ViewportHeight())
	}
}

func TestEditorActivityRailActionsAreVisibleAndFunctional(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	e.Focus(true)
	e.area = geometry.Rect{X: 0, Y: 0, W: 100, H: 30}

	// Explorer
	if !e.HandleMouse(input.MouseEvent{X: 1, Y: 1, Action: input.MousePress}, e.area) || e.activity != 0 || e.focusPane != 0 {
		t.Fatal("Explorer rail action failed")
	}
	// Search
	if !e.HandleMouse(input.MouseEvent{X: 1, Y: 4, Action: input.MousePress}, e.area) || e.activity != 1 || !e.Viewer.Searching || e.focusPane != 1 {
		t.Fatal("Search rail action failed")
	}
	// Terminal
	if !e.HandleMouse(input.MouseEvent{X: 1, Y: 7, Action: input.MousePress}, e.area) || e.activity != 2 || !e.TerminalOpen || e.focusPane != 2 {
		t.Fatal("Terminal rail action failed")
	}
	// Settings
	if !e.HandleMouse(input.MouseEvent{X: 1, Y: 10, Action: input.MousePress}, e.area) || e.activity != 3 || e.focusPane != 1 {
		t.Fatal("Settings rail action failed")
	}
}

func TestEditorTerminalCloseButton(t *testing.T) {
	e := New(t.TempDir())
	e.Focus(true)
	e.TerminalOpen = true
	area := geometry.Rect{X: 0, Y: 0, W: 100, H: 30}
	termH := e.terminalHeight(area)
	mainH := area.H - termH - 1
	termArea := geometry.Rect{X: area.X, Y: area.Y + mainH + 1, W: area.W, H: termH}
	closeX := termArea.X + termArea.W - 2
	closeY := termArea.Y
	if !e.HandleMouse(input.MouseEvent{X: closeX, Y: closeY, Action: input.MousePress}, area) {
		t.Fatal("terminal close click not consumed")
	}
	if e.TerminalOpen {
		t.Fatal("terminal remained open after close click")
	}
	if e.focusPane != 1 {
		t.Fatal("editor did not regain focus after terminal close")
	}
}

func TestSearchActivityReturnsToEditorAfterEnter(t *testing.T) {
	e := New(t.TempDir())
	e.Focus(true)
	e.area = geometry.Rect{X: 0, Y: 0, W: 100, H: 30}

	if !e.HandleMouse(input.MouseEvent{X: 1, Y: 4, Action: input.MousePress}, e.area) {
		t.Fatal("search rail click not handled")
	}
	if e.activity != 1 || !e.Viewer.Searching {
		t.Fatalf("search did not open: activity=%d searching=%v", e.activity, e.Viewer.Searching)
	}
	for _, r := range "func" {
		e.HandleKey(input.Key{Type: input.KeyRune, Rune: r})
	}
	if !e.HandleKey(input.Key{Type: input.KeyEnter}) {
		t.Fatal("search enter not handled")
	}
	if e.activity != 0 || e.focusPane != 1 || e.Viewer.Searching {
		t.Fatalf("search did not return to editor: activity=%d focus=%d searching=%v", e.activity, e.focusPane, e.Viewer.Searching)
	}
}

func TestSearchActivityEscReturnsToEditor(t *testing.T) {
	e := New(t.TempDir())
	e.Focus(true)
	e.area = geometry.Rect{X: 0, Y: 0, W: 100, H: 30}
	e.HandleMouse(input.MouseEvent{X: 1, Y: 4, Action: input.MousePress}, e.area)
	if !e.HandleKey(input.Key{Type: input.KeyEsc}) {
		t.Fatal("search escape not handled")
	}
	if e.activity != 0 || e.focusPane != 1 || e.Viewer.Searching {
		t.Fatalf("search did not close: activity=%d focus=%d searching=%v", e.activity, e.focusPane, e.Viewer.Searching)
	}
}

func TestEditorMouseScrollReachesFinalSourceLine(t *testing.T) {
	e := New(t.TempDir())
	e.Viewer.Lines = make([]string, 48)
	for i := range e.Viewer.Lines {
		e.Viewer.Lines[i] = fmt.Sprintf("source-line-%02d", i+1)
	}
	e.Viewer.Cursor = 0
	e.Viewer.Col = 0
	e.Viewer.ShowStatusBar = false

	w, h := 120, 44
	b := buffer.New(w, h)
	area := geometry.Rect{W: w, H: h}
	theme := style.NordTheme()
	e.Draw(b, area, theme)

	// Scroll through the actual code viewport, not the larger tab/status pane.
	for i := 0; i < 64; i++ {
		e.HandleMouse(input.MouseEvent{X: 80, Y: 20, Action: input.MouseWheelDown}, area)
	}
	e.Draw(b, area, theme)

	found := false
	for y := 0; y < h; y++ {
		var row []rune
		for x := 0; x < w; x++ {
			row = append(row, b.CellAt(x, y).Ch)
		}
		if strings.Contains(string(row), "source-line-48") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mouse scrolling stopped before the final source line; scroll=%d viewport=%d", e.Viewer.Scroll, e.Viewer.ViewportHeight())
	}
}

func TestEditorTerminalToggleFocusesTerminal(t *testing.T) {
	e := New(t.TempDir())
	e.Focus(true)
	if !e.HandleKey(input.Key{Type: input.KeyRune, Rune: 'j', Mods: input.ModCtrl}) {
		t.Fatal("Ctrl+J was not handled")
	}
	if !e.TerminalOpen || e.focusPane != 2 || !e.Terminal.IsFocused() {
		t.Fatalf("terminal did not receive focus: open=%v pane=%d focused=%v", e.TerminalOpen, e.focusPane, e.Terminal.IsFocused())
	}
	if !e.HandleKey(input.Key{Type: input.KeyEsc}) {
		t.Fatal("Esc was not handled")
	}
	if e.focusPane != 1 {
		t.Fatalf("Esc did not return focus to editor: pane=%d", e.focusPane)
	}
}

func TestEditorTerminalInvalidationCallback(t *testing.T) {
	e := New(t.TempDir())
	called := false
	e.OnInvalidate = func() { called = true }
	e.Terminal = e.newTerminal()
	e.Terminal.OnChange()
	if !called {
		t.Fatal("terminal output did not route through editor invalidation")
	}
}

func TestEditorDoesNotHijackArrowNavigation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {\n    value := 123\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	if !e.LoadFile(path) {
		t.Fatal("failed to load test file")
	}
	e.setFocusPane(1)
	e.Viewer.Cursor = 2
	e.Viewer.Col = 8

	for _, k := range []input.Key{
		{Type: input.KeyLeft},
		{Type: input.KeyRight},
		{Type: input.KeyUp},
		{Type: input.KeyDown},
	} {
		e.HandleKey(k)
		if e.focusPane != 1 {
			t.Fatalf("arrow %v changed focus pane to %d", k.Type, e.focusPane)
		}
	}
}

func TestEditorDoesNotUseArrowKeysForTabSwitching(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package p\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	e := New(dir)
	if !e.LoadFile(filepath.Join(dir, "a.go")) || !e.LoadFile(filepath.Join(dir, "b.go")) {
		t.Fatal("failed to load test tabs")
	}
	e.ActiveTab = 0
	e.Viewer = e.Tabs[0].Viewer
	e.setFocusPane(1)

	for _, k := range []input.Key{
		{Type: input.KeyLeft},
		{Type: input.KeyRight},
		{Type: input.KeyLeft, Mods: input.ModCtrl},
		{Type: input.KeyRight, Mods: input.ModCtrl},
		{Type: input.KeyLeft, Mods: input.ModAlt},
		{Type: input.KeyRight, Mods: input.ModAlt},
	} {
		e.HandleKey(k)
		if e.ActiveTab != 0 {
			t.Fatalf("arrow combination %v/%v changed active tab to %d", k.Type, k.Mods, e.ActiveTab)
		}
	}
}

func TestEditorRoutesMouseDragToViewerSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() { println(\"hello world\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	e := New(dir)
	e.Focus(true)
	e.setFocusPane(1)

	area := geometry.Rect{X: 0, Y: 0, W: 100, H: 30}
	e.area = area
	work := geometry.Rect{X: 5, Y: 0, W: 95, H: 30}
	right := geometry.Rect{X: work.X + e.splitWidth(work) + 1, Y: work.Y, W: work.W - e.splitWidth(work) - 1, H: work.H}
	code := e.codeArea(right)
	if code.W <= 0 || code.H <= 0 {
		t.Fatal("invalid code area")
	}

	// The exact code x-coordinate is derived from the viewer's line-number
	// gutter: use the first visible character on line 1 and drag several cells.
	numWidth := decimalWidthForTest(len(e.Viewer.Lines))
	codeX := code.X + numWidth + 4
	startX := codeX
	endX := codeX + 5
	y := code.Y + 1
	if !e.HandleMouse(input.MouseEvent{X: startX, Y: y, Button: input.MouseLeft, Action: input.MousePress}, area) {
		t.Fatal("selection press was not consumed")
	}
	if !e.HandleMouse(input.MouseEvent{X: endX, Y: y, Button: input.MouseLeft, Action: input.MouseDrag}, area) {
		t.Fatal("selection drag was not consumed")
	}
	if !e.HandleMouse(input.MouseEvent{X: endX, Y: y, Button: input.MouseLeft, Action: input.MouseRelease}, area) {
		t.Fatal("selection release was not consumed")
	}
	if !e.Viewer.HasSelection() {
		t.Fatal("viewer lost mouse selection during outer-widget capture")
	}
}

func decimalWidthForTest(n int) int {
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

func TestEditorThemeSelectionAppliesAndNotifies(t *testing.T) {
	e := New(t.TempDir())
	var notified string
	e.OnThemeChange = func(theme *style.EditorTheme) {
		if theme != nil {
			notified = theme.Name
		}
	}
	if !e.SelectTheme(3) {
		t.Fatal("SelectTheme(3) failed")
	}
	if e.ThemeOverride == nil || e.ThemeOverride.Name != "Dracula" {
		t.Fatalf("unexpected selected theme: %#v", e.ThemeOverride)
	}
	if notified != "Dracula" {
		t.Fatalf("theme callback not notified: %q", notified)
	}
}
