// Package input turns a raw stdin byte stream into keyboard and mouse events, including SGR (1006) mouse click/drag/scroll decoding.
package input

type KeyType uint8

// KeyModifier describes modifier state carried by Kitty/progressive CSI-u
// keyboard reports. The zero value means no modifier.
type KeyModifier uint8

const (
	ModShift KeyModifier = 1 << iota
	ModAlt
	ModCtrl
	ModSuper
	ModHyper
	ModMeta
)

const (
	KeyRune KeyType = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyEsc
	KeyTab
	KeyShiftTab
	KeyBackspace
	KeyCtrlC
	KeyCtrlP
	KeyCtrlW
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeySpace
	KeyF2
	KeyF5
	KeyF6
)

type Key struct {
	Type KeyType
	Rune rune
	Mods KeyModifier
}

type MouseAction uint8

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseDrag
	MouseWheelUp
	MouseWheelDown
)

type MouseButton uint8

const (
	MouseLeft MouseButton = iota
	MouseMiddle
	MouseRight
	MouseNone
)

type MouseEvent struct {
	X, Y   int // 0-based terminal cell coordinates
	Button MouseButton
	Action MouseAction
	Mods   KeyModifier
}

// Paste is emitted for bracketed-paste input. The payload is intentionally
// kept as text: unlike individual key events, a paste is inherently a bulk
// operation and may allocate once for its contents.
// Event is exactly one of Key, Mouse, or Paste (Resize may accompany none).

type Event struct {
	IsMouse bool
	Key     Key
	Mouse   MouseEvent
	Paste   string
	Resize  bool
	Width   int
	Height  int
}
