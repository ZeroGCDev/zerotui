package input

import (
	"testing"
)

func TestUTF8RuneDecoding(t *testing.T) {
	r := &Reader{buf: []byte("A€界")}
	out := make(chan Event, 4)
	r.drain(out)
	want := []rune{'A', '€', '界'}
	for _, w := range want {
		select {
		case ev := <-out:
			if ev.Key.Type != KeyRune || ev.Key.Rune != w {
				t.Fatalf("got %+v, want rune %q", ev.Key, w)
			}
		default:
			t.Fatalf("missing rune %q", w)
		}
	}
}

func TestMalformedMouseSequenceIsBounded(t *testing.T) {
	r := &Reader{buf: make([]byte, maxSGRMouseSequence)}
	r.buf[0], r.buf[1], r.buf[2] = 0x1b, '[', '<'
	for i := 3; i < len(r.buf); i++ {
		r.buf[i] = '9'
	}
	out := make(chan Event, 1)
	r.drain(out)
	if len(r.buf) != 0 {
		t.Fatalf("malformed mouse buffer was not bounded/discarded: %d", len(r.buf))
	}
}

func TestMouseIntegerOverflowRejected(t *testing.T) {
	seq := []byte("\x1b[<999999999999999999999999;1;1M")
	if _, ok := decodeSGRMouse(seq); ok {
		t.Fatal("expected oversized mouse integer to be rejected")
	}
}

func FuzzDecodeSGRMouse(f *testing.F) {
	f.Add([]byte("\x1b[<0;1;1M"))
	f.Add([]byte("\x1b[<64;10;20m"))
	f.Add([]byte("\x1b[<999999999999;1;1M"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodeSGRMouse(data)
	})
}
