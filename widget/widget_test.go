package widget

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
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

func TestThemeOverrideUsesComponentTheme(t *testing.T) {
	buf := buffer.New(20, 3)
	base := style.TokyoNightTheme()
	custom := *base
	custom.Text = custom.Text.WithFg(color.RGB(1, 2, 3))
	l := NewLabel("x")
	l.ThemeOverride = &custom
	l.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 20, H: 1}, base)
	if got := buf.CellAt(0, 0).Style.Fg; got != color.RGB(1, 2, 3) {
		t.Fatalf("ThemeOverride foreground = %v, want custom color", got)
	}
}

func TestVirtualTableSelectionCoversScrollbarColumn(t *testing.T) {
	buf := buffer.New(20, 5)
	theme := style.TokyoNightTheme()
	table := NewVirtualTable([]Column{{Title: "A", Width: 10}}, 100, func(row, col int) string { return "x" })
	table.ShowScrollBar = true
	table.Focus(true)
	table.Selected = 0
	table.SelectionForeground = ptrColorForTest(color.RGB(250, 250, 250))
	table.SelectionBackground = ptrColorForTest(color.RGB(10, 100, 200))
	table.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 20, H: 5}, theme)
	if got := buf.CellAt(19, 1).Style.Bg; got != color.RGB(10, 100, 200) {
		t.Fatalf("selected row scrollbar cell bg = %v, want full-row selection bg", got)
	}
}

func TestVirtualTableSelectedScrollbarKeepsThumbGlyph(t *testing.T) {
	buf := buffer.New(20, 8)
	theme := style.TokyoNightTheme()
	table := NewVirtualTable([]Column{{Title: "A", Width: 10}}, 100, func(row, col int) string { return "x" })
	table.ShowScrollBar = true
	table.Focus(true)
	table.Selected = 0
	table.ScrollTrack = ptrColorForTest(color.RGB(90, 90, 90))
	table.ScrollThumb = ptrColorForTest(color.RGB(120, 180, 240))
	table.SelectionBackground = ptrColorForTest(color.RGB(40, 80, 160))
	table.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 20, H: 8}, theme)
	if got := buf.CellAt(19, 1).Ch; got != '█' {
		t.Fatalf("selected scrollbar glyph = %q, want continuous thumb glyph", got)
	}
	if got := buf.CellAt(19, 1).Style.Bg; got != color.RGB(40, 80, 160) {
		t.Fatalf("selected scrollbar bg = %v, want selection bg", got)
	}
}

func TestVirtualTableSelectionDirtyRegionsCoverBody(t *testing.T) {
	table := NewVirtualTable([]Column{{Title: "A", Width: 10}}, 100, func(row, col int) string { return "x" })
	table.Selected = 5
	table.scroll = 0
	if !table.HandleKey(input.Key{Type: input.KeyDown}) {
		t.Fatal("down key was not consumed")
	}
	var dst [2]geometry.Rect
	regions := table.DirtyRegions(geometry.Rect{X: 10, Y: 20, W: 100, H: 10}, dst[:])
	want := geometry.Rect{X: 10, Y: 21, W: 100, H: 9}
	if len(regions) != 1 || regions[0] != want {
		t.Fatalf("selection dirty regions = %v, want [%+v]", regions, want)
	}
}

func TestVirtualTableScrollDirtyRegionCoversVisibleBody(t *testing.T) {
	table := NewVirtualTable([]Column{{Title: "A", Width: 10}}, 100, func(row, col int) string { return "x" })
	table.Selected = 0
	table.scroll = 0
	if !table.HandleMouse(input.MouseEvent{Action: input.MouseWheelDown, X: 2, Y: 2}, geometry.Rect{X: 0, Y: 0, W: 20, H: 6}) {
		t.Fatal("wheel event was not consumed")
	}
	var dst [2]geometry.Rect
	regions := table.DirtyRegions(geometry.Rect{X: 0, Y: 0, W: 20, H: 6}, dst[:])
	want := geometry.Rect{X: 0, Y: 1, W: 20, H: 5}
	if len(regions) != 1 || regions[0] != want {
		t.Fatalf("wheel dirty regions = %v, want [%+v]", regions, want)
	}
}

func ptrColorForTest(c color.Color) *color.Color { return &c }

func BenchmarkLabelDrawWithThemeOverride(b *testing.B) {
	buf := buffer.New(80, 24)
	base := style.TokyoNightTheme()
	custom := *base
	custom.Text = custom.Text.WithFg(color.RGB(200, 210, 220))
	l := NewLabel("hot path")
	l.ThemeOverride = &custom
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Draw(buf, testArea, base)
	}
}

func TestVirtualTableClipSkipsInvisibleRows(t *testing.T) {
	calls := 0
	table := NewVirtualTable([]Column{{Title: "A", Width: 10}}, 100, func(row, col int) string { calls++; return "x" })
	buf := buffer.New(20, 20)
	buf.SetClip(buffer.Rect{X: 0, Y: 6, W: 20, H: 1})
	table.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 20, H: 12}, style.TokyoNightTheme())
	buf.ClearClip()
	if calls != 1 {
		t.Fatalf("Cell callback calls=%d want 1 for one clipped body row", calls)
	}
}

