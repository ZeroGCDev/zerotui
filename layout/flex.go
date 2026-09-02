package layout

import "github.com/ZeroGCDev/zerotui/geometry"

type Direction uint8

const (
	Horizontal Direction = iota
	Vertical
)

// Item is one child of a Flex: either a fixed cell size or, if Size==0, a share of the remaining space proportional to Weight (default weight 1).
type Item struct {
	Node   Node
	Size   int // fixed size in cells; 0 = flexible
	Weight int // used when Size==0; 0 treated as 1
}

func Fix(n Node, size int) Item     { return Item{Node: n, Size: size} }
func Flex1(n Node) Item             { return Item{Node: n, Weight: 1} }
func FlexN(n Node, weight int) Item { return Item{Node: n, Weight: weight} }

// FlexBox arranges Items in a row or column, giving fixed items their exact size and dividing whatever remains among flexible items by weight. A 1-cell gap is left between items when Gap is left at its default (0 disables the gap explicitly via NoGap).
type FlexBox struct {
	Dir   Direction
	Items []Item
	Gap   int
}

func NewFlex(dir Direction, items ...Item) *FlexBox {
	return &FlexBox{Dir: dir, Items: items, Gap: 1}
}

func (f *FlexBox) Visible() bool {
	for _, it := range f.Items {
		if it.Node != nil && isVisible(it.Node) {
			return true
		}
	}
	return false
}

func (f *FlexBox) Compute(area geometry.Rect) []Placement {
	return f.ComputeInto(area, nil)
}

func isVisible(n Node) bool {
	if v, ok := n.(VisibilityNode); ok {
		return v.Visible()
	}
	return true
}

func (f *FlexBox) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	// Count only visible direct children. Closed panels therefore give their
	// cells back to siblings without rebuilding the layout tree.
	visible := 0
	for _, it := range f.Items {
		if it.Node == nil || isVisible(it.Node) {
			visible++
		}
	}
	if visible == 0 {
		return out
	}

	main := area.W
	if f.Dir == Vertical {
		main = area.H
	}
	gapTotal := f.Gap * (visible - 1)
	fixedTotal, weightTotal := 0, 0
	for _, it := range f.Items {
		if it.Node != nil && !isVisible(it.Node) {
			continue
		}
		if it.Size > 0 {
			fixedTotal += it.Size
		} else {
			w := it.Weight
			if w <= 0 {
				w = 1
			}
			weightTotal += w
		}
	}
	remaining := main - fixedTotal - gapTotal
	if remaining < 0 {
		remaining = 0
	}
	if weightTotal == 0 {
		weightTotal = 1
	}

	pos := 0
	emitted := 0
	for _, it := range f.Items {
		if it.Node == nil || !isVisible(it.Node) {
			continue
		}
		size := it.Size
		if size == 0 {
			w := it.Weight
			if w <= 0 {
				w = 1
			}
			size = remaining * w / weightTotal
		}
		var childArea geometry.Rect
		if f.Dir == Horizontal {
			childArea = geometry.Rect{X: area.X + pos, Y: area.Y, W: size, H: area.H}
		} else {
			childArea = geometry.Rect{X: area.X, Y: area.Y + pos, W: area.W, H: size}
		}
		if r, ok := it.Node.(ReusableNode); ok {
			out = r.ComputeInto(childArea, out)
		} else {
			out = append(out, it.Node.Compute(childArea)...)
		}
		emitted++
		pos += size
		if emitted < visible {
			pos += f.Gap
		}
	}
	return out
}
