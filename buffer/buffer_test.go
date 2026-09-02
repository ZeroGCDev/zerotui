package buffer

import (
	"bytes"
	"io"
	"testing"

	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/style"
)

func TestSetStringAndRenderDiff(t *testing.T) {
	b := New(20, 5)
	st := style.New(color.White, color.Black)
	b.Clear(st)
	b.SetString(2, 1, "hello", st)
	n, err := b.Render(io.Discard)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if n == 0 {
		t.Fatal("expected first frame to write bytes (full repaint)")
	}

	// Second render of an unchanged back buffer should emit nothing.
	b.Clear(st)
	b.SetString(2, 1, "hello", st)
	n2, err := b.Render(io.Discard)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("expected 0 bytes for an unchanged frame, got %d", n2)
	}
}

func TestSetBytesClipping(t *testing.T) {
	b := New(5, 1)
	st := style.New(color.White, color.Black)
	b.SetBytes(3, 0, []byte("abcdef"), st) // should clip at buffer width
	if b.back[3].Ch != 'a' || b.back[4].Ch != 'b' {
		t.Fatalf("unexpected clipped content: %q %q", b.back[3].Ch, b.back[4].Ch)
	}
}

// BenchmarkRenderNoChange is the core zero-GC claim: once a frame has been painted, redrawing identical content and calling Render again must not touch the allocator. Run with: go test ./pkg/buffer -bench . -benchmem
func BenchmarkRenderNoChange(b *testing.B) {
	buf := New(120, 40)
	st := style.New(color.Green, color.Black)
	buf.Clear(st)
	buf.SetString(2, 2, "BTC-PERP 78900.123456789", st)
	buf.Render(io.Discard) // prime front buffer

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear(st)
		buf.SetString(2, 2, "BTC-PERP 78900.123456789", st)
		buf.Render(io.Discard)
	}
}

// BenchmarkRenderPriceTick simulates the realistic steady state: one numeric field changes every frame (a live price), everything else is static.
func BenchmarkRenderPriceTick(b *testing.B) {
	buf := New(120, 40)
	st := style.New(color.Green, color.Black)
	var scratch [16]byte
	var tick uint

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear(st)
		out := appendUintForTest(scratch[:0], uint(78900+tick%50))
		tick++
		buf.SetBytes(2, 2, out, st)
		buf.Render(io.Discard)
	}
}

func appendUintForTest(dst []byte, v uint) []byte {
	return appendUint(dst, v)
}

func TestRenderRegionsPreservesUntouchedCells(t *testing.T) {
	b := New(20, 5)
	st := style.New(color.White, color.Black)
	b.Clear(st)
	b.SetString(0, 0, "STATIC", st)
	b.SetString(0, 2, "DYNAMIC", st)
	if _, err := b.Render(io.Discard); err != nil {
		t.Fatal(err)
	}

	b.ClearRegion(Rect{X: 0, Y: 2, W: 7, H: 1}, st)
	b.SetString(0, 2, "CHANGED", st)
	if _, err := b.RenderRegions(io.Discard, []Rect{{X: 0, Y: 2, W: 7, H: 1}}); err != nil {
		t.Fatal(err)
	}

	if got := b.CellAt(0, 0).Ch; got != 'S' {
		t.Fatalf("static cell changed: %q", got)
	}
	if got := b.CellAt(0, 2).Ch; got != 'C' {
		t.Fatalf("dynamic cell missing: %q", got)
	}
}

func BenchmarkRenderRegionsSingleCell(b *testing.B) {
	buf := New(120, 40)
	st := style.New(color.Green, color.Black)
	buf.Clear(st)
	buf.Set(40, 20, '0', st)
	buf.Render(io.Discard)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.ClearRegion(Rect{X: 40, Y: 20, W: 8, H: 1}, st)
		buf.Set(40, 20, rune('0'+byte(i%10)), st)
		buf.RenderRegions(io.Discard, []Rect{{X: 40, Y: 20, W: 8, H: 1}})
	}
}

func TestRenderRegionsAvoidsRedundantCursorMoveForAdjacentRuns(t *testing.T) {
	b := New(20, 2)
	st := style.New(color.White, color.Black)
	b.Clear(st)
	b.SetString(0, 0, "abcdefghijklmnop", st)
	if _, err := b.Render(io.Discard); err != nil {
		t.Fatal(err)
	}
	b.ClearRegion(Rect{X: 0, Y: 0, W: 16, H: 1}, st)
	b.SetString(0, 0, "ABCDEFGHIJKLMNOP", st)
	var out captureWriter
	if _, err := b.RenderRegions(&out, []Rect{{X: 0, Y: 0, W: 8, H: 1}, {X: 8, Y: 0, W: 8, H: 1}}); err != nil {
		t.Fatal(err)
	}
	first := bytes.Index(out.b, []byte("\x1b[1;1H"))
	second := bytes.Index(out.b[first+1:], []byte("\x1b[1;9H"))
	if first < 0 || second >= 0 {
		t.Fatalf("adjacent runs emitted an unnecessary second cursor move: %q", string(out.b))
	}
}

type captureWriter struct{ b []byte }

func (w *captureWriter) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
