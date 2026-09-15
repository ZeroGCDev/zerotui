//go:build !linux && !darwin

package term

import "fmt"

type PTYSession struct{}

func StartPTY(shell, cwd string, w, h int, env []string) (*PTYSession, error) {
	return nil, fmt.Errorf("proper PTY sessions are currently supported on Linux and macOS; %s has no supported PTY backend", osName)
}
func (p *PTYSession) Read([]byte) (int, error)  { return 0, fmt.Errorf("PTY unsupported") }
func (p *PTYSession) Write([]byte) (int, error) { return 0, fmt.Errorf("PTY unsupported") }
func (p *PTYSession) Resize(int, int) error     { return fmt.Errorf("PTY unsupported") }
func (p *PTYSession) Close() error              { return nil }
func (p *PTYSession) Wait() error               { return nil }
func (p *PTYSession) PID() int                  { return 0 }

var osName = "this platform"
