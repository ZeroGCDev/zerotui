package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// TextInput is a single-line editable field (order size, limit price entry, symbol search). Editing allocates (Go strings are immutable); this is expected to be an infrequent, user-paced operation, unlike the market-data render path.
// Border draws a visible frame around the field so it reads as an input rather than plain text - a single-row `[ like this ]` frame when area.H == 1 or 2 (the common case inside a Fix(...,1) layout slot), or a full titled box (Placeholder as the title) when given 3+ rows. Off by default to match every existing example exactly.
type TextInput struct {
	FocusMixin
	Placeholder string
	Value       []rune
	cursor      int
	OnSubmit    func(value string)
	Numeric     bool // restrict input to digits and '.'
	Border      bool
	Background  *color.Color // nil = inherit whatever's behind it (default)
}

func NewTextInput(placeholder string) *TextInput {
	return &TextInput{Placeholder: placeholder}
}

func (t *TextInput) String() string { return string(t.Value) }

func (t *TextInput) SetValue(s string) {
	t.Value = []rune(s)
	t.cursor = len(t.Value)
}

// contentArea returns where the editable text itself is drawn, after accounting for Border - computed identically by Draw and HandleMouse so click-to-position-cursor stays accurate whether or not a frame is drawn.
func (t *TextInput) contentArea(area geometry.Rect) geometry.Rect {
	if !t.Border || area.W < 3 {
		return area
	}
	if area.H >= 3 {
		return area.Inset(1)
	}
	return geometry.Rect{X: area.X + 1, Y: area.Y, W: area.W - 2, H: 1}
}

func (t *TextInput) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.H < 1 {
		return
	}
	frameSt := bgOr(theme.Border, t.Background)
	if t.focused {
		frameSt = bgOr(theme.BorderFocus, t.Background)
	}
	fillSt := bgOr(theme.Panel, t.Background)

	content := t.contentArea(area)
	if t.Border && area.W >= 3 {
		if area.H >= 3 {
			buffer.DrawBorder(buf, area.X, area.Y, area.W, area.H, t.Placeholder, frameSt, bgOr(theme.Title, t.Background), fillSt, false)
		} else {
			buf.FillRect(area.X, area.Y, area.W, 1, ' ', fillSt)
			buf.Set(area.X, area.Y, '[', frameSt)
			buf.Set(area.X+area.W-1, area.Y, ']', frameSt)
		}
	} else if t.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *t.Background})
	}
	buf.FillRect(content.X, content.Y, content.W, 1, ' ', fillSt)

	textSt := bgOr(theme.Text, t.Background)
	if t.focused {
		textSt = bgOr(theme.BorderFocus, t.Background)
	}
	if len(t.Value) == 0 && !t.focused {
		buf.SetString(content.X, content.Y, t.Placeholder, bgOr(theme.TextMuted, t.Background))
		return
	}
	buf.SetString(content.X, content.Y, string(t.Value), textSt)
	if t.focused {
		cx := content.X + t.cursor
		if cx < content.X+content.W {
			buf.Set(cx, content.Y, '▏', bgOr(theme.Info, t.Background))
		}
	}
}

func (t *TextInput) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyBackspace:
		if t.cursor > 0 {
			t.Value = append(t.Value[:t.cursor-1], t.Value[t.cursor:]...)
			t.cursor--
		}
		return true
	case input.KeyLeft:
		if t.cursor > 0 {
			t.cursor--
		}
		return true
	case input.KeyRight:
		if t.cursor < len(t.Value) {
			t.cursor++
		}
		return true
	case input.KeyEnter:
		if t.OnSubmit != nil {
			t.OnSubmit(string(t.Value))
		}
		return true
	case input.KeyRune, input.KeySpace:
		r := k.Rune
		if t.Numeric && !(r >= '0' && r <= '9') && r != '.' && r != '-' {
			return true
		}
		t.Value = append(t.Value[:t.cursor], append([]rune{r}, t.Value[t.cursor:]...)...)
		t.cursor++
		return true
	}
	return false
}

func (t *TextInput) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action == input.MousePress && area.Contains(ev.X, ev.Y) {
		content := t.contentArea(area)
		t.cursor = ev.X - content.X
		if t.cursor < 0 {
			t.cursor = 0
		}
		if t.cursor > len(t.Value) {
			t.cursor = len(t.Value)
		}
		return true
	}
	return false
}