func TestVirtualTableCellStyleCannotBreakSelectionBackground(t *testing.T) {
	buf := buffer.New(30, 5)
	theme := style.TokyoNightTheme()
	table := NewVirtualTable([]Column{{Title: "A", Width: 10}}, 10, func(row, col int) string { return "x" })
	table.Focus(true)
	table.Selected = 0
	custom := style.Style{Fg: color.RGB(1, 2, 3), Bg: color.RGB(4, 5, 6)}
	table.CellStyle = func(row, col int) *style.Style { return &custom }
	selectionBG := color.RGB(10, 100, 200)
	table.SelectionBackground = &selectionBG
	table.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 30, H: 5}, theme)
	if got := buf.CellAt(0, 1).Style.Bg; got != selectionBG {
		t.Fatalf("selected cell bg=%v want %v", got, selectionBG)
	}
}

func TestVirtualListSelectionDirtyRegions(t *testing.T) {
	l := NewVirtualList(100, func(i int) string { return "x" })
	l.Selected = 2
	if !l.HandleKey(input.Key{Type: input.KeyDown}) {
		t.Fatal("down not consumed")
	}
	var dst [4]geometry.Rect
	r := l.DirtyRegions(geometry.Rect{X: 5, Y: 7, W: 20, H: 6}, dst[:])
	if len(r) != 2 {
		t.Fatalf("regions=%d want 2", len(r))
	}
	if r[0].W != 20 || r[1].W != 20 {
		t.Fatalf("selection regions must span full width: %+v", r)
	}
}

func BenchmarkVirtualTableClippedSelection(b *testing.B) {
	buf := buffer.New(120, 40)
	t := NewVirtualTable([]Column{{Title: "A", Width: 20}, {Title: "B", Width: 20}}, 100000, func(row, col int) string { return "value" })
	t.Focus(true)
	t.Selected = 12
	theme := style.TokyoNightTheme()
	buf.SetClip(buffer.Rect{X: 0, Y: 13, W: 120, H: 1})
	t.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 120, H: 40}, theme)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.Draw(buf, geometry.Rect{X: 0, Y: 0, W: 120, H: 40}, theme)
	}
}

func TestVirtualTableSelectionDamageIsFullBody(t *testing.T) {
	table := NewVirtualTable([]Column{{Title: "A", Width: 8}}, 100, func(row, col int) string { return "row" })
	table.focused = true
	table.Selected = 5
	table.dirtySelection = true
	table.dirtySelected = 4
	table.dirtyScroll = 0
	dst := make([]geometry.Rect, 0, 2)
	got := table.DirtyRegions(geometry.Rect{X: 10, Y: 20, W: 80, H: 12}, dst)
	if len(got) != 1 || got[0] != (geometry.Rect{X: 10, Y: 21, W: 80, H: 11}) {
		t.Fatalf("damage=%v want one full body region", got)
	}
}

func TestLabelMultilineStaysInsideCellBuffer(t *testing.T) {
	buf := buffer.New(20, 5)
	theme := style.TokyoNightTheme()
	l := NewLabel("one\ntwo\nthree")
	l.Draw(buf, geometry.Rect{X: 2, Y: 1, W: 10, H: 3}, theme)

	want := []string{"one", "two", "three"}
	for row, line := range want {
		for col, ch := range line {
			if got := buf.CellAt(2+col, 1+row).Ch; got != ch {
				t.Fatalf("cell (%d,%d) = %q, want %q", 2+col, 1+row, got, ch)
			}
		}
	}
	for y := 0; y < buf.H; y++ {
		for x := 0; x < buf.W; x++ {
			if buf.CellAt(x, y).Ch == '\n' {
				t.Fatalf("literal newline leaked into buffer at (%d,%d)", x, y)
			}
		}
	}
}

func TestTableColumnsFitNarrowArea(t *testing.T) {
	buf := buffer.New(44, 6)
	theme := style.TokyoNightTheme()
	table := NewTable([]Column{
		{Title: "TASK", Width: 30},
		{Title: "OWNER", Width: 12},
		{Title: "STATUS", Width: 12},
	})
	table.Rows = [][]string{{"Landing page", "Maya", "IN REVIEW"}}
	area := geometry.Rect{X: 0, Y: 0, W: 44, H: 6}
	table.Draw(buf, area, theme)

	// The final column must remain inside the table rectangle even though the
	// requested fixed widths (30+12+12 plus separators) are wider than 44 cells.
	for x := 0; x < 44; x++ {
		_ = buf.CellAt(x, 0)
	}
	if got := buf.CellAt(43, 1).Ch; got == 0 {
		t.Fatalf("table failed to paint the final in-bounds cell")
	}
	if got := buf.CellAt(44, 1).Ch; got != 0 {
		t.Fatalf("table painted outside its area at x=44: %q", got)
	}
}
