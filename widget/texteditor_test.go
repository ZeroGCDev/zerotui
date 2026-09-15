package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestTextEditorIsReusableWithoutCodeProviders(t *testing.T) {
	e := NewTextEditor()
	if e.Document == nil {
		t.Fatal("expected document")
	}
	if e.Rehighlight != nil {
		t.Fatal("plain TextEditor should not require a syntax provider")
	}
	e.SetText("hello")
	if got := e.Text(); got != "hello" {
		t.Fatalf("Text() = %q", got)
	}
	if !e.HandleKey(input.Key{Type: input.KeyRune, Rune: '!'}) {
		t.Fatal("expected rune edit to be handled")
	}
	if got := e.Text(); got != "!hello" {
		t.Fatalf("after edit Text() = %q", got)
	}
}

func TestCodeEditorIsThinSyntaxSpecialization(t *testing.T) {
	c := NewCodeEditor()
	if c.TextEditor == nil {
		t.Fatal("expected embedded TextEditor")
	}
	if c.Highlighter == nil || c.FoldProvider == nil {
		t.Fatal("expected default code providers")
	}
	c.SetLanguage("go")
	c.SetText("package main\nfunc main() {}")
	if len(c.Tokens) == 0 {
		t.Fatal("expected syntax tokens")
	}
}

func TestTextEditorCoalescesAdjacentTypingUndo(t *testing.T) {
	v := NewTextEditor()
	v.Focus(true)
	for _, r := range "hello" {
		v.HandleKey(input.Key{Type: input.KeyRune, Rune: r})
	}
	if got := v.Text(); got != "hello" {
		t.Fatalf("text=%q", got)
	}
	if !v.Undo() || v.Text() != "" {
		t.Fatalf("coalesced undo failed: text=%q canUndo=%v", v.Text(), v.CanUndo())
	}
	if !v.Redo() || v.Text() != "hello" {
		t.Fatalf("coalesced redo failed: text=%q", v.Text())
	}
}

func TestTextEditorStructuralEditBreaksTypingCoalescing(t *testing.T) {
	v := NewTextEditor()
	v.Focus(true)
	v.HandleKey(input.Key{Type: input.KeyRune, Rune: 'a'})
	v.HandleKey(input.Key{Type: input.KeyEnter})
	v.HandleKey(input.Key{Type: input.KeyRune, Rune: 'b'})
	if !v.Undo() || v.Text() != "a\n" {
		t.Fatalf("expected only b undone: %q", v.Text())
	}
	if !v.Undo() || v.Text() != "a" {
		t.Fatalf("expected newline undone: %q", v.Text())
	}
	if !v.Undo() || v.Text() != "" {
		t.Fatalf("expected a undone: %q", v.Text())
	}
}

func TestTextEditorContextMenuUsesThemeAwareSurface(t *testing.T) {
	e := NewTextEditor()
	e.SetText("package main\n")
	e.Focus(true)
	area := geometry.Rect{X: 0, Y: 0, W: 40, H: 10}
	if !e.HandleMouse(input.MouseEvent{X: 15, Y: 1, Button: input.MouseRight, Action: input.MousePress}, area) {
		t.Fatal("expected context-menu click to be consumed")
	}

	theme := style.NewEditorTheme(style.TokyoNightTheme())
	buf := buffer.New(area.W, area.H)
	e.Draw(buf, area, &theme.Theme)

	menu := e.contextMenuRect(area)
	cell := buf.CellAt(menu.X+2, menu.Y+2)
	if cell.Style.Bg != theme.ContextMenu.Bg {
		t.Fatalf("context menu background = %#v, want %#v", cell.Style.Bg, theme.ContextMenu.Bg)
	}
	if cell.Style.Attr.Has(style.Reverse) {
		t.Fatal("context menu must not use terminal Reverse styling")
	}
}
