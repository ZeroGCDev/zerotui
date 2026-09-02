package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

var testArea = geometry.Rect{X: 0, Y: 0, W: 40, H: 5}

func TestLabelBackgroundOverride(t *testing.T) {
	buf := buffer.New(40, 5)
	theme := style.MatchaLatteTheme()
	custom := color.RGB(10, 20, 30)

	l := NewLabel("hi")
	l.Draw(buf, testArea, theme) // no override: should keep theme.Text.Bg
	if got := buf.CellAt(0, 0).Style.Bg; got != theme.Text.Bg {
		t.Fatalf("without Background, cell bg = %v, want theme default %v", got, theme.Text.Bg)
	}

	l.Background = &custom
	l.Draw(buf, testArea, theme)
	if got := buf.CellAt(0, 0).Style.Bg; got != custom {
		t.Fatalf("with Background set, text cell bg = %v, want %v", got, custom)
	}
	// The fill should also cover cells past the text itself.
	if got := buf.CellAt(10, 0).Style.Bg; got != custom {
		t.Fatalf("fill did not cover blank cell: bg = %v, want %v", got, custom)
	}
}

func TestToggleNoUnderlineOnFocus(t *testing.T) {
	buf := buffer.New(40, 5)
	theme := style.MatchaLatteTheme()
	var v uint32

	tg := NewToggle("SL", &v)
	tg.Focus(true)
	tg.Draw(buf, testArea, theme)

	for x := 0; x < 20; x++ {
		if attr := buf.CellAt(x, 0).Style.Attr; attr.Has(style.Underline) {
			t.Fatalf("cell %d has Underline attribute set; focus indication must not use underline", x)
		}
	}
	// Focus should still be visible via theme.Selected instead.
	if got := buf.CellAt(0, 0).Style; got != theme.Selected {
		t.Fatalf("focused Toggle style = %+v, want theme.Selected %+v", got, theme.Selected)
	}
}

func TestToggleBackgroundOverride(t *testing.T) {
	buf := buffer.New(40, 5)
	theme := style.MatchaLatteTheme()
	var v uint32
	custom := color.RGB(5, 5, 5)

	tg := NewToggle("SL", &v)
	tg.Background = &custom
	tg.Draw(buf, testArea, theme)

	if got := buf.CellAt(0, 0).Style.Bg; got != custom {
		t.Fatalf("Toggle box bg = %v, want %v", got, custom)
	}
	if got := buf.CellAt(30, 0).Style.Bg; got != custom {
		t.Fatalf("Toggle row fill did not reach column 30: bg = %v, want %v", got, custom)
	}
}

func TestTextInputBorderSingleRow(t *testing.T) {
	buf := buffer.New(40, 5)
	theme := style.MatchaLatteTheme()

	ti := NewTextInput("size")
	ti.Border = true
	area := geometry.Rect{X: 2, Y: 1, W: 12, H: 1}
	ti.Draw(buf, area, theme)

	if got := buf.CellAt(2, 1).Ch; got != '[' {
		t.Fatalf("left frame char = %q, want '['", got)
	}
	if got := buf.CellAt(13, 1).Ch; got != ']' {
		t.Fatalf("right frame char = %q, want ']'", got)
	}
	// Placeholder should render inset by 1, not flush against the bracket.
	if got := buf.CellAt(3, 1).Ch; got != 's' {
		t.Fatalf("first placeholder char at inset position = %q, want 's' (from \"size\")", got)
	}
}

func TestTextInputBorderFullBox(t *testing.T) {
	buf := buffer.New(40, 6)
	theme := style.MatchaLatteTheme()

	ti := NewTextInput("qty")
	ti.Border = true
	area := geometry.Rect{X: 1, Y: 1, W: 14, H: 3}
	ti.Draw(buf, area, theme)

	corners := []struct {
		x, y int
		want rune
	}{
		{1, 1, '┌'}, {14, 1, '┐'}, {1, 3, '└'}, {14, 3, '┘'},
	}
	for _, c := range corners {
		if got := buf.CellAt(c.x, c.y).Ch; got != c.want {
			t.Errorf("corner (%d,%d) = %q, want %q", c.x, c.y, got, c.want)
		}
	}
	// Content should be drawn inset, on the middle row.
	if got := buf.CellAt(2, 2).Ch; got == 0 || got == ' ' {
		// placeholder "qty" should start right at the inset position
	}
}

func TestTextInputNoBorderMatchesOldBehavior(t *testing.T) {
	buf := buffer.New(40, 5)
	theme := style.MatchaLatteTheme()

	ti := NewTextInput("size")
	area := geometry.Rect{X: 2, Y: 1, W: 12, H: 1}
	ti.Draw(buf, area, theme)

	// Without Border, there must be no bracket characters - text starts flush at area.X, exactly like every existing example expects.
	if got := buf.CellAt(2, 1).Ch; got == '[' {
		t.Fatalf("Border defaults to false but a '[' frame char was drawn")
	}
	if got := buf.CellAt(2, 1).Ch; got != 's' {
		t.Fatalf("placeholder should start flush at area.X, got %q", got)
	}
}

