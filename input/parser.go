package input

import (
	"bytes"
	"io"
	"time"
	"unicode/utf8"
)

// Reader accumulates raw bytes from an io.Reader (normally os.Stdin in raw mode) and decodes complete escape sequences before emitting events, so arrow keys and mouse reports never leak through as separate runes.
const (
	maxSGRMouseSequence = 128
	maxBracketedPaste   = 4 << 20
)

type Reader struct {
	src io.Reader
	buf []byte
	tmp [128]byte
}

func NewReader(src io.Reader) *Reader {
	return &Reader{src: src, buf: make([]byte, 0, 256)}
}

// Run blocks, pushing decoded events to out, until src.Read returns a non-recoverable error (typically because the app is shutting down and stdin was closed/restored).
func (r *Reader) Run(out chan<- Event) {
	for {
		n, err := r.src.Read(r.tmp[:])
		if err != nil || n == 0 {
			time.Sleep(2 * time.Millisecond)
			if err != nil {
				return
			}
			continue
		}
		r.buf = append(r.buf, r.tmp[:n]...)
		r.drain(out)
	}
}

func (r *Reader) drain(out chan<- Event) {
	for len(r.buf) > 0 {
		b0 := r.buf[0]

		if b0 == 0x1b {
			// Bracketed paste: ESC[200~ payload ESC[201~. Keep the payload as
			// one event so pasted newlines are never mistaken for Enter keys.
			const pasteStart = "\x1b[200~"
			if len(r.buf) >= len(pasteStart) && string(r.buf[:len(pasteStart)]) == pasteStart {
				const pasteEnd = "\x1b[201~"
				end := indexBytes(r.buf[len(pasteStart):], []byte(pasteEnd))
				if end < 0 {
					if len(r.buf) > len(pasteStart)+maxBracketedPaste {
						r.buf = r.buf[:0]
					}
					return
				}
				payloadEnd := len(pasteStart) + end
				payload := string(r.buf[len(pasteStart):payloadEnd])
				r.buf = r.buf[payloadEnd+len(pasteEnd):]
				out <- Event{Paste: payload}
				continue
			}
			// SGR mouse: ESC [ < ... (M|m)
			if len(r.buf) >= 3 && r.buf[1] == '[' && r.buf[2] == '<' {
				end := -1
				for j := 3; j < len(r.buf); j++ {
					if r.buf[j] == 'M' || r.buf[j] == 'm' {
						end = j
						break
					}
				}
				if end == -1 {
					if len(r.buf) >= maxSGRMouseSequence {
						// Never let a malformed/incomplete mouse sequence grow the
						// input buffer without bound. Drop the malformed sequence.
						r.buf = r.buf[:0]
					}
					return // wait for more bytes
				}
				ev, ok := decodeSGRMouse(r.buf[:end+1])
				r.buf = r.buf[end+1:]
				if ok {
					out <- ev
				}
				continue
			}
			// CSI keyboard reports. SGR mouse was handled above. Kitty /
			// progressive-enhancement keyboard mode uses CSI code;mods u.
			if len(r.buf) >= 3 && r.buf[1] == '[' {
				end := -1
				for j := 2; j < len(r.buf); j++ {
					if r.buf[j] >= 0x40 && r.buf[j] <= 0x7e {
						end = j
						break
					}
				}
				if end == -1 {
					if len(r.buf) > maxSGRMouseSequence {
						r.buf = r.buf[:0]
					}
					return
				}
				seq := r.buf[:end+1]
				if seq[end] == 'u' {
					if key, ok := decodeKittyKey(seq); ok {
						r.buf = r.buf[end+1:]
						out <- Event{Key: key}
						continue
					}
					// Malformed CSI-u: consume it as one unknown sequence.
					r.buf = r.buf[end+1:]
					continue
				}
				if key, ok := decodeCSIKey(seq); ok {
					r.buf = r.buf[end+1:]
					out <- Event{Key: key}
					continue
				}
				// Unknown CSI: consume the complete sequence instead of leaking
				// its parameter bytes as printable input.
				r.buf = r.buf[end+1:]
				continue
			}
			if len(r.buf) == 1 {
				return // wait for the rest of the sequence
			}
			r.buf = r.buf[1:]
			out <- Event{Key: Key{Type: KeyEsc}}
			continue
		}

		if b0 < utf8.RuneSelf {
			r.buf = r.buf[1:]
			out <- Event{Key: decodeRune(b0)}
			continue
		}
		if !utf8.FullRune(r.buf) {
			return // wait for the rest of an incomplete UTF-8 sequence
		}
		rn, size := utf8.DecodeRune(r.buf)
		if size == 1 && rn == utf8.RuneError {
			r.buf = r.buf[1:]
			out <- Event{Key: decodeRune(b0)}
			continue
		}
		r.buf = r.buf[size:]
		out <- Event{Key: Key{Type: KeyRune, Rune: rn}}
	}
}

