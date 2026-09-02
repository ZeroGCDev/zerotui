package layout

import "github.com/ZeroGCDev/zerotui/geometry"

// Centered places a child in the middle of its parent. Width/Height are
// fractions of the available area when >0; MaxW/MaxH cap the result in cells.
// It is useful for palettes, dialogs, inspectors and floating dashboards.
type CenteredBox struct {
	Child      Node
	Width      float64
	Height     float64
	MaxW, MaxH int
	MinW, MinH int
}

func Center(child Node, width, height float64) *CenteredBox {
	return &CenteredBox{Child: child, Width: width, Height: height}
}

func (c *CenteredBox) Compute(area geometry.Rect) []Placement { return c.ComputeInto(area, nil) }

func (c *CenteredBox) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if c.Child == nil {
		return out
	}
	w := area.W
	h := area.H
	if c.Width > 0 && c.Width < 1 {
		w = int(float64(area.W) * c.Width)
	}
	if c.Height > 0 && c.Height < 1 {
		h = int(float64(area.H) * c.Height)
	}
	if c.MaxW > 0 && w > c.MaxW {
		w = c.MaxW
	}
	if c.MaxH > 0 && h > c.MaxH {
		h = c.MaxH
	}
	if w < c.MinW {
		w = c.MinW
	}
	if h < c.MinH {
		h = c.MinH
	}
	if w > area.W {
		w = area.W
	}
	if h > area.H {
		h = area.H
	}
	childArea := geometry.Rect{
		X: area.X + (area.W-w)/2,
		Y: area.Y + (area.H-h)/2,
		W: w,
		H: h,
	}
	if r, ok := c.Child.(ReusableNode); ok {
		return r.ComputeInto(childArea, out)
	}
	return append(out, c.Child.Compute(childArea)...)
}