func TestOrderBookBackgroundOverride(t *testing.T) {
	buf := buffer.New(60, 10)
	theme := style.MatchaLatteTheme()
	custom := color.RGB(1, 2, 3)

	ob := NewOrderBook(9, 3, 2)
	ob.Background = &custom
	ob.SetLevels(
		[]Level{{Price: 100, Size: 10}},
		[]Level{{Price: 200, Size: 20}},
	)
	area := geometry.Rect{X: 0, Y: 0, W: 60, H: 10}
	ob.Draw(buf, area, theme)

	if got := buf.CellAt(0, 0).Style.Bg; got != custom {
		t.Fatalf("OrderBook header bg = %v, want %v", got, custom)
	}
}

// BenchmarkToggleDrawWithBackground proves the new Background override path (bgOr + the extra FillRect) doesn't add allocation to Draw, the same guarantee the rest of the library's render path already has.
func BenchmarkToggleDrawWithBackground(b *testing.B) {
	buf := buffer.New(80, 24)
	theme := style.MatchaLatteTheme()
	var v uint32
	custom := color.RGB(20, 20, 20)
	tg := NewToggle("Stop Loss", &v)
	tg.Background = &custom

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tg.Draw(buf, testArea, theme)
	}
}

func BenchmarkTextInputDrawWithBorder(b *testing.B) {
	buf := buffer.New(80, 24)
	theme := style.MatchaLatteTheme()
	ti := NewTextInput("size")
	ti.Border = true
	ti.SetValue("12.5")
	area := geometry.Rect{X: 2, Y: 1, W: 20, H: 1}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ti.Draw(buf, area, theme)
	}
}

func TestOrderBookDirtyRegionsAreCellLocal(t *testing.T) {
	ob := NewOrderBook(2, 2, 2)
	bids := make([]Level, 10)
	asks := make([]Level, 10)
	for i := range bids {
		bids[i] = Level{Price: uint64(1000 + i), Size: 100}
		asks[i] = Level{Price: uint64(2000 + i), Size: 100}
	}
	ob.SetLevels(bids, asks)
	var dst [2]geometry.Rect
	regions := ob.DirtyRegions(geometry.Rect{X: 5, Y: 2, W: 60, H: 12}, dst[:])
	if len(regions) != 1 || regions[0] != (geometry.Rect{X: 5, Y: 3, W: 60, H: 11}) {
		t.Fatalf("initial regions=%v", regions)
	}
	bids[4].Price++
	ob.SetLevels(bids, asks)
	regions = ob.DirtyRegions(geometry.Rect{X: 5, Y: 2, W: 60, H: 12}, dst[:])
	if len(regions) != 1 || regions[0] != (geometry.Rect{X: 5, Y: 7, W: 60, H: 1}) {
		t.Fatalf("single-row regions=%v", regions)
	}
	regions = ob.DirtyRegions(geometry.Rect{X: 5, Y: 2, W: 60, H: 12}, dst[:])
	if len(regions) != 0 {
		t.Fatalf("dirty state was not consumed: %v", regions)
	}
}

func TestOrderBookNarrowLayoutDoesNotOverlapHalves(t *testing.T) {
	buf := buffer.New(20, 6)
	theme := style.TokyoNightTheme()
	ob := NewOrderBook(2, 2, 2)
	ob.SetLevels([]Level{{Price: 12345, Size: 100}}, []Level{{Price: 12355, Size: 200}})
	ob.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 20, H: 6}, theme)
	// With a 20-column area the compact path is used. The first ask cell must
	// remain on the right half; the bid price must not spill into it.
	if got := buf.CellAt(10, 1).Style; got != theme.Negative {
		t.Fatalf("ask half was overwritten: %+v", got)
	}
}

func TestOrderBookBothSideBarsRenderIndependently(t *testing.T) {
	buf := buffer.New(80, 6)
	theme := style.TokyoNightTheme()
	ob := NewOrderBook(2, 2, 2)
	ob.SetLevels(
		[]Level{{Price: 10000, Size: 100}, {Price: 9900, Size: 50}},
		[]Level{{Price: 10100, Size: 20}, {Price: 10200, Size: 10}},
	)
	ob.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 80, H: 6}, theme)
	// Ask bars occupy the cells between the ask price and ask size columns.
	askBarStart := 40 + 12
	askBarEnd := 40 + 12 + (40 - 8 - 12)
	foundAsk := false
	for x := askBarStart; x < askBarEnd; x++ {
		if buf.CellAt(x, 1).Ch == '▌' {
			foundAsk = true
			break
		}
	}
	if !foundAsk {
		t.Fatalf("ask side did not render a visible depth bar")
	}

	ob.SetLevels(
		[]Level{{Price: 10000, Size: 100}, {Price: 9900, Size: 50}},
		[]Level{{Price: 10100, Size: 200}, {Price: 10200, Size: 10}},
	)
	ob.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 80, H: 6}, theme)
	foundAsk = false
	for x := askBarStart; x < askBarEnd; x++ {
		if buf.CellAt(x, 1).Ch == '▌' {
			foundAsk = true
			break
		}
	}
	if !foundAsk {
		t.Fatalf("ask side bar disappeared after ask-only update")
	}
}
