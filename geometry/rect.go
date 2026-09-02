// Package geometry provides the plain-value Rect used for layout and hit testing. All operations are value-receiver and allocation-free.
package geometry

type Rect struct {
	X, Y, W, H int
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func (r Rect) Inset(n int) Rect {
	return r.InsetXY(n, n)
}

func (r Rect) InsetXY(x, y int) Rect {
	nr := Rect{X: r.X + x, Y: r.Y + y, W: r.W - 2*x, H: r.H - 2*y}
	if nr.W < 0 {
		nr.W = 0
	}
	if nr.H < 0 {
		nr.H = 0
	}
	return nr
}

// Row returns a 1-cell-tall rect at the given offset from the top of r.
func (r Rect) Row(offset int) Rect {
	return Rect{X: r.X, Y: r.Y + offset, W: r.W, H: 1}
}

// SplitH splits r into two rects side by side; left gets `w` columns.
func (r Rect) SplitH(w int) (left, right Rect) {
	if w > r.W {
		w = r.W
	}
	left = Rect{X: r.X, Y: r.Y, W: w, H: r.H}
	right = Rect{X: r.X + w, Y: r.Y, W: r.W - w, H: r.H}
	return
}

// SplitV splits r into two rects stacked vertically; top gets `h` rows.
func (r Rect) SplitV(h int) (top, bottom Rect) {
	if h > r.H {
		h = r.H
	}
	top = Rect{X: r.X, Y: r.Y, W: r.W, H: h}
	bottom = Rect{X: r.X, Y: r.Y + h, W: r.W, H: r.H - h}
	return
}
