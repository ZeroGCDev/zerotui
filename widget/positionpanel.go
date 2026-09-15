package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// PositionPanelData contains display-ready values for one live trading
// position. Formatting stays outside Draw so a market-data update never makes
// the widget allocate or perform numeric conversions.
type PositionPanelData struct {
	Symbol, Side, Quantity string
	Entry, Mark, ROI       string
	Leverage, PnL          string
	TPPrice, TPOrder       string
}

// PositionPanel is a reusable trading position surface: summary, independent
// TP/SL toggles and sliders, and a close action. It is deliberately a panel,
// not a generic "card", because it owns a bounded compositing surface and
// keyboard/mouse focus for its four protection controls.
//
// The trading engine remains outside ZeroTUI. OnClose is only a signal; this
// widget never performs network I/O.
type PositionPanel struct {
	ThemeOverride *style.Theme
	Background    *color.Color
	Title         string
	Data          PositionPanelData

	TPEnabled *uint32
	SLEnabled *uint32
	TPValue   *uint32
	SLValue   *uint32

	TPToggle *Toggle
	SLToggle *Toggle
	TPSlider *Slider
	SLSlider *Slider

	OnClose func()

	focused bool
	control int
	dirty   uint8
}

func NewPositionPanel(tpEnabled, slEnabled, tpValue, slValue *uint32) *PositionPanel {
	p := &PositionPanel{
		TPEnabled: tpEnabled,
		SLEnabled: slEnabled,
		TPValue:   tpValue,
		SLValue:   slValue,
		dirty:     0xff,
	}
	p.TPToggle = NewToggle("TP", tpEnabled)
	p.TPToggle.OnFlag, p.TPToggle.OffFlag = "ON", "OFF"
	p.SLToggle = NewToggle("SL", slEnabled)
	p.SLToggle.OnFlag, p.SLToggle.OffFlag = "ON", "OFF"
	p.TPSlider = NewSlider("TP", tpValue, 5, 500, 5, FormatBasisPointsPct('+'))
	p.SLSlider = NewSlider("SL", slValue, 5, 500, 5, FormatBasisPointsPct('-'))
	return p
}

func (p *PositionPanel) OwnsBackground() bool { return true }
func (p *PositionPanel) Focus(v bool)         { p.focused = v; p.syncFocus(); p.dirty |= 16 }
func (p *PositionPanel) IsFocused() bool      { return p.focused }

func (p *PositionPanel) SetData(d PositionPanelData) {
	if p.Data != d {
		p.Data = d
		p.dirty |= 1 | 2 | 4
	}
}

// SetControl selects the keyboard/mouse focus target inside the panel.
// Valid values are 0=TP toggle, 1=TP slider, 2=SL toggle, 3=SL slider.
func (p *PositionPanel) SetControl(index int) {
	if index < 0 {
		index = 0
	}
	if index > 3 {
		index = 3
	}
	if p.control != index {
		p.control = index
		p.syncFocus()
		p.dirty |= 8
	}
}

func (p *PositionPanel) Control() int { return p.control }

func (p *PositionPanel) syncFocus() {
	p.TPToggle.Focus(p.focused && p.control == 0)
	p.TPSlider.Focus(p.focused && p.control == 1)
	p.SLToggle.Focus(p.focused && p.control == 2)
	p.SLSlider.Focus(p.focused && p.control == 3)
}

func (p *PositionPanel) child() Focusable {
	switch p.control {
	case 0:
		return p.TPToggle
	case 1:
		return p.TPSlider
	case 2:
		return p.SLToggle
	default:
		return p.SLSlider
	}
}

