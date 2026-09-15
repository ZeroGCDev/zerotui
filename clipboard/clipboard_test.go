package clipboard

import "testing"

func TestProcessLocalCopyPaste(t *testing.T) {
	want := "zero tui\nclipboard"
	if !Copy(want) {
		t.Fatal("Copy returned false")
	}
	got, ok := Paste()
	if !ok || got != want {
		t.Fatalf("Paste() = %q, %v; want %q, true", got, ok, want)
	}
}
