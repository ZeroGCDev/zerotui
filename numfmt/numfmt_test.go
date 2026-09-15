package numfmt

import "testing"

func TestAppendUint(t *testing.T) {
	cases := map[uint64]string{0: "0", 7: "7", 42: "42", 100000: "100000"}
	for in, want := range cases {
		got := string(AppendUint(nil, in))
		if got != want {
			t.Errorf("AppendUint(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestAppendInt(t *testing.T) {
	if got := string(AppendInt(nil, -42)); got != "-42" {
		t.Errorf("AppendInt(-42) = %q", got)
	}
	if got := string(AppendInt(nil, 42)); got != "42" {
		t.Errorf("AppendInt(42) = %q", got)
	}
}

func TestAppendFixed(t *testing.T) {
	// 78900.123456789 at 9-digit scale, matching the reference prototype's PriceScale = 1_000_000_000.
	v := uint64(78900_123456789)
	got := string(AppendFixed(nil, v, 9))
	want := "78900.123456789"
	if got != want {
		t.Errorf("AppendFixed = %q, want %q", got, want)
	}
}

func TestAppendFixedPrec(t *testing.T) {
	v := uint64(78900_123456789)
	got := string(AppendFixedPrec(nil, v, 9, 2))
	want := "78900.12"
	if got != want {
		t.Errorf("AppendFixedPrec = %q, want %q", got, want)
	}
}

func BenchmarkAppendFixedPrec(b *testing.B) {
	var scratch [32]byte
	v := uint64(78900_123456789)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AppendFixedPrec(scratch[:0], v, 9, 2)
	}
	// for i := 0; i < b.N; i++ {
	// 	_ = AppendFixedPrec(scratch[:0], v, 9, 2)
	// }
}
