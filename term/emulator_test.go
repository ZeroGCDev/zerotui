package term

import (
	"fmt"

	"github.com/ZeroGCDev/zerotui/color"
	"testing"
)

func TestEmulatorANSIAndCursor(t *testing.T) {
	e := NewEmulator(20, 4, 20)
	_, _ = e.Write([]byte("hello\x1b[31m red\x1b[0m!"))
	rows, x, y, cur := e.Viewport(4, 0)
	if string([]rune{rows[0][0].Ch, rows[0][1].Ch, rows[0][2].Ch, rows[0][3].Ch, rows[0][4].Ch}) != "hello" {
		t.Fatal("plain text not rendered")
	}
	if rows[0][6].Style.Fg != color.Red {
		t.Fatalf("red SGR not applied: %#x", rows[0][6].Style.Fg)
	}
	if x == 0 || y != 0 || !cur {
		t.Fatalf("cursor state unexpected: %d,%d,%v", x, y, cur)
	}
}

func TestEmulatorAlternateScreenRestores(t *testing.T) {
	e := NewEmulator(10, 3, 10)
	_, _ = e.Write([]byte("main"))
	_, _ = e.Write([]byte("\x1b[?1049halt\x1b[?1049l"))
	rows, _, _, _ := e.Viewport(3, 0)
	if rows[0][0].Ch != 'm' {
		t.Fatalf("alternate screen did not restore main screen: %q", rows[0][0].Ch)
	}
}

func TestEmulatorRangeScrolling(t *testing.T) {
	e := NewEmulator(8, 3, 10)
	_, _ = e.Write([]byte("1\n2\n3\n4\n5"))
	if e.ScrollbackLen() < 2 {
		t.Fatalf("scrollback=%d want >=2", e.ScrollbackLen())
	}
	rows, _, _, _ := e.Viewport(3, 1)
	seen := false
	for _, r := range rows {
		for _, c := range r {
			if c.Ch != ' ' {
				seen = true
			}
		}
	}
	if !seen {
		t.Fatal("scrollback viewport is empty")
	}
}

func TestEmulatorScrollbackRingKeepsNewestRows(t *testing.T) {
	e := NewEmulator(8, 2, 3)
	for i := 1; i <= 12; i++ {
		_, _ = e.Write([]byte(fmt.Sprintf("line-%02d\r\n", i)))
	}
	if got := e.ScrollbackLen(); got != 3 {
		t.Fatalf("scrollback=%d want 3", got)
	}
	want := []string{"line-09", "line-10", "line-11"}
	for i, w := range want {
		row := e.ScrollbackRow(i)
		t.Logf("ring row %d: %q", i, stringRow(row))
		if len(row) < len(w) || string([]rune{row[0].Ch, row[1].Ch, row[2].Ch, row[3].Ch, row[4].Ch, row[5].Ch, row[6].Ch}) != w {
			t.Fatalf("row %d = %q want %q", i, stringRow(row), w)
		}
	}
}

func TestEmulatorUTF8SplitAcrossWrites(t *testing.T) {
	e := NewEmulator(8, 2, 4)
	_, _ = e.Write([]byte("caf\xC3"))
	_, _ = e.Write([]byte("\xA9"))
	rows, _, _, _ := e.Viewport(2, 0)
	got := string([]rune{rows[0][0].Ch, rows[0][1].Ch, rows[0][2].Ch, rows[0][3].Ch})
	if got != "café" {
		t.Fatalf("split UTF-8 decoded as %q", got)
	}
}

func TestEmulatorUTF8AndANSIMixedChunks(t *testing.T) {
	e := NewEmulator(16, 2, 4)
	_, _ = e.Write([]byte("x\x1b[31m\xE2"))
	_, _ = e.Write([]byte("\x98\x83\x1b[0my"))
	rows, _, _, _ := e.Viewport(2, 0)
	if rows[0][1].Ch != '☃' || rows[0][1].Style.Fg != color.Red || rows[0][2].Ch != 'y' {
		t.Fatalf("mixed split ANSI/UTF-8 decode failed: %+v %+v %+v", rows[0][0], rows[0][1], rows[0][2])
	}
}

