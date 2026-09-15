package benchmarks

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
	"testing"
)

func BenchmarkThemeStress_ConstructionAll(b *testing.B) {
	opts := style.BuiltInEditorThemes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = opts[i%len(opts)].Factory()
	}
}

func BenchmarkThemeStress_CloneAndNormalizeAll(b *testing.B) {
	opts := style.BuiltInEditorThemes()
	themes := make([]*style.EditorTheme, len(opts))
	for i, o := range opts {
		themes[i] = o.Factory()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = themes[i%len(themes)].Clone().WithEditorBackground()
	}
}

func BenchmarkThemeStress_LookupAll(b *testing.B) {
	opts := style.BuiltInEditorThemes()
	names := make([]string, len(opts))
	for i, o := range opts {
		names[i] = o.Name
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t, ok := style.EditorThemeByName(names[i%len(names)])
		if !ok || t == nil {
			b.Fatal("theme lookup failed")
		}
	}
}

func BenchmarkThemeStress_TextEditorDrawAll(b *testing.B) {
	opts := style.BuiltInEditorThemes()
	editors := make([]*widget.TextEditor, len(opts))
	themes := make([]*style.EditorTheme, len(opts))
	text := "package main\n\nfunc handleTick(price float64) {\n\t// stress syntax backgrounds\n\ttotal := price * 1.05\n\tprintln(total)\n}\n"
	for i, o := range opts {
		themes[i] = o.Factory()
		editors[i] = widget.NewTextEditor()
		editors[i].SetText(text)
		editors[i].ThemeOverride = themes[i]
	}
	buf := buffer.New(160, 60)
	area := geometry.Rect{W: 160, H: 60}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := editors[i%len(editors)]
		e.Scroll = i % 2
		e.Draw(buf, area, themes[i%len(themes)].Theme.Clone())
	}
}
