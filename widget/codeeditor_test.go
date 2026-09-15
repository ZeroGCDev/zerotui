package widget

import (
	"strings"
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestCodeEditorIsReusableWithoutFilesystem(t *testing.T) {
	e := NewCodeEditor()
	e.SetDocumentName("memory://scratch.go")
	e.SetLanguage("go")
	e.SetText("package main\nfunc main() {\n\tprintln(1)\n}\n")

	if len(e.Lines) != 5 {
		t.Fatalf("lines=%d", len(e.Lines))
	}
	if len(e.Tokens) == 0 {
		t.Fatal("expected syntax tokens")
	}
	if _, ok := e.Folds[1]; !ok {
		t.Fatal("expected Go fold")
	}
}

func TestCodeEditorSaveIsHostProvided(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("hello")
	called := false
	e.OnSave = func() bool {
		called = true
		return true
	}
	e.Focus(true)
	e.HandleKey(input.Key{Type: input.KeyRune, Rune: 'x'})
	if !e.Modified {
		t.Fatal("expected edit to mark document modified")
	}
	if !e.HandleKey(input.Key{Type: input.KeyRune, Rune: 's', Mods: input.ModCtrl}) {
		t.Fatal("ctrl-s was not handled")
	}
	if !called || e.Modified {
		t.Fatalf("save callback called=%v modified=%v", called, e.Modified)
	}
}

func TestCodeEditorSetLanguageUsesGenericCommentRules(t *testing.T) {
	e := NewCodeEditor()
	e.SetLanguage("python")
	e.SetText("value = 1")
	e.Cursor, e.Col = 0, 0
	e.Focus(true)
	if !e.HandleKey(input.Key{Type: input.KeyRune, Rune: '/', Mods: input.ModCtrl}) {
		t.Fatal("comment command not handled")
	}
	if e.Lines[0] != "# value = 1" {
		t.Fatalf("comment prefix=%q", e.Lines[0])
	}
}

type testHighlighter struct{}

func (testHighlighter) Highlight(_, _ string, lines []string) []SyntaxToken {
	if len(lines) == 0 {
		return nil
	}
	return []SyntaxToken{{Line: 0, Col: 0, Text: "CUSTOM", Kind: SyntaxKeyword}}
}

type testFoldProvider struct{}

func (testFoldProvider) Folds(_, _ string, lines []string) []FoldRange {
	if len(lines) < 3 {
		return nil
	}
	return []FoldRange{{Start: 0, End: 2}}
}

func TestCodeEditorAcceptsIndependentSyntaxAndFoldProviders(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("one\ntwo\nthree")
	e.SetHighlighter(testHighlighter{})
	e.SetFoldProvider(testFoldProvider{})

	if len(e.Tokens) != 1 || e.Tokens[0].Text != "CUSTOM" {
		t.Fatalf("custom tokens=%#v", e.Tokens)
	}
	got, ok := e.Folds[0]
	if !ok || got.Start != 0 || got.End != 2 {
		t.Fatalf("custom folds=%#v", e.Folds)
	}
}

func TestCodeEditorCanDisableSyntaxAndFoldingProviders(t *testing.T) {
	e := NewCodeEditor()
	e.SetText("func main() {\n}\n")
	e.SetHighlighter(nil)
	e.SetFoldProvider(nil)

	if len(e.Tokens) != 0 {
		t.Fatalf("expected no tokens, got %d", len(e.Tokens))
	}
	if len(e.Folds) != 0 {
		t.Fatalf("expected no folds, got %#v", e.Folds)
	}
}

func TestCodeEditorExplicitGoProvidersMatchDefaults(t *testing.T) {
	text := "package main\n\nfunc main() {\n\tprintln(42)\n}\n"

	defaultEditor := NewCodeEditor()
	defaultEditor.SetDocumentName("main.go")
	defaultEditor.SetLanguage("go")
	defaultEditor.SetText(text)

	explicitEditor := NewCodeEditor()
	explicitEditor.SetDocumentName("main.go")
	explicitEditor.SetLanguage("go")
	explicitEditor.SetHighlighter(GoSyntaxHighlighter{})
	explicitEditor.SetFoldProvider(GoFoldProvider{})
	explicitEditor.SetText(text)

	if len(defaultEditor.Tokens) != len(explicitEditor.Tokens) {
		t.Fatalf("default tokens=%d explicit=%d", len(defaultEditor.Tokens), len(explicitEditor.Tokens))
	}
	if len(defaultEditor.Folds) != len(explicitEditor.Folds) {
		t.Fatalf("default folds=%d explicit=%d", len(defaultEditor.Folds), len(explicitEditor.Folds))
	}
}

func TestCodeEditorCanAttachSharedDocument(t *testing.T) {
	doc := NewDocument("one\ntwo")
	doc.SetName("memory://shared.txt")
	doc.SetLanguage("text")

	a := NewCodeEditor()
	b := NewCodeEditor()
	a.SetDocument(doc)
	b.SetDocument(doc)
	a.Lines[0] = "changed"
	a.Document.Revision++

	if b.Text() != "changed\ntwo" {
		t.Fatalf("shared document text=%q", b.Text())
	}
	if !doc.Modified() {
		t.Fatal("expected shared document to be modified")
	}
	if b.Document != doc {
		t.Fatal("editors should reference the same document")
	}
}

func TestDocumentTracksSavedRevisionIndependentlyOfEditor(t *testing.T) {
	doc := NewDocument("hello")
	if doc.Modified() {
		t.Fatal("new document should be clean")
	}
	doc.SetText("hello world")
	if !doc.Modified() {
		t.Fatal("SetText should advance the document revision")
	}
	doc.MarkSaved()
	if doc.Modified() {
		t.Fatal("MarkSaved should clear dirty state")
	}
}

type rangeHighlighter struct{ ranges int }

func (h *rangeHighlighter) Highlight(_, _ string, lines []string) []SyntaxToken {
	out := make([]SyntaxToken, 0, len(lines))
	for i, line := range lines {
		out = append(out, SyntaxToken{Line: i, Col: 0, Text: line, Kind: SyntaxKeyword})
	}
	return out
}
func (h *rangeHighlighter) HighlightRange(_, _ string, lines []string, start, end int) []SyntaxToken {
	h.ranges++
	out := make([]SyntaxToken, 0, end-start)
	for i := start; i < end && i < len(lines); i++ {
		out = append(out, SyntaxToken{Line: i, Col: 0, Text: lines[i], Kind: SyntaxKeyword})
	}
	return out
}

func TestCodeEditorIncrementalRangeHighlighting(t *testing.T) {
	h := &rangeHighlighter{}
	e := NewCodeEditor()
	e.SetHighlighter(h)
	e.SetText("one\ntwo\nthree\nfour")
	before := h.ranges
	e.Cursor, e.Col = 1, 0
	e.Focus(true)
	if !e.HandleKey(input.Key{Type: input.KeyRune, Rune: 'X'}) {
		t.Fatal("edit not handled")
	}
	if h.ranges <= before {
		t.Fatalf("expected range highlight, before=%d after=%d", before, h.ranges)
	}
	if len(e.Tokens) != len(e.Lines) {
		t.Fatalf("tokens=%d lines=%d", len(e.Tokens), len(e.Lines))
	}
	if e.Tokens[2].Text != "three" || e.Tokens[3].Text != "four" {
		t.Fatalf("tokens not preserved after range edit: %#v", e.Tokens)
	}
}

func TestCodeEditorIncrementalTokensShiftAcrossLineEdits(t *testing.T) {
	h := &rangeHighlighter{}
	e := NewCodeEditor()
	e.SetHighlighter(h)
	e.SetText("one\ntwo\nthree")
	e.Focus(true)
	e.Cursor, e.Col = 0, 0
	if !e.HandleKey(input.Key{Type: input.KeyEnter}) {
		t.Fatal("newline edit not handled")
	}
	if len(e.Lines) != 4 || len(e.Tokens) != 4 {
		t.Fatalf("after insertion lines=%d tokens=%d", len(e.Lines), len(e.Tokens))
	}
	if e.Tokens[3].Text != "three" || e.Tokens[3].Line != 3 {
		t.Fatalf("shifted token=%#v", e.Tokens[3])
	}
	if !e.Undo() {
		t.Fatal("undo failed")
	}
	if len(e.Tokens) != len(e.Lines) || e.Tokens[2].Line != 2 {
		t.Fatalf("tokens after undo=%#v lines=%d", e.Tokens, len(e.Lines))
	}
}

func TestDefaultSyntaxHighlighterSupportsCommonLanguages(t *testing.T) {
	cases := map[string]string{
		"json":       `{"name": "zero", "ok": true, "n": 42}`,
		"python":     `def hello(name):\n    return f"hi {name}"`,
		"yaml":       "server:\n  port: 8080\n  enabled: true",
		"rust":       "fn main() { let value: i32 = 42; println!(\"{}\", value); }",
		"java":       "public class Main { public static void main(String[] args) { return; } }",
		"bash":       "#!/bin/bash\necho \"hello\"",
		"sh":         "#!/bin/sh\nfor x in one two; do echo $x; done",
		"toml":       "name = \"zerotui\"\nport = 8080\nenabled = true",
		"markdown":   "# ZeroTUI\n[docs](https://example.com) `code`",
		"c":          "int main() { return 0; }",
		"cpp":        "class App { public: int run() { return 0; } };",
		"javascript": "const answer = 42; console.log(answer);",
	}
	for language, text := range cases {
		e := NewCodeEditor()
		e.SetLanguage(language)
		e.SetText(text)
		if len(e.Tokens) == 0 {
			t.Errorf("language %q produced no syntax tokens", language)
		}
	}
}

func TestLanguageHighlighterRangeDoesNotScanWholeDocument(t *testing.T) {
	lines := make([]string, 500000)
	for i := range lines {
		lines[i] = "const value = 123; // line"
	}
	h := DefaultSyntaxHighlighter{}
	tokens := h.HighlightRange("large.js", "javascript", lines, 250000, 250010)
	if len(tokens) == 0 || tokens[0].Line != 250000 {
		t.Fatalf("unexpected range tokens: %#v", tokens[:min(3, len(tokens))])
	}
}

func TestCodeEditorLargeDocumentUsesViewportTokenCache(t *testing.T) {
	c := NewCodeEditor()
	data := []byte(strings.Repeat("func example() { return 123 }\n", 300_000))
	c.Load("large.go", data)
	if !c.largeDocument {
		t.Fatal("expected large-document mode")
	}
	if len(c.Tokens) != 0 {
		t.Fatalf("large document retained %d full-document tokens", len(c.Tokens))
	}
	if len(c.tokenCache) != 0 {
		t.Fatalf("expected empty viewport cache before first draw, got %d lines", len(c.tokenCache))
	}

	buf := buffer.New(120, 30)
	c.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 120, H: 30}, style.TokyoNightTheme())
	if len(c.tokenCache) == 0 {
		t.Fatal("expected visible source lines to be tokenized")
	}
	if len(c.tokenCache) >= len(c.Lines)/10 {
		t.Fatalf("viewport cache grew too large: %d cached lines for %d source lines", len(c.tokenCache), len(c.Lines))
	}

	c.Cursor = 100_000
	c.Col = 0
	c.Scroll = 100_000
	c.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 120, H: 30}, style.TokyoNightTheme())
	if _, ok := c.tokenCache[100_000]; !ok {
		t.Fatal("expected jumped-to viewport to be cached")
	}
}

