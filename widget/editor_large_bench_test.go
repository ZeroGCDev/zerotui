package widget

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

func makeLargeEditorText(lines int) string {
	var b strings.Builder
	b.Grow(lines * 24)
	for i := 0; i < lines; i++ {
		b.WriteString("line ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" payload α世界")
		if i+1 < lines {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func BenchmarkTextEditorDraw500K(b *testing.B) {
	e := NewTextEditor()
	e.SetText(makeLargeEditorText(500_000))
	buf := buffer.New(160, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 160, H: 60}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Scroll = (i * 997) % (500_000 - 60)
		e.Draw(buf, area, theme)
	}
}

func BenchmarkTextEditorSearch500K(b *testing.B) {
	e := NewTextEditor()
	e.SetText(makeLargeEditorText(500_000))
	e.Search = "世界"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.searchHit = (i * 1009) % len(e.Lines)
		stress := e.findNext()
		if !stress {
			b.Fatal("expected search hit")
		}
	}
}

func makeEditorSource(lines int) []byte {
	var b strings.Builder
	b.Grow(lines * 48)
	for i := 0; i < lines; i++ {
		b.WriteString("package main\n")
		if i%4 == 0 {
			b.WriteString("func handleTick(price float64) { total := price * 1.05; println(total) }\n")
		} else {
			b.WriteString("// market update: BTC-PERP ETH-PERP SOL-PERP\n")
		}
	}
	return []byte(b.String())
}

func BenchmarkCodeEditorViewportDraw(b *testing.B) {
	e := NewCodeEditor()
	e.Load("main.go", makeEditorSource(10000))
	buf := buffer.New(160, 60)
	theme := style.TokyoNightTheme()
	area := geometry.Rect{W: 160, H: 60}
	e.Scroll = 1000
	e.Draw(buf, area, theme)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Scroll = (i * 997) % 9940
		e.Draw(buf, area, theme)
	}
}

func BenchmarkCodeEditorSearch10000(b *testing.B) {
	e := NewCodeEditor()
	e.Load("main.go", makeEditorSource(10000))
	e.Search = "market update"
	e.searchHit = 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.searchHit = i % len(e.Lines)
		if !e.findNext() {
			b.Fatal("expected search hit")
		}
	}
}

func BenchmarkCodeEditorEditAndHighlight(b *testing.B) {
	e := NewCodeEditor()
	e.Load("main.go", makeEditorSource(1000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Cursor = i % len(e.Lines)
		e.Col = 0
		e.HandleKey(input.Key{Type: input.KeyRune, Rune: 'x'})
		e.Undo()
	}
}
