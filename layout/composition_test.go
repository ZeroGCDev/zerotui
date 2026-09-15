package layout

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

type testWidget struct{}

func (*testWidget) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func TestNestedSplitProducesMultipleResizeHandles(t *testing.T) {
	a := NewSplit(Horizontal, Wrap(&testWidget{}), Wrap(&testWidget{}), 0.5)
	b := NewSplit(Vertical, a, Wrap(&testWidget{}), 0.5)
	placements := b.Compute(geometry.Rect{W: 120, H: 40})
	count := 0
	for _, p := range placements {
		if _, ok := p.Widget.(*widget.ResizeHandle); ok {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("resize handles=%d want 2", count)
	}
}

func TestFlexPreferredHeightIsExplicit(t *testing.T) {
	positions := &preferredTestNode{height: 8}
	orders := &preferredTestNode{height: 4}
	spacer := &preferredTestNode{}

	// A node merely implementing PreferredHeight must not silently change
	// existing Flex1 semantics. All three children remain equal-weighted.
	legacy := NewFlex(Vertical, Flex1(positions), Flex1(orders), Flex1(spacer))
	pl := legacy.Compute(geometry.Rect{X: 0, Y: 0, W: 100, H: 30})
	if len(pl) != 3 || pl[0].Area.H != 9 || pl[1].Area.H != 9 || pl[2].Area.H != 9 {
		t.Fatalf("legacy Flex sizing changed: %+v", pl)
	}

	// FitHeight opts the first two children into intrinsic sizing. The spacer
	// receives the remaining height.
	root := NewFlex(Vertical,
		Flex1(FitHeight(positions)),
		Flex1(FitHeight(orders)),
		Flex1(spacer),
	)
	placements := root.Compute(geometry.Rect{X: 0, Y: 0, W: 100, H: 30})
	if len(placements) != 3 {
		t.Fatalf("placements=%d, want 3", len(placements))
	}
	if placements[0].Area.H != 8 || placements[1].Area.H != 4 || placements[2].Area.H != 16 {
		t.Fatalf("unexpected fitted heights: %d, %d, %d", placements[0].Area.H, placements[1].Area.H, placements[2].Area.H)
	}
}

type preferredTestNode struct{ height int }

func (n *preferredTestNode) Compute(area geometry.Rect) []Placement { return []Placement{{Area: area}} }
func (n *preferredTestNode) PreferredHeight() int                   { return n.height }

func TestLeafForwardsPreferredHeight(t *testing.T) {
	w := &preferredWidget{height: 7}
	leaf := Wrap(w)
	if got := leaf.PreferredHeight(); got != 7 {
		t.Fatalf("Leaf.PreferredHeight() = %d, want 7", got)
	}
	bordered := Bordered("X", leaf, nil)
	if got := bordered.(PreferredHeight).PreferredHeight(); got != 9 {
		t.Fatalf("Bordered preferred height = %d, want 9", got)
	}
}

type preferredWidget struct{ height int }

func (*preferredWidget) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}
func (w *preferredWidget) PreferredHeight() int                           { return w.height }

func TestFitHeightClampsWhenTerminalIsTooSmall(t *testing.T) {
	first := &preferredTestNode{height: 8}
	second := &preferredTestNode{height: 8}
	root := NewFlex(Vertical, Flex1(FitHeight(first)), Flex1(FitHeight(second)))
	pl := root.Compute(geometry.Rect{W: 20, H: 5})
	if len(pl) != 2 {
		t.Fatalf("placements=%d, want 2 fitted children", len(pl))
	}
	if pl[0].Area.H != 2 || pl[1].Area.H != 2 {
		t.Fatalf("unexpected proportional fitted heights: %d, %d", pl[0].Area.H, pl[1].Area.H)
	}
	if pl[1].Area.Y != 3 {
		t.Fatalf("second fitted child Y=%d, want 3", pl[1].Area.Y)
	}
}

func TestFitHeightWorksThroughBorderedAndLeaf(t *testing.T) {
	w := &preferredWidget{height: 6}
	root := NewFlex(Vertical,
		Flex1(FitHeight(Bordered("PANEL", Wrap(w), nil))),
		Flex1(&preferredTestNode{}),
	)
	pl := root.Compute(geometry.Rect{W: 80, H: 20})
	if len(pl) != 3 {
		t.Fatalf("placements=%d, want 3 (border + child + spacer)", len(pl))
	}
	if pl[0].Area.H != 8 || pl[1].Area.H != 6 {
		t.Fatalf("unexpected bordered fitted areas: %d, %d", pl[0].Area.H, pl[1].Area.H)
	}
	if pl[2].Area.H != 11 {
		t.Fatalf("spacer height=%d, want 11", pl[2].Area.H)
	}
}

