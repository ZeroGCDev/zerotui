// Package style defines per-cell visual attributes and named application themes so widgets never hardcode colors.
package style

import "github.com/ZeroGCDev/zerotui/color"

type Attr uint8

const (
	AttrNone Attr = 0
	Bold
	Dim
	Underline
	Reverse
	Blink
)

func (a Attr) Has(f Attr) bool { return a&f != 0 }

// Style is a value type (no pointers) so it can be embedded directly in Cell without allocation.
type Style struct {
	Fg   color.Color
	Bg   color.Color
	Attr Attr
}

func New(fg, bg color.Color) Style { return Style{Fg: fg, Bg: bg} }

func (s Style) WithAttr(a Attr) Style { s.Attr |= a; return s }

// WithoutAttr returns a copy of s with the supplied terminal attributes cleared.
func (s Style) WithoutAttr(a Attr) Style { s.Attr &^= a; return s }

// WithBg returns a copy of s with the background swapped, used everywhere a widget's optional per-instance Background override is applied.
func (s Style) WithBg(bg color.Color) Style { s.Bg = bg; return s }

// WithFg returns a copy of s with the foreground swapped.
func (s Style) WithFg(fg color.Color) Style { s.Fg = fg; return s }

// Clone returns an independent theme value. Prepare clones during setup when a
// single component needs a small palette variation; Draw never needs to copy a
// theme.
func (t *Theme) Clone() *Theme {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

// WithBackground returns a copy with every role using the supplied background.
// It is intended for building component themes during setup, not in Draw.
func (t *Theme) WithBackground(bg color.Color) *Theme {
	c := t.Clone()
	if c == nil {
		return nil
	}
	roles := []*Style{&c.Background, &c.Panel, &c.Border, &c.BorderFocus, &c.Text, &c.TextMuted, &c.Title, &c.Positive, &c.Negative, &c.Warning, &c.Info, &c.Selected, &c.TrackFull, &c.TrackEmpty}
	for _, st := range roles {
		st.Bg = bg
	}
	return c
}

func TokyoNightTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.TokyoWhite, Bg: color.TokyoBackground},
		Panel:       Style{Fg: color.TokyoWhite, Bg: color.TokyoPanel},
		Border:      Style{Fg: color.TokyoBorder, Bg: color.TokyoPanel},
		BorderFocus: Style{Fg: color.TokyoBlue, Bg: color.TokyoPanel},
		Text:        Style{Fg: color.TokyoWhite, Bg: color.TokyoPanel},
		TextMuted:   Style{Fg: color.TokyoGray, Bg: color.TokyoPanel},
		Title:       Style{Fg: color.TokyoCyan, Bg: color.TokyoPanel, Attr: Bold},
		Positive:    Style{Fg: color.TokyoGreen, Bg: color.TokyoPanel, Attr: Bold},
		Negative:    Style{Fg: color.TokyoRed, Bg: color.TokyoPanel, Attr: Bold},
		Warning:     Style{Fg: color.TokyoAmber, Bg: color.TokyoPanel, Attr: Bold},
		Info:        Style{Fg: color.TokyoCyan, Bg: color.TokyoPanel},
		Selected:    Style{Fg: color.TokyoBackground, Bg: color.TokyoBlue, Attr: Bold},
		TrackFull:   Style{Fg: color.TokyoCyan, Bg: color.TokyoPanel},
		TrackEmpty:  Style{Fg: color.TokyoDimGray, Bg: color.TokyoPanel},
	}
}

func MatchaLatteTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.MatchaWhite, Bg: color.MatchaBackground},
		Panel:       Style{Fg: color.MatchaWhite, Bg: color.MatchaPanel},
		Border:      Style{Fg: color.MatchaBorder, Bg: color.MatchaPanel},
		BorderFocus: Style{Fg: color.MatchaGreen, Bg: color.MatchaPanel},
		Text:        Style{Fg: color.MatchaWhite, Bg: color.MatchaPanel},
		TextMuted:   Style{Fg: color.MatchaGray, Bg: color.MatchaPanel},
		Title:       Style{Fg: color.MatchaGreen, Bg: color.MatchaPanel, Attr: Bold},
		Positive:    Style{Fg: color.MatchaGreen, Bg: color.MatchaPanel, Attr: Bold},
		Negative:    Style{Fg: color.MatchaRed, Bg: color.MatchaPanel, Attr: Bold},
		Warning:     Style{Fg: color.MatchaAmber, Bg: color.MatchaPanel, Attr: Bold},
		Info:        Style{Fg: color.MatchaCyan, Bg: color.MatchaPanel},
		Selected:    Style{Fg: color.MatchaBackground, Bg: color.MatchaGreen, Attr: Bold},
		TrackFull:   Style{Fg: color.MatchaGreen, Bg: color.MatchaPanel},
		TrackEmpty:  Style{Fg: color.MatchaDimGray, Bg: color.MatchaPanel},
	}
}

func VaporwaveTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.VaporWhite, Bg: color.VaporBackground},
		Panel:       Style{Fg: color.VaporWhite, Bg: color.VaporPanel},
		Border:      Style{Fg: color.VaporBorder, Bg: color.VaporPanel},
		BorderFocus: Style{Fg: color.VaporCyan, Bg: color.VaporPanel},
		Text:        Style{Fg: color.VaporWhite, Bg: color.VaporPanel},
		TextMuted:   Style{Fg: color.VaporGray, Bg: color.VaporPanel},
		Title:       Style{Fg: color.VaporCyan, Bg: color.VaporPanel, Attr: Bold},
		Positive:    Style{Fg: color.VaporGreen, Bg: color.VaporPanel, Attr: Bold},
		Negative:    Style{Fg: color.VaporRed, Bg: color.VaporPanel, Attr: Bold},
		Warning:     Style{Fg: color.VaporAmber, Bg: color.VaporPanel, Attr: Bold},
		Info:        Style{Fg: color.VaporCyan, Bg: color.VaporPanel},
		Selected:    Style{Fg: color.VaporBackground, Bg: color.VaporCyan, Attr: Bold},
		TrackFull:   Style{Fg: color.VaporCyan, Bg: color.VaporPanel},
		TrackEmpty:  Style{Fg: color.VaporDimGray, Bg: color.VaporPanel},
	}
}
func MochaEspressoTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.MochaWhite, Bg: color.MochaBackground},
		Panel:       Style{Fg: color.MochaWhite, Bg: color.MochaPanel},
		Border:      Style{Fg: color.MochaBorder, Bg: color.MochaPanel},
		BorderFocus: Style{Fg: color.MochaAmber, Bg: color.MochaPanel},
		Text:        Style{Fg: color.MochaWhite, Bg: color.MochaPanel},
		TextMuted:   Style{Fg: color.MochaGray, Bg: color.MochaPanel},
		Title:       Style{Fg: color.MochaAmber, Bg: color.MochaPanel, Attr: Bold},
		Positive:    Style{Fg: color.MochaGreen, Bg: color.MochaPanel, Attr: Bold},
		Negative:    Style{Fg: color.MochaRed, Bg: color.MochaPanel, Attr: Bold},
		Warning:     Style{Fg: color.MochaAmber, Bg: color.MochaPanel, Attr: Bold},
		Info:        Style{Fg: color.MochaCyan, Bg: color.MochaPanel},
		Selected:    Style{Fg: color.MochaBackground, Bg: color.MochaAmber, Attr: Bold},
		TrackFull:   Style{Fg: color.MochaAmber, Bg: color.MochaPanel},
		TrackEmpty:  Style{Fg: color.MochaDimGray, Bg: color.MochaPanel},
	}
}

func DeepAbyssTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.AbyssWhite, Bg: color.AbyssBackground},
		Panel:       Style{Fg: color.AbyssWhite, Bg: color.AbyssPanel},
		Border:      Style{Fg: color.AbyssBorder, Bg: color.AbyssPanel},
		BorderFocus: Style{Fg: color.AbyssCyan, Bg: color.AbyssPanel},
		Text:        Style{Fg: color.AbyssWhite, Bg: color.AbyssPanel},
		TextMuted:   Style{Fg: color.AbyssGray, Bg: color.AbyssPanel},
		Title:       Style{Fg: color.AbyssCyan, Bg: color.AbyssPanel, Attr: Bold},
		Positive:    Style{Fg: color.AbyssGreen, Bg: color.AbyssPanel, Attr: Bold},
		Negative:    Style{Fg: color.AbyssRed, Bg: color.AbyssPanel, Attr: Bold},
		Warning:     Style{Fg: color.AbyssAmber, Bg: color.AbyssPanel, Attr: Bold},
		Info:        Style{Fg: color.AbyssCyan, Bg: color.AbyssPanel},
		Selected:    Style{Fg: color.AbyssBackground, Bg: color.AbyssCyan, Attr: Bold},
		TrackFull:   Style{Fg: color.AbyssCyan, Bg: color.AbyssPanel},
		TrackEmpty:  Style{Fg: color.AbyssDimGray, Bg: color.AbyssPanel},
	}
}

func NordTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.NordWhite, Bg: color.NordBackground},
		Panel:       Style{Fg: color.NordWhite, Bg: color.NordPanel},
		Border:      Style{Fg: color.NordBorder, Bg: color.NordPanel},
		BorderFocus: Style{Fg: color.NordBlue, Bg: color.NordPanel},
		Text:        Style{Fg: color.NordWhite, Bg: color.NordPanel},
		TextMuted:   Style{Fg: color.NordGray, Bg: color.NordPanel},
		Title:       Style{Fg: color.NordCyan, Bg: color.NordPanel, Attr: Bold},
		Positive:    Style{Fg: color.NordGreen, Bg: color.NordPanel, Attr: Bold},
		Negative:    Style{Fg: color.NordRed, Bg: color.NordPanel, Attr: Bold},
		Warning:     Style{Fg: color.NordAmber, Bg: color.NordPanel, Attr: Bold},
		Info:        Style{Fg: color.NordCyan, Bg: color.NordPanel},
		Selected:    Style{Fg: color.NordBackground, Bg: color.NordBlue, Attr: Bold},
		TrackFull:   Style{Fg: color.NordBlue, Bg: color.NordPanel},
		TrackEmpty:  Style{Fg: color.NordDimGray, Bg: color.NordPanel},
	}
}

func DraculaTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.DraculaWhite, Bg: color.DraculaBackground},
		Panel:       Style{Fg: color.DraculaWhite, Bg: color.DraculaPanel},
		Border:      Style{Fg: color.DraculaBorder, Bg: color.DraculaPanel},
		BorderFocus: Style{Fg: color.DraculaPink, Bg: color.DraculaPanel},
		Text:        Style{Fg: color.DraculaWhite, Bg: color.DraculaPanel},
		TextMuted:   Style{Fg: color.DraculaGray, Bg: color.DraculaPanel},
		Title:       Style{Fg: color.DraculaPink, Bg: color.DraculaPanel, Attr: Bold},
		Positive:    Style{Fg: color.DraculaGreen, Bg: color.DraculaPanel, Attr: Bold},
		Negative:    Style{Fg: color.DraculaRed, Bg: color.DraculaPanel, Attr: Bold},
		Warning:     Style{Fg: color.DraculaAmber, Bg: color.DraculaPanel, Attr: Bold},
		Info:        Style{Fg: color.DraculaCyan, Bg: color.DraculaPanel},
		Selected:    Style{Fg: color.DraculaBackground, Bg: color.DraculaPink, Attr: Bold},
		TrackFull:   Style{Fg: color.DraculaCyan, Bg: color.DraculaPanel},
		TrackEmpty:  Style{Fg: color.DraculaDimGray, Bg: color.DraculaPanel},
	}
}
func CatppuccinMochaTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.CatppuccinWhite, Bg: color.CatppuccinBackground},
		Panel:       Style{Fg: color.CatppuccinWhite, Bg: color.CatppuccinPanel},
		Border:      Style{Fg: color.CatppuccinBorder, Bg: color.CatppuccinPanel},
		BorderFocus: Style{Fg: color.CatppuccinMauve, Bg: color.CatppuccinPanel},
		Text:        Style{Fg: color.CatppuccinWhite, Bg: color.CatppuccinPanel},
		TextMuted:   Style{Fg: color.CatppuccinGray, Bg: color.CatppuccinPanel},
		Title:       Style{Fg: color.CatppuccinMauve, Bg: color.CatppuccinPanel, Attr: Bold},
		Positive:    Style{Fg: color.CatppuccinGreen, Bg: color.CatppuccinPanel, Attr: Bold},
		Negative:    Style{Fg: color.CatppuccinRed, Bg: color.CatppuccinPanel, Attr: Bold},
		Warning:     Style{Fg: color.CatppuccinAmber, Bg: color.CatppuccinPanel, Attr: Bold},
		Info:        Style{Fg: color.CatppuccinCyan, Bg: color.CatppuccinPanel},
		Selected:    Style{Fg: color.CatppuccinBackground, Bg: color.CatppuccinMauve, Attr: Bold},
		TrackFull:   Style{Fg: color.CatppuccinMauve, Bg: color.CatppuccinPanel},
		TrackEmpty:  Style{Fg: color.CatppuccinDimGray, Bg: color.CatppuccinPanel},
	}
}

func RosePineTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.RosePineWhite, Bg: color.RosePineBackground},
		Panel:       Style{Fg: color.RosePineWhite, Bg: color.RosePinePanel},
		Border:      Style{Fg: color.RosePineBorder, Bg: color.RosePinePanel},
		BorderFocus: Style{Fg: color.RosePineIris, Bg: color.RosePinePanel},
		Text:        Style{Fg: color.RosePineWhite, Bg: color.RosePinePanel},
		TextMuted:   Style{Fg: color.RosePineGray, Bg: color.RosePinePanel},
		Title:       Style{Fg: color.RosePineRose, Bg: color.RosePinePanel, Attr: Bold},
		Positive:    Style{Fg: color.RosePineCyan, Bg: color.RosePinePanel, Attr: Bold},
		Negative:    Style{Fg: color.RosePineRed, Bg: color.RosePinePanel, Attr: Bold},
		Warning:     Style{Fg: color.RosePineAmber, Bg: color.RosePinePanel, Attr: Bold},
		Info:        Style{Fg: color.RosePineIris, Bg: color.RosePinePanel},
		Selected:    Style{Fg: color.RosePineBackground, Bg: color.RosePineIris, Attr: Bold},
		TrackFull:   Style{Fg: color.RosePineIris, Bg: color.RosePinePanel},
		TrackEmpty:  Style{Fg: color.RosePineDimGray, Bg: color.RosePinePanel},
	}
}

func CyberpunkTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.CyberWhite, Bg: color.CyberBackground},
		Panel:       Style{Fg: color.CyberWhite, Bg: color.CyberPanel},
		Border:      Style{Fg: color.CyberBorder, Bg: color.CyberPanel},
		BorderFocus: Style{Fg: color.CyberCyan, Bg: color.CyberPanel},
		Text:        Style{Fg: color.CyberWhite, Bg: color.CyberPanel},
		TextMuted:   Style{Fg: color.CyberGray, Bg: color.CyberPanel},
		Title:       Style{Fg: color.CyberYellow, Bg: color.CyberPanel, Attr: Bold},
		Positive:    Style{Fg: color.CyberGreen, Bg: color.CyberPanel, Attr: Bold},
		Negative:    Style{Fg: color.CyberRed, Bg: color.CyberPanel, Attr: Bold},
		Warning:     Style{Fg: color.CyberAmber, Bg: color.CyberPanel, Attr: Bold},
		Info:        Style{Fg: color.CyberCyan, Bg: color.CyberPanel},
		Selected:    Style{Fg: color.CyberBackground, Bg: color.CyberYellow, Attr: Bold},
		TrackFull:   Style{Fg: color.CyberYellow, Bg: color.CyberPanel},
		TrackEmpty:  Style{Fg: color.CyberDimGray, Bg: color.CyberPanel},
	}
}

func AutumnTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.AutumnWhite, Bg: color.AutumnBackground},
		Panel:       Style{Fg: color.AutumnWhite, Bg: color.AutumnPanel},
		Border:      Style{Fg: color.AutumnBorder, Bg: color.AutumnPanel},
		BorderFocus: Style{Fg: color.AutumnAmber, Bg: color.AutumnPanel},
		Text:        Style{Fg: color.AutumnWhite, Bg: color.AutumnPanel},
		TextMuted:   Style{Fg: color.AutumnGray, Bg: color.AutumnPanel},
		Title:       Style{Fg: color.AutumnGold, Bg: color.AutumnPanel, Attr: Bold},
		Positive:    Style{Fg: color.AutumnGreen, Bg: color.AutumnPanel, Attr: Bold},
		Negative:    Style{Fg: color.AutumnRed, Bg: color.AutumnPanel, Attr: Bold},
		Warning:     Style{Fg: color.AutumnAmber, Bg: color.AutumnPanel, Attr: Bold},
		Info:        Style{Fg: color.AutumnCyan, Bg: color.AutumnPanel},
		Selected:    Style{Fg: color.AutumnBackground, Bg: color.AutumnAmber, Attr: Bold},
		TrackFull:   Style{Fg: color.AutumnAmber, Bg: color.AutumnPanel},
		TrackEmpty:  Style{Fg: color.AutumnDimGray, Bg: color.AutumnPanel},
	}
}

func SynthwaveTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.SynthWhite, Bg: color.SynthBackground},
		Panel:       Style{Fg: color.SynthWhite, Bg: color.SynthPanel},
		Border:      Style{Fg: color.SynthBorder, Bg: color.SynthPanel},
		BorderFocus: Style{Fg: color.SynthPink, Bg: color.SynthPanel},
		Text:        Style{Fg: color.SynthWhite, Bg: color.SynthPanel},
		TextMuted:   Style{Fg: color.SynthGray, Bg: color.SynthPanel},
		Title:       Style{Fg: color.SynthPink, Bg: color.SynthPanel, Attr: Bold},
		Positive:    Style{Fg: color.SynthGreen, Bg: color.SynthPanel, Attr: Bold},
		Negative:    Style{Fg: color.SynthRed, Bg: color.SynthPanel, Attr: Bold},
		Warning:     Style{Fg: color.SynthAmber, Bg: color.SynthPanel, Attr: Bold},
		Info:        Style{Fg: color.SynthCyan, Bg: color.SynthPanel},
		Selected:    Style{Fg: color.SynthBackground, Bg: color.SynthPink, Attr: Bold},
		TrackFull:   Style{Fg: color.SynthPink, Bg: color.SynthPanel},
		TrackEmpty:  Style{Fg: color.SynthDimGray, Bg: color.SynthPanel},
	}
}

func SolarizedLightTheme() *Theme {
	return &Theme{
		Background:  Style{Fg: color.SolarLightWhite, Bg: color.SolarLightBackground},
		Panel:       Style{Fg: color.SolarLightWhite, Bg: color.SolarLightPanel},
		Border:      Style{Fg: color.SolarLightBorder, Bg: color.SolarLightPanel},
		BorderFocus: Style{Fg: color.SolarLightBlue, Bg: color.SolarLightPanel},
		Text:        Style{Fg: color.SolarLightWhite, Bg: color.SolarLightPanel},
		TextMuted:   Style{Fg: color.SolarLightGray, Bg: color.SolarLightPanel},
		Title:       Style{Fg: color.SolarLightBlue, Bg: color.SolarLightPanel, Attr: Bold},
		Positive:    Style{Fg: color.SolarLightGreen, Bg: color.SolarLightPanel, Attr: Bold},
		Negative:    Style{Fg: color.SolarLightRed, Bg: color.SolarLightPanel, Attr: Bold},
		Warning:     Style{Fg: color.SolarLightAmber, Bg: color.SolarLightPanel, Attr: Bold},
		Info:        Style{Fg: color.SolarLightCyan, Bg: color.SolarLightPanel},
		Selected:    Style{Fg: color.SolarLightBackground, Bg: color.SolarLightBlue, Attr: Bold},
		TrackFull:   Style{Fg: color.SolarLightBlue, Bg: color.SolarLightPanel},
		TrackEmpty:  Style{Fg: color.SolarLightDimGray, Bg: color.SolarLightPanel},
	}
}

// Theme centralizes the named roles every stock widget draws with, so a whole application can be reskinned by swapping one struct.
type Theme struct {
	Background  Style
	Panel       Style
	Border      Style
	BorderFocus Style
	Text        Style
	TextMuted   Style
	Title       Style
	Positive    Style
	Negative    Style
	Warning     Style
	Info        Style
	Selected    Style
	TrackFull   Style
	TrackEmpty  Style
}
