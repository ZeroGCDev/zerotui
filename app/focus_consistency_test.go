package app

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
	"testing"
)

type focusProbe struct {
	widget.FocusMixin
	focusCalls int
}

func (p *focusProbe) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}
func (p *focusProbe) Focus(v bool)                                     { p.FocusMixin.Focus(v); p.focusCalls++ }
func (p *focusProbe) HandleKey(input.Key) bool                         { return false }
func (p *focusProbe) HandleMouse(input.MouseEvent, geometry.Rect) bool { return false }

func TestFocusRingRebuildBlursRemovedWidget(t *testing.T) {
	old := &focusProbe{}
	next := &focusProbe{}
	r := focusRing{}
	r.rebuild([]widget.Focusable{old})
	if !old.IsFocused() {
		t.Fatal("initial focus not applied")
	}
	r.rebuild([]widget.Focusable{next})
	if old.IsFocused() {
		t.Fatal("removed widget remained focused after rebuild")
	}
	if !next.IsFocused() {
		t.Fatal("new widget was not focused")
	}
}

func TestRelayoutDeduplicatesRepeatedFocusablePlacements(t *testing.T) {
	p := &focusProbe{}
	root := &layout.Stack{Children: []layout.Node{layout.Wrap(p), layout.Wrap(p)}}
	a := New(root, style.NordTheme())
	a.width, a.height = 20, 4
	a.relayout()
	if len(a.focus.items) != 1 {
		t.Fatalf("got %d focus stops, want 1", len(a.focus.items))
	}
	if !p.IsFocused() {
		t.Fatal("deduplicated widget is not focused")
	}
	if p.focusCalls != 1 {
		t.Fatalf("got %d focus calls, want 1", p.focusCalls)
	}
}
