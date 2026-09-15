package buffer

import "unicode"

// RuneWidth reports the terminal cell width used by ZeroTUI's buffer.
// Zero-width formatting/combining code points are treated conservatively as
// one cell because Buffer.Cell stores a single rune and does not implement
// grapheme-cluster composition. Wide CJK and emoji code points occupy two cells.
func RuneWidth(r rune) int {
	// ASCII is by far the dominant path in editor/source rendering. Keep it
	// branch-cheap and avoid the Unicode property tables for ordinary text.
	if r < 0x80 {
		if r == 0 || r < 0x20 || r == 0x7f {
			return 0
		}
		return 1
	}
	if unicode.IsControl(r) {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

func isWideRune(r rune) bool {
	return r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		r >= 0x2e80 && r <= 0x303e || r >= 0x3040 && r <= 0xa4cf ||
		r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff ||
		r >= 0xfe10 && r <= 0xfe19 || r >= 0xfe30 && r <= 0xfe6f ||
		r >= 0xff00 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 ||
		r >= 0x1f000 && r <= 0x1faff || r >= 0x20000 && r <= 0x3fffd)
}