func decodeRune(b byte) Key {
	switch b {
	case '\r', '\n':
		return Key{Type: KeyEnter}
	case '\t':
		return Key{Type: KeyTab}
	case 0x7f, 0x08:
		return Key{Type: KeyBackspace}
	case 0x03:
		return Key{Type: KeyCtrlC}
	case 0x00:
		// Ctrl+` is transmitted by traditional terminals as NUL. Preserve the
		// shortcut as a real Ctrl+` key instead of exposing an unhandled NUL rune.
		return Key{Type: KeyRune, Rune: '`', Mods: ModCtrl}
	case 0x10:
		return Key{Type: KeyCtrlP}
	case 0x17:
		return Key{Type: KeyCtrlW, Rune: 'w', Mods: ModCtrl}
	case 0x01, 0x02, 0x04, 0x05, 0x06, 0x07, 0x0b, 0x0c, 0x0e, 0x0f, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x18, 0x19, 0x1a:
		return Key{Type: KeyRune, Rune: rune(b + ('a' - 1)), Mods: ModCtrl}
	case ' ':
		return Key{Type: KeySpace, Rune: ' '}
	default:
		return Key{Type: KeyRune, Rune: rune(b)}
	}
}

// decodeSGRMouse parses ESC [ < Cb ; Cx ; Cy M/m directly from bytes.
// The input path is deliberately allocation-free: mouse drag can generate
// hundreds of reports per second and must never create a string or fmt parser
// object for each packet.
func decodeSGRMouse(seq []byte) (Event, bool) {
	if len(seq) < 7 || seq[0] != 0x1b || seq[1] != '[' || seq[2] != '<' {
		return Event{}, false
	}
	i := 3
	cb, ok := parseIntUntil(seq, &i, ';')
	if !ok {
		return Event{}, false
	}
	cx, ok := parseIntUntil(seq, &i, ';')
	if !ok {
		return Event{}, false
	}
	cy, ok := parseIntUntilTerm(seq, &i)
	if !ok {
		return Event{}, false
	}
	press := seq[len(seq)-1] == 'M'
	me := MouseEvent{X: cx - 1, Y: cy - 1}
	if cb&4 != 0 {
		me.Mods |= ModShift
	}
	if cb&8 != 0 {
		me.Mods |= ModAlt
	}
	if cb&16 != 0 {
		me.Mods |= ModCtrl
	}
	switch {
	case cb&64 != 0:
		if cb&1 != 0 {
			me.Action = MouseWheelDown
		} else {
			me.Action = MouseWheelUp
		}
	case cb&32 != 0:
		me.Action = MouseDrag
		me.Button = MouseButton(cb & 3)
	default:
		if press {
			me.Action = MousePress
		} else {
			me.Action = MouseRelease
		}
		me.Button = MouseButton(cb & 3)
	}
	return Event{IsMouse: true, Mouse: me}, true
}

func decodeCSIKey(seq []byte) (Key, bool) {
	if len(seq) < 3 || seq[0] != 0x1b || seq[1] != '[' {
		return Key{}, false
	}
	final := seq[len(seq)-1]
	mods := KeyModifier(0)
	// xterm modifier parameters are 1 + bitmask: 2=Shift, 3=Alt, 5=Ctrl.
	// Accept the common CSI 1;5A / 1;2B forms without allocating strings.
	paramEnd := len(seq) - 1
	start := paramEnd
	for i := 2; i < paramEnd; i++ {
		if seq[i] == ';' {
			start = i + 1
		}
	}
	if start < paramEnd {
		v, ok := parseDecimal(seq[start:paramEnd])
		if ok && v > 0 {
			mods = csiMods(v)
		}
	}
	var kt KeyType
	switch final {
	case 'A':
		kt = KeyUp
	case 'B':
		kt = KeyDown
	case 'C':
		kt = KeyRight
	case 'D':
		kt = KeyLeft
	case 'H':
		kt = KeyHome
	case 'F':
		kt = KeyEnd
	case 'Z':
		kt = KeyShiftTab
		mods |= ModShift
	case '~':
		v, ok := parseDecimal(seq[2:paramEnd])
		if !ok {
			return Key{}, false
		}
		switch v {
		case 3:
			kt = KeyDelete
		case 5:
			kt = KeyPageUp
		case 6:
			kt = KeyPageDown
		case 12:
			kt = KeyF2
		case 15:
			kt = KeyF5
		case 17:
			kt = KeyF6
		default:
			return Key{}, false
		}
	default:
		return Key{}, false
	}
	return Key{Type: kt, Mods: mods}, true
}

