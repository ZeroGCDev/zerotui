/*
// Package widget defines the drawing contract every zerotui component implements, plus a full set of ready-made trading-dashboard widgets: Label, Button, Toggle, Slider, Gauge, Sparkline, Table, List, Tabs, TextInput, Panel, PriceTicker and OrderBook.
*/
package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

/*
Widget is the minimum contract: draw yourself into `area` of buf.

Implementations must not allocate in Draw on the steady-state path - keep any scratch buffers as struct fields, not locals created per call.

Every stock widget in this package exposes an optional `Background *color.Color` field. Leave it nil to inherit whatever's already behind the widget (typically the enclosing Panel/Bordered's fill, or the app's Theme.Background).

Set it to override just that widget's backdrop, e.g. to highlight one row in a Table or tint a specific Toggle.
*/
type Widget interface {
	Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme)
}

// BackgroundOwner marks a widget that establishes an opaque background for
// the cells it occupies. The retained compositor uses this to reconstruct the
// correct backdrop before repainting a child damage region.
type BackgroundOwner interface {
	OwnsBackground() bool
}

// DirtyRegionProvider lets a high-frequency widget expose precise screen-space
// repaint bands without changing the Widget.Draw contract. The destination is
// caller-owned so implementations can remain allocation-free. Returning no
// regions means the caller may fall back to the widget's full placement.
type DirtyRegionProvider interface {
	DirtyRegions(area geometry.Rect, dst []geometry.Rect) []geometry.Rect
}

// Focusable widgets participate in Tab-navigation and receive routed keyboard/mouse input while focused. HandleKey/HandleMouse return true if the event was consumed.
type Focusable interface {
	Widget
	Focus(focused bool)
	IsFocused() bool
	HandleKey(k input.Key) bool
	HandleMouse(ev input.MouseEvent, area geometry.Rect) bool
}

// MouseHandler is implemented by widgets that need pointer interaction but
// should not participate in keyboard focus (resize handles are the primary
// example). App routes mouse events to these widgets as well.
type MouseHandler interface {
	Widget
	HandleMouse(ev input.MouseEvent, area geometry.Rect) bool
}

// FocusMixin gives concrete widgets IsFocused/Focus for free.
type FocusMixin struct{ focused bool }

func (f *FocusMixin) Focus(v bool)    { f.focused = v }
func (f *FocusMixin) IsFocused() bool { return f.focused }

// bgOr returns st with its background swapped to *bg if bg is non-nil, otherwise st unchanged. Every widget's optional Background override goes through this; it's a plain function (not a closure) so it stays allocation-free on the render path.
func bgOr(st style.Style, bg *color.Color) style.Style {
	if bg != nil {
		st = st.WithBg(*bg)
	}
	return st
}
