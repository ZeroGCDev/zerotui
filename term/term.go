/* Package term handles raw-mode setup, terminal size queries, and the alt-screen/mouse/cursor control sequences. It shells out to `stty` (present on every Linux/macOS box zerotui targets) instead of taking on a cgo or golang.org/x/sys dependency, matching the original prototype's proven portable approach. */
package term

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// EnableRaw puts the terminal into raw mode, enters the alt screen, hides the cursor and enables SGR mouse reporting. It returns a restore func that must be called (typically via defer) before the process exits.
func EnableRaw() func() {
	run("stty", "raw", "-echo")
	EnterAltScreen()
	HideCursor()
	EnableMouse()
	return func() {
		DisableMouse()
		ShowCursor()
		ExitAltScreen()
		run("stty", "-raw", "echo")
	}
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// Size returns the current terminal width/height in cells via `stty size`, which is portable across Linux and macOS without any ioctl/cgo code.
func Size() (width, height int, err error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 80, 24, err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 2 {
		return 80, 24, nil
	}
	h, _ := strconv.Atoi(fields[0])
	w, _ := strconv.Atoi(fields[1])
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	return w, h, nil
}

func EnterAltScreen() { os.Stdout.WriteString("\x1b[?1049h") }
func ExitAltScreen()  { os.Stdout.WriteString("\x1b[?1049l") }
func HideCursor()     { os.Stdout.WriteString("\x1b[?25l") }
func ShowCursor()     { os.Stdout.WriteString("\x1b[?25h") }
func EnableMouse()    { os.Stdout.WriteString("\x1b[?1002h\x1b[?1006h") }
func DisableMouse()   { os.Stdout.WriteString("\x1b[?1002l\x1b[?1006l") }
