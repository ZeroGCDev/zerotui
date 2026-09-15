//go:build linux

package term

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const (
	tiocgwinsz  = 0x5413
	tiocswinsz  = 0x5414
	tiocgptn    = 0x80045430
	tiocspctlck = 0x40045431
)

type PTYSession struct {
	file      *os.File
	cmd       *exec.Cmd
	closeOnce sync.Once
}

func StartPTY(shell, cwd string, w, h int, env []string) (*PTYSession, error) {
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	f, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	unlock := 0
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), tiocspctlck, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		f.Close()
		return nil, fmt.Errorf("unlock pty: %w", e)
	}
	var n uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), tiocgptn, uintptr(unsafe.Pointer(&n))); e != 0 {
		f.Close()
		return nil, fmt.Errorf("pty number: %w", e)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		f.Close()
		return nil, err
	}
	cmd := exec.Command(shell)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		slave.Close()
		f.Close()
		return nil, err
	}
	slave.Close()
	s := &PTYSession{file: f, cmd: cmd}
	if err := s.Resize(w, h); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}
func (p *PTYSession) Read(b []byte) (int, error) {
	if p == nil || p.file == nil {
		return 0, os.ErrClosed
	}
	return p.file.Read(b)
}

func (p *PTYSession) Write(b []byte) (int, error) {
	if p == nil || p.file == nil {
		return 0, os.ErrClosed
	}
	total := len(b)
	for len(b) > 0 {
		n, err := p.file.Write(b)
		if n > 0 {
			b = b[n:]
		}
		if err != nil {
			return total - len(b), err
		}
		if n == 0 {
			return total - len(b), io.ErrShortWrite
		}
	}
	return total, nil
}
func (p *PTYSession) Resize(w, h int) error {
	if w < 1 || h < 1 {
		return nil
	}
	if w > 65535 || h > 65535 {
		return fmt.Errorf("pty size out of range: %dx%d", w, h)
	}
	ws := struct{ Rows, Cols, X, Y uint16 }{uint16(h), uint16(w), 0, 0}
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, p.file.Fd(), tiocswinsz, uintptr(unsafe.Pointer(&ws)))
	if e != 0 {
		return e
	}
	return nil
}
func (p *PTYSession) Close() error {
	if p == nil {
		return nil
	}
	var err error
	p.closeOnce.Do(func() {
		if p.cmd != nil && p.cmd.Process != nil {
			// The shell is a session leader. Hang up the whole process group so
			// children (editors, pagers, REPLs) cannot survive the widget.
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGHUP)
		}
		if p.file != nil {
			err = p.file.Close()
		}
	})
	return err
}
func (p *PTYSession) Wait() error {
	if p == nil || p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}
func (p *PTYSession) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