func parseDecimal(b []byte) (int, bool) {
	if len(b) == 0 {
		return 0, false
	}
	v := 0
	max := int(^uint(0) >> 1)
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int(c - '0')
		if v > (max-d)/10 {
			return 0, false
		}
		v = v*10 + d
	}
	return v, true
}

func csiMods(v int) KeyModifier {
	if v > 0 {
		v--
	}
	var m KeyModifier
	if v&1 != 0 {
		m |= ModShift
	}
	if v&2 != 0 {
		m |= ModAlt
	}
	if v&4 != 0 {
		m |= ModCtrl
	}
	if v&8 != 0 {
		m |= ModSuper
	}
	if v&16 != 0 {
		m |= ModHyper
	}
	if v&32 != 0 {
		m |= ModMeta
	}
	return m
}

func indexBytes(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	if len(haystack) < len(needle) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if bytes.Equal(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func decodeKittyKey(seq []byte) (Key, bool) {
	if len(seq) < 4 || seq[0] != 0x1b || seq[1] != '[' || seq[len(seq)-1] != 'u' {
		return Key{}, false
	}
	i := 2
	code, ok := parseIntUntil(seq, &i, ';')
	if !ok {
		// Kitty permits a single codepoint with the default modifier value.
		i = 2
		code, ok = parseIntUntilTermU(seq, &i)
		if !ok {
			return Key{}, false
		}
		return kittyCodeToKey(code, 0), true
	}
	mods, ok := parseIntUntilTermU(seq, &i)
	if !ok || code <= 0 || code > utf8.MaxRune {
		return Key{}, false
	}
	// Kitty encodes modifiers as 1 + bitmask: 2=Shift, 3=Alt, 5=Ctrl, etc.
	if mods > 1 {
		mods--
	} else {
		mods = 0
	}
	return kittyCodeToKey(code, KeyModifier(mods)), true
}

func parseIntUntilTermU(seq []byte, i *int) (int, bool) {
	v, start := 0, *i
	for *i < len(seq) {
		b := seq[*i]
		if b == 'u' {
			return v, *i > start
		}
		if b < '0' || b > '9' {
			return 0, false
		}
		v = v*10 + int(b-'0')
		(*i)++
	}
	return 0, false
}

func kittyCodeToKey(code int, mods KeyModifier) Key {
	k := Key{Type: KeyRune, Rune: rune(code), Mods: mods}
	switch code {
	case 9:
		k.Type = KeyTab
		if mods&ModShift != 0 {
			k.Type = KeyShiftTab
		}
	case 13:
		k.Type = KeyEnter
	case 27:
		k.Type = KeyEsc
	case 127:
		k.Type = KeyBackspace
	case 57352:
		k.Type = KeyUp
	case 57353:
		k.Type = KeyDown
	case 57354:
		k.Type = KeyRight
	case 57355:
		k.Type = KeyLeft
	case 57356:
		k.Type = KeyHome
	case 57357:
		k.Type = KeyEnd
	case 57358:
		k.Type = KeyPageUp
	case 57359:
		k.Type = KeyPageDown
	case 57360:
		k.Type = KeyDelete
	}
	if mods&ModCtrl != 0 {
		switch code {
		case 'c', 'C':
			k.Type = KeyCtrlC
		case 'p', 'P':
			k.Type = KeyCtrlP
		}
	}
	return k
}

func parseIntUntil(seq []byte, i *int, delim byte) (int, bool) {
	v := 0
	start := *i
	for *i < len(seq) {
		b := seq[*i]
		if b == delim {
			(*i)++
			return v, *i-1 > start
		}
		if b < '0' || b > '9' {
			return 0, false
		}
		d := int(b - '0')
		if v > (int(^uint(0)>>1)-d)/10 {
			return 0, false
		}
		v = v*10 + d
		(*i)++
	}
	return 0, false
}

func parseIntUntilTerm(seq []byte, i *int) (int, bool) {
	v := 0
	start := *i
	for *i < len(seq) {
		b := seq[*i]
		if b == 'M' || b == 'm' {
			return v, *i > start
		}
		if b < '0' || b > '9' {
			return 0, false
		}
		d := int(b - '0')
		if v > (int(^uint(0)>>1)-d)/10 {
			return 0, false
		}
		v = v*10 + d
		(*i)++
	}
	return 0, false
}
