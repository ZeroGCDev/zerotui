/*
Package color provides a 24-bit truecolor type used throughout zerotui.

Colors are plain uint32 values (no heap allocation, no pointer chasing), which keeps every widget's style state inline and GC-invisible.
*/
package color

// Color packs 0xRRGGBB. Default means "inherit terminal default colour".
type Color uint32

const Default Color = 0xFFFFFFFF

// RGB constructs a truecolor value.
func RGB(r, g, b uint8) Color {
	return Color(uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// Components returns the individual channels.
func (c Color) Components() (r, g, b uint8) {
	return uint8(c >> 16), uint8(c >> 8), uint8(c)
}

// Lerp blends two colors linearly, t in [0,1] scaled to [0,255].
func Lerp(a, b Color, t255 uint8) Color {
	ar, ag, ab := a.Components()
	br, bg, bb := b.Components()
	l := func(x, y uint8) uint8 {
		return uint8((int(x)*(255-int(t255)) + int(y)*int(t255)) / 255)
	}
	return RGB(l(ar, br), l(ag, bg), l(ab, bb))
}

// Standard palette
var (
	Black       = RGB(10, 10, 12)
	Background  = RGB(14, 16, 20)
	Panel       = RGB(20, 23, 28)
	White       = RGB(235, 235, 240)
	Gray        = RGB(140, 145, 155)
	DimGray     = RGB(50, 55, 65)
	Border      = RGB(48, 54, 64)
	Green       = RGB(38, 201, 132)
	GreenDim    = RGB(20, 110, 78)
	GreenDimest = RGB(10, 60, 48)
	Red         = RGB(235, 78, 92)
	RedDim      = RGB(120, 45, 55)
	Amber       = RGB(255, 176, 32)
	Cyan        = RGB(56, 199, 224)
	Blue        = RGB(88, 141, 224)
	Magenta     = RGB(196, 106, 224)
	Yellow      = RGB(230, 210, 60)
)

// Tokyo Night
var (
	TokyoBackground = RGB(26, 27, 38)
	TokyoPanel      = RGB(36, 40, 59)
	TokyoBorder     = RGB(41, 46, 66)
	TokyoWhite      = RGB(192, 202, 245)
	TokyoGray       = RGB(169, 177, 214)
	TokyoDimGray    = RGB(86, 95, 137)
	TokyoGreen      = RGB(158, 206, 106)
	TokyoRed        = RGB(247, 118, 142)
	TokyoAmber      = RGB(255, 158, 100)
	TokyoCyan       = RGB(125, 207, 255)
	TokyoBlue       = RGB(122, 162, 247)
)

// Matcha Latte
var (
	MatchaBackground = RGB(42, 49, 40)
	MatchaPanel      = RGB(53, 62, 50)
	MatchaBorder     = RGB(68, 78, 65)
	MatchaWhite      = RGB(236, 239, 230)
	MatchaGray       = RGB(160, 170, 155)
	MatchaDimGray    = RGB(85, 95, 80)
	MatchaGreen      = RGB(163, 190, 140)
	MatchaRed        = RGB(216, 114, 114)
	MatchaAmber      = RGB(229, 169, 112)
	MatchaCyan       = RGB(143, 188, 187)
	MatchaBlue       = RGB(129, 161, 193)
)

// Vaporwave
var (
	VaporBackground = RGB(26, 14, 38)
	VaporPanel      = RGB(42, 20, 56)
	VaporBorder     = RGB(70, 35, 95)
	VaporWhite      = RGB(240, 230, 255)
	VaporGray       = RGB(170, 145, 190)
	VaporDimGray    = RGB(85, 65, 100)
	VaporGreen      = RGB(46, 230, 166)
	VaporRed        = RGB(255, 85, 114)
	VaporAmber      = RGB(255, 170, 51)
	VaporCyan       = RGB(5, 217, 232)
	VaporBlue       = RGB(43, 146, 222)
)

// Mocha Espresso Palette (Warm Monochrome & Latte)
var (
	MochaBackground = RGB(33, 28, 25)
	MochaPanel      = RGB(45, 38, 34)
	MochaBorder     = RGB(65, 55, 50)
	MochaWhite      = RGB(242, 233, 225)
	MochaGray       = RGB(175, 160, 150)
	MochaDimGray    = RGB(90, 80, 75)
	MochaGreen      = RGB(140, 175, 125)
	MochaRed        = RGB(215, 100, 90)
	MochaAmber      = RGB(225, 145, 75)
	MochaCyan       = RGB(130, 175, 185)
	MochaBlue       = RGB(110, 150, 180)
)

// Deep Abyss Palette (OLED Minimalist)
var (
	AbyssBackground = RGB(10, 12, 16)
	AbyssPanel      = RGB(16, 20, 26)
	AbyssBorder     = RGB(30, 36, 46)
	AbyssWhite      = RGB(230, 235, 245)
	AbyssGray       = RGB(130, 140, 155)
	AbyssDimGray    = RGB(45, 52, 64)
	AbyssGreen      = RGB(42, 220, 140)
	AbyssRed        = RGB(245, 68, 85)
	AbyssAmber      = RGB(255, 160, 0)
	AbyssCyan       = RGB(0, 210, 255)
	AbyssBlue       = RGB(75, 145, 255)
)

// Nord Palette (Arctic Dark)
var (
	NordBackground = RGB(46, 52, 64)
	NordPanel      = RGB(59, 66, 82)
	NordBorder     = RGB(67, 76, 94)
	NordWhite      = RGB(236, 239, 244)
	NordGray       = RGB(216, 222, 233)
	NordDimGray    = RGB(76, 86, 106)
	NordGreen      = RGB(163, 190, 140)
	NordRed        = RGB(191, 97, 106)
	NordAmber      = RGB(208, 135, 112)
	NordCyan       = RGB(136, 192, 208)
	NordBlue       = RGB(129, 161, 193)
)

// Dracula Palette
var (
	DraculaBackground = RGB(40, 42, 54)
	DraculaPanel      = RGB(68, 71, 90)
	DraculaBorder     = RGB(98, 114, 164)
	DraculaWhite      = RGB(248, 248, 242)
	DraculaGray       = RGB(189, 147, 249)
	DraculaDimGray    = RGB(98, 114, 164)
	DraculaGreen      = RGB(80, 250, 123)
	DraculaRed        = RGB(255, 85, 85)
	DraculaAmber      = RGB(255, 184, 108)
	DraculaCyan       = RGB(139, 233, 253)
	DraculaPink       = RGB(255, 121, 198)
)

// Catppuccin Mocha Palette (Soothing Warm Dark)
var (
	CatppuccinBackground = RGB(30, 30, 46)
	CatppuccinPanel      = RGB(24, 24, 37)
	CatppuccinBorder     = RGB(69, 71, 90)
	CatppuccinWhite      = RGB(205, 214, 244)
	CatppuccinGray       = RGB(166, 173, 200)
	CatppuccinDimGray    = RGB(88, 91, 112)
	CatppuccinGreen      = RGB(166, 227, 161)
	CatppuccinRed        = RGB(243, 139, 168)
	CatppuccinAmber      = RGB(250, 179, 135)
	CatppuccinCyan       = RGB(148, 226, 213)
	CatppuccinBlue       = RGB(137, 180, 250)
	CatppuccinMauve      = RGB(203, 166, 247)
)

// Rose Pine Palette (Dreamy Soft Vintage)
var (
	RosePineBackground = RGB(25, 23, 36)
	RosePinePanel      = RGB(31, 29, 46)
	RosePineBorder     = RGB(82, 79, 103)
	RosePineWhite      = RGB(224, 222, 244)
	RosePineGray       = RGB(144, 140, 170)
	RosePineDimGray    = RGB(110, 106, 134)
	RosePineGreen      = RGB(49, 116, 143)
	RosePineRed        = RGB(235, 111, 146)
	RosePineAmber      = RGB(246, 193, 119)
	RosePineCyan       = RGB(156, 207, 216)
	RosePineRose       = RGB(235, 188, 186)
	RosePineIris       = RGB(196, 167, 231)
)

// Cyberpunk 2077 Palette (High Voltage Neon Yellow/Cyan)
var (
	CyberBackground = RGB(16, 16, 22)
	CyberPanel      = RGB(26, 26, 36)
	CyberBorder     = RGB(255, 241, 0)
	CyberWhite      = RGB(230, 245, 255)
	CyberGray       = RGB(140, 160, 180)
	CyberDimGray    = RGB(60, 70, 85)
	CyberGreen      = RGB(0, 255, 159)
	CyberRed        = RGB(255, 0, 85)
	CyberAmber      = RGB(255, 119, 0)
	CyberCyan       = RGB(0, 232, 255)
	CyberYellow     = RGB(255, 241, 0)
)

// Autumn Leaves Palette (Cozy Pumpkin & Forest Warmth)
var (
	AutumnBackground = RGB(34, 26, 23)
	AutumnPanel      = RGB(46, 36, 32)
	AutumnBorder     = RGB(75, 58, 50)
	AutumnWhite      = RGB(245, 232, 216)
	AutumnGray       = RGB(180, 158, 142)
	AutumnDimGray    = RGB(95, 80, 70)
	AutumnGreen      = RGB(135, 154, 107)
	AutumnRed        = RGB(196, 77, 55)
	AutumnAmber      = RGB(224, 122, 58)
	AutumnCyan       = RGB(143, 168, 160)
	AutumnGold       = RGB(224, 172, 74)
)

// Synthwave Horizon Palette (Sunset Purples & Hot Pink)
var (
	SynthBackground = RGB(20, 14, 34)
	SynthPanel      = RGB(32, 20, 52)
	SynthBorder     = RGB(84, 38, 110)
	SynthWhite      = RGB(242, 235, 255)
	SynthGray       = RGB(165, 140, 190)
	SynthDimGray    = RGB(80, 55, 100)
	SynthGreen      = RGB(50, 240, 180)
	SynthRed        = RGB(255, 60, 110)
	SynthAmber      = RGB(255, 140, 50)
	SynthCyan       = RGB(40, 220, 255)
	SynthPink       = RGB(255, 45, 165)
)

// Solarized Light Palette (Clean Professional Day Theme)
var (
	SolarLightBackground = RGB(253, 246, 227)
	SolarLightPanel      = RGB(238, 232, 213)
	SolarLightBorder     = RGB(203, 194, 168)
	SolarLightWhite      = RGB(88, 110, 117) // Main text is dark in light themes
	SolarLightGray       = RGB(147, 161, 161)
	SolarLightDimGray    = RGB(181, 137, 0)
	SolarLightGreen      = RGB(133, 153, 0)
	SolarLightRed        = RGB(220, 50, 47)
	SolarLightAmber      = RGB(203, 75, 22)
	SolarLightCyan       = RGB(42, 161, 152)
	SolarLightBlue       = RGB(38, 139, 210)
)