func (p *PositionPanel) controlAreas(a geometry.Rect) (geometry.Rect, geometry.Rect, geometry.Rect, geometry.Rect) {
	left := a.X + 2
	right := a.X + a.W - 2
	available := right - left
	if available < 1 {
		return geometry.Rect{}, geometry.Rect{}, geometry.Rect{}, geometry.Rect{}
	}
	toggleW := 8
	if available < toggleW+2 {
		toggleW = available / 2
	}
	if toggleW < 1 {
		toggleW = 1
	}
	sliderX := left + toggleW + 1
	sliderW := right - sliderX + 1
	if sliderW < 1 {
		sliderW = 1
	}
	return geometry.Rect{X: left, Y: a.Y + 3, W: toggleW, H: 1},
		geometry.Rect{X: sliderX, Y: a.Y + 3, W: sliderW, H: 1},
		geometry.Rect{X: left, Y: a.Y + 4, W: toggleW, H: 1},
		geometry.Rect{X: sliderX, Y: a.Y + 4, W: sliderW, H: 1}
}

func (p *PositionPanel) closeButtonVisible(area geometry.Rect) bool {
	if area.W < 12 {
		return false
	}
	x := area.X + 2
	x += CellWidth(p.Data.Symbol)
	if p.Data.Symbol != "" {
		x += 2
	}
	if p.Data.Side != "" {
		x += CellWidth(p.Data.Side) + 2
	}
	if p.Data.Quantity != "" {
		x += 4 + CellWidth(p.Data.Quantity)
	}
	return x <= area.X+area.W-10
}

func (p *PositionPanel) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if p.ThemeOverride != nil {
		theme = p.ThemeOverride
	}
	if area.W < 24 || area.H < 6 {
		return
	}
	fill := theme.Panel
	if p.Background != nil {
		fill = fill.WithBg(*p.Background)
	}
	border := theme.Border
	if p.focused {
		border = theme.BorderFocus
	}
	buffer.DrawBorder(buf, area.X, area.Y, area.W, area.H, p.Title, border, theme.Title, fill, true)

	// Row 1: compact position summary. Keep the frequently changing market
	// values on their own row so a tick only damages one narrow band.
	x := area.X + 2
	y := area.Y + 1
	buf.SetString(x, y, p.Data.Symbol, theme.Title)
	x += CellWidth(p.Data.Symbol) + 2
	if p.Data.Side != "" {
		st := theme.Text
		if p.Data.Side == "LONG" {
			st = theme.Positive
		} else if p.Data.Side == "SHORT" {
			st = theme.Negative
		}
		buf.SetString(x, y, p.Data.Side, st)
		x += CellWidth(p.Data.Side) + 2
	}
	if p.Data.Quantity != "" {
		buf.SetString(x, y, "Qty ", theme.TextMuted)
		x += 4
		buf.SetString(x, y, p.Data.Quantity, theme.Text)
	}
	closeX := area.X + area.W - 9
	if p.closeButtonVisible(area) {
		buf.SetString(closeX, y, "[CLOSE]", theme.Warning)
	}

	// Row 2: all quote/risk values share a predictable column rhythm instead
	// of being positioned by the previous string's byte length.
	y++
	colW := (area.W - 4) / 4
	if colW < 10 {
		colW = 10
	}
	writeKV := func(col int, label, value string, st style.Style) {
		cx := area.X + 2 + col*colW
		buf.SetString(cx, y, label, theme.TextMuted)
		buf.SetString(cx+CellWidth(label), y, value, st)
	}
	writeKV(0, "Entry ", p.Data.Entry, theme.Text)
	writeKV(1, "Mark ", p.Data.Mark, theme.Text)
	roiStyle := theme.Text
	if len(p.Data.ROI) > 0 && p.Data.ROI[0] == '-' {
		roiStyle = theme.Negative
	} else if len(p.Data.ROI) > 0 && p.Data.ROI[0] == '+' {
		roiStyle = theme.Positive
	}
	writeKV(2, "ROI ", p.Data.ROI, roiStyle)
	pnlStyle := theme.Text
	if len(p.Data.PnL) > 0 && p.Data.PnL[0] == '-' {
		pnlStyle = theme.Negative
	} else if len(p.Data.PnL) > 0 && p.Data.PnL[0] == '+' {
		pnlStyle = theme.Positive
	}
	writeKV(3, "PnL ", p.Data.PnL, pnlStyle)

	// Rows 3-4: protection controls. The slider itself computes a smaller
	// track when the terminal is narrow, so the control never spills outside
	// the panel.
	toggleTP, sliderTP, toggleSL, sliderSL := p.controlAreas(area)
	p.TPToggle.Draw(buf, toggleTP, theme)
	p.TPSlider.Draw(buf, sliderTP, theme)
	p.SLToggle.Draw(buf, toggleSL, theme)
	p.SLSlider.Draw(buf, sliderSL, theme)

	// Row 5: stable metadata/footer.
	y = area.Y + area.H - 1
	x = area.X + 2
	buf.SetString(x, y, "Lev ", theme.TextMuted)
	x += 4
	buf.SetString(x, y, p.Data.Leverage, theme.Text)
	x += CellWidth(p.Data.Leverage) + 2
	buf.SetString(x, y, "TP ", theme.TextMuted)
	x += 3
	buf.SetString(x, y, p.Data.TPPrice, theme.Positive)
	x += CellWidth(p.Data.TPPrice) + 2
	buf.SetString(x, y, p.Data.TPOrder, theme.TextMuted)
}