func TestFitHeightMixedChildrenKeepIntrinsicAndRemainingFlexSpace(t *testing.T) {
	first := &preferredTestNode{height: 6}
	second := &preferredTestNode{height: 4}
	spacer := &preferredTestNode{}
	root := NewFlex(Vertical,
		Flex1(FitHeight(first)),
		Flex1(FitHeight(second)),
		Flex1(spacer),
	)
	pl := root.Compute(geometry.Rect{W: 40, H: 20})
	if len(pl) != 3 {
		t.Fatalf("placements=%d, want 3", len(pl))
	}
	if pl[0].Area.H != 6 || pl[1].Area.H != 4 || pl[2].Area.H != 8 {
		t.Fatalf("unexpected heights: %d, %d, %d", pl[0].Area.H, pl[1].Area.H, pl[2].Area.H)
	}
}

func TestFitHeightTinyTerminalDoesNotStarveLaterChildren(t *testing.T) {
	a := &preferredTestNode{height: 100}
	b := &preferredTestNode{height: 100}
	root := NewFlex(Vertical, Flex1(FitHeight(a)), Flex1(FitHeight(b)))
	pl := root.Compute(geometry.Rect{W: 20, H: 3})
	if len(pl) != 2 {
		t.Fatalf("placements=%d, want 2", len(pl))
	}
	if pl[0].Area.H != 1 || pl[1].Area.H != 1 {
		t.Fatalf("unexpected tiny-terminal heights: %d, %d", pl[0].Area.H, pl[1].Area.H)
	}
}

func TestFlexNilItemsDoNotConsumeSpaceOrGaps(t *testing.T) {
	a := &preferredTestNode{}
	b := &preferredTestNode{}
	root := NewFlex(Vertical, Flex1(a), Flex1(nil), Flex1(b))
	pl := root.Compute(geometry.Rect{W: 20, H: 10})
	if len(pl) != 2 {
		t.Fatalf("placements=%d, want 2", len(pl))
	}
	if pl[0].Area.H != 4 || pl[1].Area.H != 4 || pl[1].Area.Y != 5 {
		t.Fatalf("unexpected areas: %+v", pl)
	}
}

func TestLayoutPreferredHeightPropagatesThroughWrappers(t *testing.T) {
	child := &preferredWidget{height: 5}
	n := Padding(FixedSize(SizeBounds(Wrap(child), 1, 20, 3, 8), 10, 0), 0, 1, 0, 2)
	if got := n.(PreferredHeight).PreferredHeight(); got != 8 {
		t.Fatalf("wrapped preferred height=%d, want 8", got)
	}
	panel := ClosableRounded("P", n, nil, nil)
	if got := panel.PreferredHeight(); got != 10 {
		t.Fatalf("closable preferred height=%d, want 10", got)
	}
}

func TestCenterSupportsAbsoluteAndFractionalSizes(t *testing.T) {
	child := Wrap(&testWidget{})
	abs := Center(child, 40, 5).Compute(geometry.Rect{X: 0, Y: 0, W: 100, H: 20})
	if len(abs) != 1 || abs[0].Area != (geometry.Rect{X: 30, Y: 7, W: 40, H: 5}) {
		t.Fatalf("absolute center area=%+v", abs)
	}
	frac := Center(child, .5, .5).Compute(geometry.Rect{X: 0, Y: 0, W: 100, H: 20})
	if len(frac) != 1 || frac[0].Area != (geometry.Rect{X: 25, Y: 5, W: 50, H: 10}) {
		t.Fatalf("fractional center area=%+v", frac)
	}
}

func TestSplitClampsOversizedGap(t *testing.T) {
	s := NewSplit(Horizontal, Wrap(&testWidget{}), Wrap(&testWidget{}), .5)
	s.Gap = 100
	pl := s.Compute(geometry.Rect{W: 10, H: 4})
	if len(pl) != 3 {
		t.Fatalf("placements=%d, want 3", len(pl))
	}
	for _, p := range pl {
		if p.Area.X < 0 || p.Area.X+p.Area.W > 10 {
			t.Fatalf("placement escaped split: %+v", p.Area)
		}
	}
}
