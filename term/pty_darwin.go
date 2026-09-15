//go:build darwin

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

// Darwin's native PTY API is exposed through /dev/ptmx and the TIOCPTY*
// ioctls. Keeping this backend in the term package avoids cgo and gives the
// terminal the same persistent master/slave PTY semantics as Linux.
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

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("open ptmx: %w", err)
	}
	fail := func(err error) (*PTYSession, error) {
		_ = master.Close()
		return nil, err
	}

	if err := darwinPTYIoctl(master.Fd(), syscall.TIOCPTYGRANT, nil); err != nil {
		return fail(fmt.Errorf("grant pty: %w", err))
	}
	if err := darwinPTYIoctl(master.Fd(), syscall.TIOCPTYUNLK, nil); err != nil {
		return fail(fmt.Errorf("unlock pty: %w", err))
	}

	var name [256]byte
	if err := darwinPTYIoctl(master.Fd(), syscall.TIOCPTYGNAME, unsafe.Pointer(&name[0])); err != nil {
		return fail(fmt.Errorf("get slave name: %w", err))
	}
	slaveName := cString(name[:])
	if slaveName == "" {
		return fail(fmt.Errorf("pty slave name is empty"))
	}
	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return fail(fmt.Errorf("open pty slave: %w", err))
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
		_ = slave.Close()
		return fail(fmt.Errorf("start shell: %w", err))
	}
	_ = slave.Close()

	s := &PTYSession{file: master, cmd: cmd}
	if err := s.Resize(w, h); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("resize pty: %w", err)
	}
	return s, nil
}

func darwinPTYIoctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
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
	return darwinPTYIoctl(p.file.Fd(), syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
}
func (p *PTYSession) Close() error {
	if p == nil {
		return nil
	}
	var err error
	p.closeOnce.Do(func() {
		if p.cmd != nil && p.cmd.Process != nil {
			// The shell is the session leader; hang up the whole process group
			// so child programs cannot outlive the terminal widget.
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