// DirtyRegions reports only the rows affected since the last query. The app
// supplies its reusable scratch slice, so this remains allocation-free.
func (p *PositionPanel) DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect {
	dirty := p.dirty
	p.dirty = 0
	if dirty == 0 || area.W <= 0 || area.H <= 0 {
		return dst[:0]
	}
	dst = dst[:0]
	add := func(y int) {
		if y >= area.Y && y < area.Y+area.H {
			dst = append(dst, geometry.Rect{X: area.X, Y: y, W: area.W, H: 1})
		}
	}
	if dirty&1 != 0 {
		add(area.Y + 1)
	}
	if dirty&2 != 0 {
		add(area.Y + 2)
	}
	if dirty&4 != 0 {
		add(area.Y + area.H - 1)
	}
	if dirty&8 != 0 {
		add(area.Y + 3)
		add(area.Y + 4)
	}
	if dirty&16 != 0 {
		return append(dst, area)
	}
	return dst
}

func (p *PositionPanel) HandleKey(k input.Key) bool {
	if !p.focused {
		return false
	}
	switch k.Type {
	case input.KeyTab:
		p.control = (p.control + 1) % 4
		p.syncFocus()
		return true
	case input.KeyShiftTab:
		p.control--
		if p.control < 0 {
			p.control = 3
		}
		p.syncFocus()
		return true
	case input.KeyRune:
		if k.Rune == 'c' || k.Rune == 'C' {
			if p.OnClose != nil {
				p.OnClose()
			}
			return true
		}
	}
	return p.child().HandleKey(k)
}

func (p *PositionPanel) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if area.W < 24 || area.H < 6 || !area.Contains(ev.X, ev.Y) {
		return false
	}
	closeX := area.X + area.W - 9
	if ev.Y == area.Y+1 && p.closeButtonVisible(area) && ev.X >= closeX && ev.X < closeX+7 {
		if ev.Action == input.MousePress && p.OnClose != nil {
			p.OnClose()
		}
		return true
	}
	t1, s1, t2, s2 := p.controlAreas(area)
	if t1.Contains(ev.X, ev.Y) {
		p.focused, p.control = true, 0
		p.syncFocus()
		return p.TPToggle.HandleMouse(ev, t1)
	}
	if s1.Contains(ev.X, ev.Y) {
		p.focused, p.control = true, 1
		p.syncFocus()
		return p.TPSlider.HandleMouse(ev, s1)
	}
	if t2.Contains(ev.X, ev.Y) {
		p.focused, p.control = true, 2
		p.syncFocus()
		return p.SLToggle.HandleMouse(ev, t2)
	}
	if s2.Contains(ev.X, ev.Y) {
		p.focused, p.control = true, 3
		p.syncFocus()
		return p.SLSlider.HandleMouse(ev, s2)
	}
	return false
}
