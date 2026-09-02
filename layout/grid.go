package layout

import "github.com/ZeroGCDev/zerotui/geometry"

/*
Grid arranges children in Rows x Cols equal-sized cells, row-major. Fewer children than Rows*Cols simply leaves trailing cells empty; more children than cells are ignored (size your grid to your widget count).
*/
type Grid struct {
	Rows, Cols int
	Children   []Node
	Gap        int
}

func NewGrid(rows, cols int, children ...Node) *Grid {
	return &Grid{Rows: rows, Cols: cols, Children: children, Gap: 1}
}

func (g *Grid) Compute(area geometry.Rect) []Placement { return g.ComputeInto(area, nil) }

func (g *Grid) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if g.Rows <= 0 || g.Cols <= 0 {
		return out
	}
	cellW := (area.W - g.Gap*(g.Cols-1)) / g.Cols
	cellH := (area.H - g.Gap*(g.Rows-1)) / g.Rows
	if cellW < 0 {
		cellW = 0
	}
	if cellH < 0 {
		cellH = 0
	}
	for i, child := range g.Children {
		if i >= g.Rows*g.Cols {
			break
		}
		r, c := i/g.Cols, i%g.Cols
		cellArea := geometry.Rect{X: area.X + c*(cellW+g.Gap), Y: area.Y + r*(cellH+g.Gap), W: cellW, H: cellH}
		if rnode, ok := child.(ReusableNode); ok {
			out = rnode.ComputeInto(cellArea, out)
		} else {
			out = append(out, child.Compute(cellArea)...)
		}
	}
	return out
}