func TestDefaultGoFoldProviderIgnoresBracesInLiteralsAndComments(t *testing.T) {
	lines := []string{
		"func main() {",
		`    s := "{ not a fold }"`,
		"    // } also not a fold",
		"    if ok {",
		"        println(ok)",
		"    }",
		"}",
	}
	folds := (DefaultFoldProvider{}).Folds("main.go", "go", lines)
	if len(folds) != 2 {
		t.Fatalf("folds=%d, want 2: %#v", len(folds), folds)
	}
	if _, ok := func() (FoldRange, bool) {
		for _, f := range folds {
			if f.Start == 0 && f.End == 6 {
				return f, true
			}
		}
		return FoldRange{}, false
	}(); !ok {
		t.Fatalf("missing outer fold: %#v", folds)
	}
}

func TestCodeEditorRehighlightsGoAfterMultilineCommentEdit(t *testing.T) {
	c := NewCodeEditor()
	c.Load("main.go", []byte("package main\n/* comment\nsecond comment\n*/\nfunc main() {}\n"))
	commentKind := func(line int) bool {
		for _, ts := range c.Tokens {
			if ts.Line == line && ts.Kind == SyntaxComment {
				return true
			}
		}
		return false
	}
	if !commentKind(2) {
		t.Fatal("line inside multiline comment was not highlighted as comment")
	}
	// Change the opening delimiter so the lexical state must be recomputed for
	// all following lines; a local token-range update would leave stale comment
	// tokens here.
	c.Cursor, c.Col = 1, 1
	c.deleteForward()
	if commentKind(2) {
		t.Fatal("stale multiline-comment token survived lexical boundary edit")
	}
}

