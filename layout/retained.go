package layout

import "github.com/ZeroGCDev/zerotui/geometry"

// Retained caches a layout subtree's flattened placements until its geometry
// or explicit invalidation changes. It is intended for large dashboards where
// one hot widget changes while sibling layout branches remain stable.
//
// The cache is bounded by the subtree's placement count. ComputeInto reuses the
// backing slice and performs no allocation after the first computation.
type Retained struct {
	Child Node
	cache []Placement
	area  geometry.Rect
	dirty bool
}

func NewRetained(child Node) *Retained {
	return &Retained{Child: child, dirty: true}
}

func (r *Retained) Invalidate() {
	if r != nil {
		r.dirty = true
	}
}
func (r *Retained) Visible() bool {
	if r == nil || r.Child == nil {
		return false
	}
	if v, ok := r.Child.(VisibilityNode); ok {
		return v.Visible()
	}
	return true
}

func (r *Retained) Compute(area geometry.Rect) []Placement { return r.ComputeInto(area, nil) }

func (r *Retained) ComputeInto(area geometry.Rect, out []Placement) []Placement {
	if r == nil || r.Child == nil || !r.Visible() {
		return out
	}
	if r.dirty || r.area != area || len(r.cache) == 0 {
		r.cache = ComputeInto(r.Child, area, r.cache)
		r.area = area
		r.dirty = false
	}
	return append(out, r.cache...)
}

// Dirty reports whether this subtree must recompute its placement cache.
func (r *Retained) Dirty() bool { return r != nil && r.dirty }
