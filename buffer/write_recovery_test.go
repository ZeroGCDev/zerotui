package buffer

import (
	"errors"
	"testing"

	"github.com/ZeroGCDev/zerotui/style"
)

type failWriter struct {
	limit int
	err   error
}

func (w *failWriter) Write(p []byte) (int, error) {
	if w.limit < len(p) {
		return w.limit, w.err
	}
	return len(p), nil
}

func TestShortWriteForcesRecovery(t *testing.T) {
	b := New(8, 1)
	st := style.Style{}
	b.SetString(0, 0, "ABCDEFGH", st)

	fw := &failWriter{limit: 2, err: errors.New("terminal failed")}
	if _, err := b.Render(fw); err == nil {
		t.Fatal("expected write error")
	}

	// The next render must conservatively repaint even though the front buffer
	// was partially synchronized before the failed write.
	b.Set(7, 0, 'Z', st)
	var out captureWriter
	if _, err := b.Render(&out); err != nil {
		t.Fatalf("recovery render: %v", err)
	}
	if len(out.b) == 0 {
		t.Fatal("expected recovery repaint")
	}
	out.b = out.b[:0]
	if _, err := b.Render(&out); err != nil {
		t.Fatalf("steady render: %v", err)
	}
	if len(out.b) != 0 {
		t.Fatal("expected no output after successful recovery")
	}
}