func TestTextEditorContextMenuOpensAndCloses(t *testing.T) {
	v := NewTextEditor()
	v.SetText("hello")
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 8}
	if !v.HandleMouse(input.MouseEvent{Action: input.MousePress, Button: input.MouseRight, X: 20, Y: 1}, area) {
		t.Fatal("right click was not consumed")
	}
	if !v.contextMenu {
		t.Fatal("context menu did not open")
	}
	if !v.HandleMouse(input.MouseEvent{Action: input.MousePress, Button: input.MouseLeft, X: 1, Y: 1}, area) {
		t.Fatal("click outside context menu was not consumed")
	}
	if v.contextMenu {
		t.Fatal("context menu did not close")
	}
}

func TestTerminalContextMenuOpensAndCloses(t *testing.T) {
	tr := NewTerminal(".")
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 8}
	if !tr.HandleMouse(input.MouseEvent{Action: input.MousePress, Button: input.MouseRight, X: 20, Y: 1}, area) {
		t.Fatal("terminal right click was not consumed")
	}
	if !tr.contextMenu {
		t.Fatal("terminal context menu did not open")
	}
	if !tr.HandleMouse(input.MouseEvent{Action: input.MousePress, Button: input.MouseLeft, X: 1, Y: 1}, area) {
		t.Fatal("terminal outside click was not consumed")
	}
	if tr.contextMenu {
		t.Fatal("terminal context menu did not close")
	}
}

func TestCodeEditorEnterPreservesFoldsIncrementally(t *testing.T) {
	cases := []struct {
		name string
		text string
		col  int
	}{
		{"after_open_brace", "x\nfunc f() {\n\ty()\n}\nz", 10},
		{"before_open_brace", "x\nfunc f() {\n\ty()\n}\nz", 0},
		{"between_brace_pair", "x\nfunc f() { y() }\nz", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewCodeEditor()
			e.SetText(tc.text)
			e.SetLanguage("go")
			e.Cursor, e.Col = 1, tc.col
			e.Focus(true)
			if !e.HandleKey(input.Key{Type: input.KeyEnter}) {
				t.Fatal("enter not handled")
			}
			want := make(map[int]FoldRange)
			for _, f := range foldGoLines(e.Lines) {
				want[f.Start] = f
			}
			if len(e.Folds) != len(want) {
				t.Fatalf("fold count=%d want=%d", len(e.Folds), len(want))
			}
			for start, wf := range want {
				if gf, ok := e.Folds[start]; !ok || gf != wf {
					t.Fatalf("fold at %d=%+v want=%+v", start, gf, wf)
				}
			}
		})
	}
}
