package buffer

import (
	"bytes"
	"io"
	"testing"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestDirtyRowsDoNotLeakAcrossRows(t *testing.T) {
	b := New(20, 5)
	st := style.New(color.White, color.Black)
	b.Clear(st)
	var out bytes.Buffer
	if _, err := b.Render(&out); err != nil {
		t.Fatal(err)
	}
	b.Clear(st)
	b.Set(2, 1, 'X', st)
	out.Reset()
	if _, err := b.Render(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Fatal("expected changed cell output")
	}
	out.Reset()
	if _, err := b.Render(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("unchanged frame emitted %d bytes: %q", out.Len(), out.String())
	}
}

func TestRenderSynchronizedWrapsChangedFrame(t *testing.T) {
	b := New(4, 2)
	st := style.New(color.White, color.Black)
	b.Clear(st)
	var out bytes.Buffer
	if _, err := b.RenderSynchronized(&out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); len(got) < 16 || got[:8] != "\x1b[?2026h" || got[len(got)-8:] != "\x1b[?2026l" {
		t.Fatalf("missing synchronized framing: %q", got)
	}
	out.Reset()
	if _, err := b.RenderSynchronized(&out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("unchanged frame emitted %q", out.String())
	}
}

func BenchmarkRenderSynchronizedSparse(b *testing.B) {
	bfr := New(120, 40)
	st := style.New(color.White, color.Black)
	bfr.Set(2, 2, 'x', st)
	_, _ = bfr.RenderSynchronized(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bfr.Set(2+i%2, 2, 'x', st)
		_, _ = bfr.RenderSynchronized(io.Discard)
	}
}
