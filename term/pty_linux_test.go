//go:build linux

package term

import (
	"bytes"
	"testing"
	"time"
)

func TestPTYSessionInteractiveShell(t *testing.T) {
	s, err := StartPTY("/bin/sh", ".", 80, 24, []string{"PS1=zerotui$ "})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Write([]byte("printf 'ZEROTUI_PTY_OK\\n'\n")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	deadline := time.After(2 * time.Second)
	var got []byte
	for !bytes.Contains(got, []byte("ZEROTUI_PTY_OK")) {
		select {
		case <-deadline:
			t.Fatalf("did not receive shell output: %q", got)
		default:
		}
		n, err := s.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if err != nil && !bytes.Contains(got, []byte("ZEROTUI_PTY_OK")) {
			t.Fatal(err)
		}
	}
}
