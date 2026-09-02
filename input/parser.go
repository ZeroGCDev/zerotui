package input

import (
	"io"
	"time"
	"unicode/utf8"
)

// Reader accumulates raw bytes from an io.Reader (normally os.Stdin in raw mode) and decodes complete escape sequences before emitting events, so arrow keys and mouse reports never leak through as separate runes.
const maxSGRMouseSequence = 128

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
				if end == 2 {
					k := seq[2]
					var kt KeyType
					switch k {
					case 'A':
						kt = KeyUp
					case 'B':
						kt = KeyDown
					case 'C':
						kt = KeyRight
					case 'D':
						kt = KeyLeft
					case 'Z':
						kt = KeyShiftTab
					default:
						r.buf = r.buf[end+1:]
						continue
					}
					r.buf = r.buf[end+1:]
					out <- Event{Key: Key{Type: kt}}
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
	case 0x10:
		return Key{Type: KeyCtrlP}
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
