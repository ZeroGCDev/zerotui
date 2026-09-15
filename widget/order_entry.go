package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// OrderEntry collects an order intent and passes it to OnSubmit; it never performs network I/O.
type OrderEntry struct {
	ThemeOverride *style.Theme
	Background    *color.Color
	FocusMixin
	Symbol, Side, Price, Qty string
	Field                    int
	OnSubmit                 func(symbol, side, price, qty string)
}

func NewOrderEntry() *OrderEntry           { return &OrderEntry{Side: "BUY"} }
func (o *OrderEntry) OwnsBackground() bool { return o.Background != nil }
func (o *OrderEntry) Draw(b *buffer.Buffer, a geometry.Rect, th *style.Theme) {
	if o.ThemeOverride != nil {
		th = o.ThemeOverride
	}
	if a.W < 8 || a.H < 1 {
		return
	}
	if o.Background != nil {
		b.FillRect(a.X, a.Y, a.W, a.H, ' ', style.Style{Bg: *o.Background})
	}
	b.SetString(a.X, a.Y, "ORDER", bgOr(th.Title, o.Background))
	if a.H > 1 {
		b.SetString(a.X, a.Y+1, "SYM "+o.Symbol, bgOr(th.Text, o.Background))
	}
	if a.H > 2 {
		b.SetString(a.X, a.Y+2, "SIDE "+o.Side, bgOr(th.Text, o.Background))
	}
	if a.H > 3 {
		b.SetString(a.X, a.Y+3, "PX  "+o.Price, bgOr(th.Text, o.Background))
	}
	if a.H > 4 {
		b.SetString(a.X, a.Y+4, "QTY "+o.Qty, bgOr(th.Text, o.Background))
	}
}
func (o *OrderEntry) HandleKey(k input.Key) bool {
	if !o.focused {
		return false
	}
	switch k.Type {
	case input.KeyEnter:
		if o.OnSubmit != nil {
			o.OnSubmit(o.Symbol, o.Side, o.Price, o.Qty)
		}
		return true
	case input.KeyTab:
		o.Field = (o.Field + 1) % 4
		return true
	case input.KeyBackspace:
		o.del()
		return true
	case input.KeyRune:
		o.add(k.Rune)
		return true
	case input.KeySpace:
		o.add(' ')
		return true
	}
	return false
}
func (o *OrderEntry) add(r rune) {
	switch o.Field {
	case 0:
		o.Symbol += string(r)
	case 1:
		o.Side += string(r)
	case 2:
		o.Price += string(r)
	case 3:
		o.Qty += string(r)
	}
}
func (o *OrderEntry) del() {
	var s *string
	switch o.Field {
	case 0:
		s = &o.Symbol
	case 1:
		s = &o.Side
	case 2:
		s = &o.Price
	case 3:
		s = &o.Qty
	}
	if s == nil || *s == "" {
		return
	}
	for i := len(*s) - 1; i >= 0; i-- {
		if (*s)[i]&0xc0 != 0x80 {
			*s = (*s)[:i]
			return
		}
	}
}
