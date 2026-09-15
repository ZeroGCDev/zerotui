package app

import (
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
)

// spatialIndex is a fixed-grid broad phase over the flattened placement tree.
// It is rebuilt only when layout changes. Render-time queries reuse all storage.
type spatialIndex struct {
	tileW, tileH int
	cols, rows   int
	head         []int
	next         []int
	placement    []int
	seen         []uint32
	generation   uint32
	candidates   []int
}

const noSpatialEntry = -1

func (s *spatialIndex) rebuild(width, height int, placements []layout.Placement) {
	const tw, th = 8, 4
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	cols := (width + tw - 1) / tw
	rows := (height + th - 1) / th
	tileCount := cols * rows
	if cap(s.head) < tileCount {
		s.head = make([]int, tileCount)
	} else {
		s.head = s.head[:tileCount]
	}
	for i := range s.head {
		s.head[i] = noSpatialEntry
	}

	// Count placement/tile intersections first so the entry arrays are allocated
	// exactly once, during reflow only.
	entries := 0
	for _, p := range placements {
		entries += tileCoverageCount(p.Area, tw, th, cols, rows)
	}
	if cap(s.next) < entries {
		s.next = make([]int, entries)
	} else {
		s.next = s.next[:entries]
	}
	if cap(s.placement) < entries {
		s.placement = make([]int, entries)
	} else {
		s.placement = s.placement[:entries]
	}
	if cap(s.seen) < len(placements) {
		s.seen = make([]uint32, len(placements))
	} else {
		s.seen = s.seen[:len(placements)]
		for i := range s.seen {
			s.seen[i] = 0
		}
	}
	if cap(s.candidates) < len(placements) {
		s.candidates = make([]int, len(placements))
	} else {
		s.candidates = s.candidates[:len(placements)]
	}

	entry := 0
	for pi, p := range placements {
		if p.Area.W <= 0 || p.Area.H <= 0 {
			continue
		}
		minTX := clampInt(p.Area.X/tw, 0, cols-1)
		minTY := clampInt(p.Area.Y/th, 0, rows-1)
		maxTX := clampInt((p.Area.X+p.Area.W-1)/tw, 0, cols-1)
		maxTY := clampInt((p.Area.Y+p.Area.H-1)/th, 0, rows-1)
		for ty := minTY; ty <= maxTY; ty++ {
			for tx := minTX; tx <= maxTX; tx++ {
				tile := ty*cols + tx
				s.placement[entry] = pi
				s.next[entry] = s.head[tile]
				s.head[tile] = entry
				entry++
			}
		}
	}
	s.tileW, s.tileH, s.cols, s.rows = tw, th, cols, rows
	s.generation = 1
}

func tileCoverageCount(r geometry.Rect, tw, th, cols, rows int) int {
	if r.W <= 0 || r.H <= 0 {
		return 0
	}
	minTX := clampInt(r.X/tw, 0, cols-1)
	minTY := clampInt(r.Y/th, 0, rows-1)
	maxTX := clampInt((r.X+r.W-1)/tw, 0, cols-1)
	maxTY := clampInt((r.Y+r.H-1)/th, 0, rows-1)
	if maxTX < minTX || maxTY < minTY {
		return 0
	}
	return (maxTX - minTX + 1) * (maxTY - minTY + 1)
}

func (s *spatialIndex) candidateIndices(r geometry.Rect, placements []layout.Placement) []int {
	out := s.candidates[:0]
	if s.cols == 0 || len(placements) == 0 || r.W <= 0 || r.H <= 0 {
		return out
	}
	minTX := clampInt(r.X/s.tileW, 0, s.cols-1)
	minTY := clampInt(r.Y/s.tileH, 0, s.rows-1)
	maxTX := clampInt((r.X+r.W-1)/s.tileW, 0, s.cols-1)
	maxTY := clampInt((r.Y+r.H-1)/s.tileH, 0, s.rows-1)

	s.generation++
	if s.generation == 0 {
		for i := range s.seen {
			s.seen[i] = 0
		}
		s.generation = 1
	}
	for ty := minTY; ty <= maxTY; ty++ {
		for tx := minTX; tx <= maxTX; tx++ {
			for e := s.head[ty*s.cols+tx]; e != noSpatialEntry; e = s.next[e] {
				pi := s.placement[e]
				if pi < 0 || pi >= len(placements) || s.seen[pi] == s.generation {
					continue
				}
				s.seen[pi] = s.generation
				if rectsTouch(placements[pi].Area, r) {
					out = append(out, pi)
				}
			}
		}
	}

	// The spatial grid is a broad-phase index, not a z-order list. Because
	// entries are linked into each tile at the head, the query order is
	// intentionally arbitrary. Painting in that order is incorrect for
	// overlapping retained placements: a border/background can be painted
	// after its child and make the child appear to disappear after a partial
	// repaint. Restore the original placement order without allocating.
	for i := 1; i < len(out); i++ {
		v := out[i]
		j := i - 1
		for j >= 0 && out[j] > v {
			out[j+1] = out[j]
			j--
		}
		out[j+1] = v
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
