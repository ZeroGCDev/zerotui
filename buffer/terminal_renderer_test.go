package buffer

import (
	"bytes"
	"testing"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestColorOnlySGRTransition(t *testing.T) {
	b := New(2, 1)
	a := style.Style{Fg: color.RGB(255, 0, 0), Bg: color.RGB(0, 0, 0), Attr: style.Bold}
	c := style.Style{Fg: color.RGB(0, 255, 0), Bg: color.RGB(0, 0, 0), Attr: style.Bold}
	b.Set(0, 0, 'A', a)
	b.Set(1, 0, 'B', c)
	var out bytes.Buffer
	if _, err := b.Render(&out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if bytes.Contains(out.Bytes(), []byte("\x1b[0;1;38;2;0;255;0;48;2;0;0;0m")) {
		t.Fatalf("color-only transition unexpectedly reset full style: %q", got)
	}
	if !bytes.Contains(out.Bytes(), []byte("\x1b[38;2;0;255;0m")) {
		t.Fatalf("missing compact foreground transition: %q", got)
	}
}

func TestRelativeCursorMove(t *testing.T) {
	b := New(20, 1)
	st := style.New(color.White, color.Black)
	b.Clear(st)
	if _, err := b.Render(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	b.Set(0, 0, 'C', st)
	b.Set(3, 0, 'B', st)
	var out bytes.Buffer
	if _, err := b.Render(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("\x1b[2C")) {
		t.Fatalf("expected relative cursor move, output=%q", out.String())
	}
}

func TestUnicodeEncoding(t *testing.T) {
	b := New(4, 1)
	st := style.New(color.White, color.Black)
	b.SetString(0, 0, "界A", st)
	var out bytes.Buffer
	if _, err := b.Render(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("界A")) {
		t.Fatalf("unicode glyph was not encoded correctly: %q", out.String())
	}
}
