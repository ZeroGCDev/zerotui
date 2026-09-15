package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

func benchmarkEditorFixture(b *testing.B, lines int) string {
	b.Helper()
	root := b.TempDir()
	var data []byte
	for i := 0; i < lines; i++ {
		data = append(data, []byte("package main\nfunc handleTick(price float64) { total := price * 1.05; println(total) }\n")...)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), data, 0644); err != nil {
		b.Fatal(err)
	}
	return root
}

func BenchmarkEditorDraw1000Lines(b *testing.B) {
	root := benchmarkEditorFixture(b, 500)
	e := New(root)
	buf := buffer.New(160, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 160, H: 60}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Viewer.Scroll = (i * 17) % 940
		e.Draw(buf, area, theme)
	}
}
