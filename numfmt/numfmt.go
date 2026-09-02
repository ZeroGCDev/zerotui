/*
Package numfmt formats integers and fixed-point values by appending to a caller-owned []byte, exactly like strconv.AppendInt.

Widgets keep a small reusable scratch buffer (dst[:0]) so redrawing a price/PNL every frame never touches the allocator, unlike fmt.Sprintf.
*/
package numfmt

// AppendUint appends the base-10 digits of v to dst.
func AppendUint(dst []byte, v uint64) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var tmp [20]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, tmp[i:]...)
}

// AppendInt appends a signed decimal integer to dst.
func AppendInt(dst []byte, v int64) []byte {
	if v < 0 {
		dst = append(dst, '-')
		v = -v
	}
	return AppendUint(dst, uint64(v))
}

/* AppendFixed appends v (an integer scaled by 10^decimals) as a decimal number with exactly `decimals` fraction digits, e.g. AppendFixed(dst,7890012345, 9) with a 1e9 price scale renders "7.890012345" */
func AppendFixed(dst []byte, v uint64, decimals int) []byte {
	scale := pow10(decimals)
	intPart := v / scale
	frac := v % scale
	dst = AppendUint(dst, intPart)
	if decimals == 0 {
		return dst
	}
	dst = append(dst, '.')
	divisor := scale / 10
	for divisor > 0 {
		digit := (frac / divisor) % 10
		dst = append(dst, byte('0'+digit))
		divisor /= 10
	}
	return dst
}

// AppendFixedPrec is like AppendFixed but only prints `show` of the available fraction digits (rounded down), useful for compact price tickers where 9-digit scaled prices only need 2-4 visible decimals
func AppendFixedPrec(dst []byte, v uint64, decimals, show int) []byte {
	if show >= decimals {
		return AppendFixed(dst, v, decimals)
	}
	drop := pow10(decimals - show)
	return AppendFixed(dst, v/drop, show)
}

// AppendPadLeft left-pads the just-appended run of `width` bytes starting at the original length of dst, using `pad`. Call BEFORE appending is not how this works - instead use PadLeft on an already-rendered numeric tail.
func PadLeft(dst []byte, from int, width int, pad byte) []byte {
	n := len(dst) - from
	if n >= width {
		return dst
	}
	need := width - n
	dst = append(dst, make([]byte, need)...)
	copy(dst[from+need:], dst[from:from+n])
	for i := 0; i < need; i++ {
		dst[from+i] = pad
	}
	return dst
}

func pow10(n int) uint64 {
	p := uint64(1)
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}
