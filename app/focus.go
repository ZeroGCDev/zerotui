package app

import "github.com/ZeroGCDev/zerotui/widget"

type focusRing struct {
	items []widget.Focusable
	idx   int
}

func (f *focusRing) rebuild(items []widget.Focusable) {
	// Preserve the currently focused widget across relayout (e.g. resize) if it still exists in the new placement set.
	var cur widget.Focusable
	if len(f.items) > 0 && f.idx < len(f.items) {
		cur = f.items[f.idx]
	}
	f.items = items
	f.idx = 0
	for i, it := range items {
		if it == cur {
			f.idx = i
			break
		}
	}
	f.applyFocus()
}

func (f *focusRing) applyFocus() {
	for i, it := range f.items {
		it.Focus(i == f.idx)
	}
}

func (f *focusRing) next() {
	if len(f.items) == 0 {
		return
	}
	f.idx = (f.idx + 1) % len(f.items)
	f.applyFocus()
}

func (f *focusRing) prev() {
	if len(f.items) == 0 {
		return
	}
	f.idx = (f.idx - 1 + len(f.items)) % len(f.items)
	f.applyFocus()
}

func (f *focusRing) current() widget.Focusable {
	if len(f.items) == 0 || f.idx >= len(f.items) {
		return nil
	}
	return f.items[f.idx]
}
