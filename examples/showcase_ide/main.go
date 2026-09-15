package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZeroGCDev/zerotui/app"
	"github.com/ZeroGCDev/zerotui/editor"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
)

// showcase_ide is the reference editor example.
//
// The terminal is closed initially and is created lazily when opened.
// Ctrl+J / Ctrl+` toggles the integrated terminal.
// Ctrl+R runs the active file.
// Ctrl+c to Close
func main() {
	root := "."

	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	root, _ = filepath.Abs(root)

	ed := editor.New(root)

	zedTheme := style.ZedEditorTheme()
	ed.SetTheme(zedTheme)

	a := app.New(layout.Wrap(ed), &zedTheme.Theme)

	// Keep the IDE's activity rail and settings chrome on the same palette as
	// the editor. Theme selection is applied live without restarting the app.
	ed.OnThemeChange = func(t *style.EditorTheme) {
		if t != nil {
			*a.Theme = t.Theme
		}
		a.Invalidate()
	}
	ed.OnInvalidate = a.Invalidate

	// Disable the application's default quit keys so Ctrl+Q can be
	// handled explicitly below.
	a.QuitKeys = nil

	a.OnKey = func(k input.Key) bool {
		// Ctrl+Q
		if k.Type == input.KeyRune &&
			k.Mods&input.ModCtrl != 0 &&
			k.Rune == 'q' {
			return false
		}

		// Ctrl+R — run the active file.
		if k.Type == input.KeyRune &&
			k.Mods&input.ModCtrl != 0 &&
			k.Rune == 'r' {
			return ed.RunActiveFile()
		}

		return false
	}

	if err := a.Run(); err != nil {
		fmt.Println(err)
	}
}
