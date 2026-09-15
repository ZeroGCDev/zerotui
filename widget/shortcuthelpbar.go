package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// Shortcut describes one key/label pair rendered by ShortcutHelpBar.
type Shortcut struct{ Key, Label string }

// ShortcutHelpBar renders a compact, allocation-free shortcut legend.
type ShortcutHelpBar struct {
	ThemeOverride *style.Theme
	Shortcuts     []Shortcut
	Background    *color.Color
	Separator     string
}

func NewShortcutHelpBar(shortcuts ...Shortcut) *ShortcutHelpBar {
	return &ShortcutHelpBar{Shortcuts: shortcuts, Separator: "  •  "}
}
func (h *ShortcutHelpBar) OwnsBackground() bool { return h.Background != nil }
func (h *ShortcutHelpBar) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if h.ThemeOverride != nil {
		theme = h.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	if h.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', bgOr(theme.Panel, h.Background))
	}
	x := area.X
	for i, s := range h.Shortcuts {
		if i > 0 {
			buf.SetString(x, area.Y, h.Separator, bgOr(theme.TextMuted, h.Background))
			x += len(h.Separator)
		}
		buf.SetString(x, area.Y, s.Key, bgOr(theme.Info, h.Background))
		x += len(s.Key)
		buf.SetString(x, area.Y, " "+s.Label, bgOr(theme.Text, h.Background))
		x += len(s.Label) + 1
		if x >= area.X+area.W {
			break
		}
	}
}