func TestEmulatorAlternateScreenRestoresStyle(t *testing.T) {
	e := NewEmulator(12, 3, 4)
	_, _ = e.Write([]byte("\x1b[31mA\x1b[?1049h\x1b[32mB\x1b[?1049lC"))
	rows, _, _, _ := e.Viewport(3, 0)
	if rows[0][1].Ch != 'C' || rows[0][1].Style.Fg != color.Red {
		t.Fatalf("alternate screen did not restore main style: %+v", rows[0][1])
	}
}

func stringRow(row []Cell) string {
	if len(row) > 32 {
		row = row[:32]
	}
	r := make([]rune, len(row))
	for i, c := range row {
		r[i] = c.Ch
	}
	return string(r)
}

func TestEmulatorCopyViewportRangeMatchesFullViewport(t *testing.T) {
	e := NewEmulator(12, 4, 20)
	_, _ = e.Write([]byte("one\ntwo\nthree\nfour\nfive\n"))
	full, _, _, _ := e.Viewport(4, 1)
	dst := make([][]Cell, 4)
	_, _, _ = e.CopyViewportRange(dst, 4, 1, 1, 2)
	for i := 1; i < 3; i++ {
		if len(dst[i]) != len(full[i]) {
			t.Fatalf("row %d length=%d want %d", i, len(dst[i]), len(full[i]))
		}
		for x := range full[i] {
			if dst[i][x] != full[i][x] {
				t.Fatalf("row %d col %d mismatch: got %+v want %+v", i, x, dst[i][x], full[i][x])
			}
		}
	}
}

func TestEmulatorWideRuneConsumesTwoCells(t *testing.T) {
	e := NewEmulator(6, 2, 10)
	_, _ = e.Write([]byte("界A"))
	v, _, _, _, _, _ := e.Snapshot()
	if got := v[0][0].Ch; got != '界' {
		t.Fatalf("wide rune = %q", got)
	}
	if got := v[0][1].Ch; got != 0 {
		t.Fatalf("wide continuation = %q, want NUL", got)
	}
	if got := v[0][2].Ch; got != 'A' {
		t.Fatalf("following rune = %q", got)
	}
}

func TestEmulatorWideRuneWrapsAtLastColumn(t *testing.T) {
	e := NewEmulator(3, 2, 10)
	_, _ = e.Write([]byte("ab界"))
	v, _, _, _, _, _ := e.Snapshot()
	if got := v[0][0].Ch; got != 'a' {
		t.Fatalf("row0 col0 = %q", got)
	}
	if got := v[0][1].Ch; got != 'b' {
		t.Fatalf("row0 col1 = %q", got)
	}
	if got := v[1][0].Ch; got != '界' {
		t.Fatalf("row1 col0 = %q", got)
	}
	if got := v[1][1].Ch; got != 0 {
		t.Fatalf("row1 col1 = %q", got)
	}
}

func TestEmulatorPrivate47PreservesAlternateScreen(t *testing.T) {
	e := NewEmulator(8, 2, 10)
	_, _ = e.Write([]byte("main\x1b[?47hALT\x1b[?47l\x1b[?47h"))
	v, _, _, _, alt, _ := e.Snapshot()
	if !alt {
		t.Fatal("expected alternate screen")
	}
	if got := string([]rune{v[0][0].Ch, v[0][1].Ch, v[0][2].Ch}); got != "ALT" {
		t.Fatalf("alternate screen was cleared by 47: %q", got)
	}
}

