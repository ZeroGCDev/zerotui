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

func TestMalformedUTF8AndExtremeCSIInput(t *testing.T) {
	cases := [][]byte{
		{0xff, 0xfe, 0xfd},
		[]byte("\x1b[<;\x1b[<999999999999999999999999;999999999999999999M"),
		[]byte("\x1b[<0;0;0"),
		[]byte("\x1b[<64;999999999;999999999M"),
	}
	for _, data := range cases {
		r := &Reader{buf: append([]byte(nil), data...)}
		out := make(chan Event, 16)
		r.drain(out)
		if len(r.buf) > maxSGRMouseSequence {
			t.Fatalf("input buffer grew beyond bound: %d", len(r.buf))
		}
	}
}

func TestReaderBracketedPaste(t *testing.T) {
	r := &Reader{buf: []byte("\x1b[200~hello\nworld\x1b[201~x")}
	out := make(chan Event, 2)
	r.drain(out)
	got := <-out
	if got.Paste != "hello\nworld" {
		t.Fatalf("paste=%q", got.Paste)
	}
	if ev := <-out; ev.Key.Rune != 'x' {
		t.Fatalf("next=%+v", ev)
	}
}

func TestDecodeCSIModifiers(t *testing.T) {
	for _, tc := range []struct {
		seq  string
		typ  KeyType
		mods KeyModifier
	}{
		{"\x1b[1;2A", KeyUp, ModShift},
		{"\x1b[1;3B", KeyDown, ModAlt},
		{"\x1b[1;5C", KeyRight, ModCtrl},
		{"\x1b[1;9D", KeyLeft, ModSuper},
	} {
		k, ok := decodeCSIKey([]byte(tc.seq))
		if !ok || k.Type != tc.typ || k.Mods != tc.mods {
			t.Fatalf("%q => %+v %v", tc.seq, k, ok)
		}
	}
}

func TestDecodeMouseModifiers(t *testing.T) {
	ev, ok := decodeSGRMouse([]byte("\x1b[<28;2;3M")) // button 0 + shift/alt/ctrl
	if !ok || ev.Mouse.Mods != (ModShift|ModAlt|ModCtrl) {
		t.Fatalf("%+v %v", ev, ok)
	}
}
