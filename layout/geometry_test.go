package layout

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

type geometryWidget struct{}

func (*geometryWidget) Draw(*buffer.Buffer, geometry.Rect, *style.Theme) {}

func assertPlacementsInside(t *testing.T, area geometry.Rect, placements []Placement) {
	t.Helper()
	for i, p := range placements {
		r := p.Area
		if r.W < 0 || r.H < 0 {
			t.Fatalf("placement %d has negative size: %+v", i, r)
		}
		if r.W == 0 || r.H == 0 {
			continue
		}
		if r.X < area.X || r.Y < area.Y || r.X+r.W > area.X+area.W || r.Y+r.H > area.Y+area.H {
			t.Fatalf("placement %d escaped parent: child=%+v parent=%+v", i, r, area)
		}
	}
}

func TestLayoutsKeepPositivePlacementsInsideParent(t *testing.T) {
	leaf := Wrap(&geometryWidget{})
	cases := []Node{
		NewFlex(Vertical, Fix(leaf, 100), Flex1(leaf)),
		NewFlex(Horizontal, Fix(leaf, 100), Flex1(leaf)),
		NewGrid(3, 3, leaf, leaf, leaf, leaf, leaf),
		NewSplit(Horizontal, leaf, leaf, .7),
		NewSplit(Vertical, leaf, leaf, .7),
		Center(leaf, 40, 5),
		FixedSize(leaf, 40, 5),
		SizeBounds(leaf, 10, 20, 3, 8),
		Padding(leaf, 2, 1, 2, 1),
		NewStack(leaf, leaf),
		NewOverlay(func() bool { return true }, leaf),
		Responsive(50, FixedSize(leaf, 10, 4), FixedSize(leaf, 30, 8)),
		ClosableRounded("P", leaf, nil, nil),
		NewRetained(leaf),
	}
	area := geometry.Rect{X: 3, Y: 2, W: 60, H: 20}
	for i, n := range cases {
		assertPlacementsInside(t, area, n.Compute(area))
		_ = i
	}
}

func TestFlexOversizedFixedItemsAreClamped(t *testing.T) {
	a, b := Wrap(&geometryWidget{}), Wrap(&geometryWidget{})
	area := geometry.Rect{W: 10, H: 4}
	pl := NewFlex(Vertical, Fix(a, 20), Fix(b, 20)).Compute(area)
	if len(pl) != 2 || pl[0].Area.H+pl[1].Area.H != 3 {
		t.Fatalf("oversized fixed items did not partition available height: %+v", pl)
	}
	assertPlacementsInside(t, area, pl)
}

func TestBorderedAndCenterRespectTinyParent(t *testing.T) {
	leaf := Wrap(&geometryWidget{})
	areas := []geometry.Rect{
		{X: 5, Y: 3, W: 1, H: 1},
		{X: 5, Y: 3, W: 2, H: 2},
		{X: 5, Y: 3, W: 3, H: 2},
	}
	for _, area := range areas {
		for _, n := range []Node{
			Bordered("X", leaf, nil),
			Center(leaf, 40, 5),
			Padding(leaf, 4, 4, 4, 4),
		} {
			assertPlacementsInside(t, area, n.Compute(area))
		}
	}
}
