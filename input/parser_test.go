package input

import "testing"

func TestDecodeSGRMouse(t *testing.T) {
	cases := []struct {
		name   string
		seq    []byte
		x, y   int
		action MouseAction
	}{
		{"press", []byte("\x1b[<0;12;8M"), 11, 7, MousePress},
		{"release", []byte("\x1b[<0;12;8m"), 11, 7, MouseRelease},
		{"drag", []byte("\x1b[<32;13;9M"), 12, 8, MouseDrag},
		{"wheel-up", []byte("\x1b[<64;13;9M"), 12, 8, MouseWheelUp},
		{"wheel-down", []byte("\x1b[<65;13;9M"), 12, 8, MouseWheelDown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, ok := decodeSGRMouse(tc.seq)
			if !ok || !ev.IsMouse || ev.Mouse.X != tc.x || ev.Mouse.Y != tc.y || ev.Mouse.Action != tc.action {
				t.Fatalf("decode=%+v ok=%v", ev, ok)
			}
		})
	}
}

func BenchmarkDecodeSGRMouse(b *testing.B) {
	seq := []byte("\x1b[<32;120;40M")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := decodeSGRMouse(seq); !ok {
			b.Fatal("decode failed")
		}
	}
}

func TestDecodeKittyKey(t *testing.T) {
	cases := []struct {
		name     string
		seq      string
		wantType KeyType
		wantRune rune
		wantMods KeyModifier
	}{
		{"ctrl-c", "\x1b[99;5u", KeyCtrlC, 'c', ModCtrl},
		{"shift-tab", "\x1b[9;2u", KeyShiftTab, '\t', ModShift},
		{"alt-a", "\x1b[97;3u", KeyRune, 'a', ModAlt},
		{"up", "\x1b[57352u", KeyUp, rune(57352), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := decodeKittyKey([]byte(tc.seq))
			if !ok || got.Type != tc.wantType || got.Rune != tc.wantRune || got.Mods != tc.wantMods {
				t.Fatalf("decode=%+v ok=%v", got, ok)
			}
		})
	}
}

func TestReaderParsesKittyKey(t *testing.T) {
	r := NewReader(nil)
	r.buf = append(r.buf, []byte("\x1b[97;5u")...)
	out := make(chan Event, 1)
	r.drain(out)
	got := <-out
	if got.Key.Type != KeyRune || got.Key.Rune != 'a' || got.Key.Mods != ModCtrl {
		t.Fatalf("event=%+v", got)
	}
}