func TestEmulatorRISClearsScreenAndScrollback(t *testing.T) {
	e := NewEmulator(8, 2, 8)
	_, _ = e.Write([]byte("one\n"))
	_, _ = e.Write([]byte("two\n"))
	_, _ = e.Write([]byte("\x1bc"))
	v, _, _, _, alt, _ := e.Snapshot()
	if alt {
		t.Fatal("RIS left emulator on alternate screen")
	}
	if got := v[0][0].Ch; got != ' ' {
		t.Fatalf("screen not cleared: %q", got)
	}
	if got := e.ScrollbackLen(); got != 0 {
		t.Fatalf("scrollback after RIS=%d", got)
	}
}

func TestEmulatorOriginModeCUPUsesScrollRegion(t *testing.T) {
	e := NewEmulator(8, 5, 8)
	_, _ = e.Write([]byte("\x1b[2;4r\x1b[?6h\x1b[1;1HX"))
	v, _, _, _, _, _ := e.Snapshot()
	if got := v[1][0].Ch; got != 'X' {
		t.Fatalf("origin CUP wrote %q at row1", got)
	}
}

func TestEmulatorAlternateScreenRestoresMainCursor(t *testing.T) {
	e := NewEmulator(8, 3, 4)
	_, _ = e.Write([]byte("abc\x1b[?47hALT\x1b[?47lZ"))
	v, cx, cy, _, alt, _ := e.Snapshot()
	if alt {
		t.Fatal("expected primary screen")
	}
	if cx != 4 || cy != 0 {
		t.Fatalf("restored cursor=(%d,%d), want (4,0)", cx, cy)
	}
	if got := v[0][0].Ch; got != 'a' {
		t.Fatalf("main screen lost content: %q", got)
	}
	if got := v[0][3].Ch; got != 'Z' {
		t.Fatalf("post-alt output at col3=%q", got)
	}
}

func TestEmulatorVTQueries(t *testing.T) {
	e := NewEmulator(20, 5, 8)
	_, _ = e.Write([]byte("abc\x1b[6n"))
	got := string(e.DrainResponses(nil))
	if got != "\x1b[1;4R" {
		t.Fatalf("DSR cursor report=%q", got)
	}
	_, _ = e.Write([]byte("\x1b[5n"))
	if got := string(e.DrainResponses(nil)); got != "\x1b[0n" {
		t.Fatalf("DSR status=%q", got)
	}
	_, _ = e.Write([]byte("\x1b[c"))
	if got := string(e.DrainResponses(nil)); got == "" {
		t.Fatal("DA produced no response")
	}
}

func TestEmulatorTabStops(t *testing.T) {
	e := NewEmulator(24, 2, 0)
	_, _ = e.Write([]byte("a\tX"))
	v, _, _, _, _, _ := e.Snapshot()
	if v[0][8].Ch != 'X' {
		t.Fatalf("default tab stop placed X at col %d", 8)
	}
	_, _ = e.Write([]byte("\r\x1b[3g\tY"))
	v, _, _, _, _, _ = e.Snapshot()
	if v[0][9].Ch == 'Y' {
		t.Fatal("clear-all tab stops left a stop")
	}
}

func TestEmulatorInsertLinesReusesRows(t *testing.T) {
	e := NewEmulator(20, 5, 0)
	_, _ = e.Write([]byte("a\nb\nc\nd\ne"))
	before := cap(e.main[4].cells)
	_, _ = e.Write([]byte("\x1b[2;1H\x1b[1L"))
	if cap(e.main[4].cells) != before {
		t.Fatal("insert lines changed row storage capacity")
	}
}

func TestEmulatorResizePreservesAndAddsTabStops(t *testing.T) {
	e := NewEmulator(8, 2, 0)
	e.Resize(17, 2)
	_, _ = e.Write([]byte("\x1b[13G"))
	_, _, _, _, _, _ = e.Snapshot()
	_, _ = e.Write([]byte("\tX"))
	v, _, _, _, _, _ := e.Snapshot()
	if v[0][16].Ch != 'X' {
		t.Fatalf("expanded default tab stop missing: %q", v[0][16].Ch)
	}
}
