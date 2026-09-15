package widget

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

func TestFilePickerFilter(t *testing.T) {
	if !isTextEditorFile("main.go") || !isTextEditorFile("app.log") || isTextEditorFile("photo.png") {
		t.Fatal("file filter")
	}
}
func TestFilePickerUnicodeHugePathsAndChurn(t *testing.T) {
	dir := t.TempDir()
	unicode := "日本語_é_😀.go"
	if err := os.WriteFile(filepath.Join(dir, unicode), []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A deliberately long-but-valid filename exercises path handling without
	// depending on platform-specific maximum path limits.
	longName := strings.Repeat("x", 180) + ".txt"
	if err := os.WriteFile(filepath.Join(dir, longName), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewFilePicker(dir)
	p.Query = "日本語"
	p.refresh()
	if len(p.Filtered) != 1 || p.Filtered[0] != filepath.Join(dir, unicode) {
		t.Fatalf("unicode query filtered=%v", p.Filtered)
	}
	p.Query = "X" // case-insensitive search must find the long filename.
	p.refresh()
	if len(p.Filtered) != 1 || p.Filtered[0] != filepath.Join(dir, longName) {
		t.Fatalf("long-path query filtered=%v", p.Filtered)
	}

	for i := 0; i < 20; i++ {
		name := filepath.Join(dir, "churn_"+strconv.Itoa(i)+".go")
		if err := os.WriteFile(name, []byte("package p\n"), 0644); err != nil {
			t.Fatal(err)
		}
		p.SetRoot(dir)
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		p.SetRoot(dir)
	}
	if p.Query != "X" || len(p.Filtered) != 1 {
		t.Fatalf("query/filter state lost after filesystem churn: query=%q filtered=%d", p.Query, len(p.Filtered))
	}
}

func TestWidgetsHandleExtremeAreas(t *testing.T) {
	buf := buffer.New(2, 2)
	theme := style.TokyoNightTheme()
	widgets := []Widget{
		NewLabel("界😀"),
		NewTextEditor(),
		NewSparkline(2),
		NewFastLogView(4),
		NewVirtualTable([]Column{{Title: "X", Width: 1}}, 1, func(int, int) string { return "界" }),
		NewFilePicker(t.TempDir()),
	}
	areas := []geometry.Rect{{W: 0, H: 0}, {X: -10, Y: -10, W: 1, H: 1}, {W: 100000, H: 100000}}
	for _, w := range widgets {
		for _, area := range areas {
			w.Draw(buf, area, theme)
		}
	}
}

func TestTextEditorHugeUnicodeLine(t *testing.T) {
	e := NewTextEditor()
	line := strings.Repeat("界😀é", 200_000)
	e.SetText(line)
	if len(e.Lines) != 1 || e.Lines[0] != line {
		t.Fatal("huge Unicode line was not preserved")
	}
	buf := buffer.New(120, 30)
	e.Draw(buf, geometry.Rect{W: 120, H: 30}, style.TokyoNightTheme())
}

func TestTextEditorEmbeddedStatusBarUsesFinalRowForSource(t *testing.T) {
	v := NewTextEditor()
	v.SetDocument(NewDocument("one\ntwo\nthree"))
	v.ShowStatusBar = false
	buf := buffer.New(32, 4)
	v.Draw(buf, geometry.Rect{W: 32, H: 4}, style.NordTheme())
	// Header + three source rows: with no editor-owned footer, the final
	// document line must occupy the final row instead of being hidden.
	row := ""
	for x := 0; x < 32; x++ {
		row += string(buf.CellAt(x, 3).Ch)
	}
	if !strings.Contains(row, "three") {
		t.Fatalf("final source line not visible in embedded editor: %q", row)
	}
	for x := 0; x < 32; x++ {
		if buf.CellAt(x, 3).Ch == 'L' {
			t.Fatal("embedded editor unexpectedly drew its own line/column status")
		}
	}
}
