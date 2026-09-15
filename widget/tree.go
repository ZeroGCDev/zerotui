package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// TreeNode is the presentation-neutral description of one tree row.
// ID is stable and is used for selection/expansion; Depth is supplied by TreeView.
type TreeNode struct {
	ID, Label   string
	Depth       int
	HasChildren bool
	Expanded    bool
	Disabled    bool
}

// TreeModel supplies hierarchical data to TreeView. Implementations may load
// children lazily. The widget never performs filesystem or application I/O.
type TreeModel interface {
	Roots() []TreeNode
	Children(parentID string) []TreeNode
	SetExpanded(id string, expanded bool) bool
}

// TreeView is a reusable keyboard/mouse tree presentation. It owns selection,
// scrolling and the visible-node projection; the application owns the model.
type TreeView struct {
	ThemeOverride *style.Theme
	FocusMixin
	Model        TreeModel
	Selected     int
	Scroll       int
	ManualScroll bool
	Background   *color.Color
	OnActivate   func(TreeNode)
	RowRenderer  func(buf *buffer.Buffer, area geometry.Rect, node TreeNode, selected, focused bool, theme *style.Theme)
	visible      []TreeNode
	dirty        bool
}

func NewTreeView(model TreeModel) *TreeView {
	return &TreeView{Model: model, dirty: true}
}

func (t *TreeView) OwnsBackground() bool     { return t.Background != nil }
func (t *TreeView) SetModel(model TreeModel) { t.Model = model; t.Invalidate() }
func (t *TreeView) Invalidate()              { t.dirty = true }
func (t *TreeView) SelectedNode() (TreeNode, bool) {
	v := t.ensureVisible()
	if t.Selected < 0 || t.Selected >= len(v) {
		return TreeNode{}, false
	}
	return v[t.Selected], true
}
func (t *TreeView) Visible(dst []TreeNode) []TreeNode {
	v := t.ensureVisible()
	dst = append(dst[:0], v...)
	return dst
}
func (t *TreeView) rebuildVisible() {
	t.visible = t.visible[:0]
	if t.Model == nil {
		t.dirty = false
		return
	}
	var walk func([]TreeNode)
	walk = func(xs []TreeNode) {
		for _, n := range xs {
			t.visible = append(t.visible, n)
			if n.HasChildren && n.Expanded {
				walk(t.Model.Children(n.ID))
			}
		}
	}
	walk(t.Model.Roots())
	t.dirty = false
}
func (t *TreeView) ensureVisible() []TreeNode {
	if t.dirty {
		t.rebuildVisible()
	}
	if t.Selected < 0 {
		t.Selected = 0
	}
	if t.Selected >= len(t.visible) {
		t.Selected = len(t.visible) - 1
	}
	if len(t.visible) == 0 {
		t.Selected = 0
	}
	return t.visible
}
func (t *TreeView) normalize(area geometry.Rect) {
	v := t.ensureVisible()
	h := area.H
	if h < 1 {
		return
	}
	if !t.ManualScroll {
		if t.Selected < t.Scroll {
			t.Scroll = t.Selected
		}
		if t.Selected >= t.Scroll+h {
			t.Scroll = t.Selected - h + 1
		}
	}
	maxScroll := len(v) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.Scroll < 0 {
		t.Scroll = 0
	}
	if t.Scroll > maxScroll {
		t.Scroll = maxScroll
	}
}

func defaultTreeRow(buf *buffer.Buffer, area geometry.Rect, n TreeNode, selected, focused bool, theme *style.Theme) {
	st := theme.Text
	if selected {
		if focused {
			st = theme.Selected
		} else {
			st = theme.Info
		}
	}
	prefix := "  "
	if n.HasChildren {
		if n.Expanded {
			prefix = "▾ "
		} else {
			prefix = "▸ "
		}
	}
	buf.FillRect(area.X, area.Y, area.W, 1, ' ', st)
	buf.SetString(area.X+1, area.Y, spaces(2*n.Depth)+prefix+n.Label, st)
}

func (t *TreeView) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if t.ThemeOverride != nil {
		theme = t.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	if t.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', style.Style{Bg: *t.Background})
	}
	t.normalize(area)
	v := t.visible
	render := t.RowRenderer
	if render == nil {
		render = defaultTreeRow
	}
	for row := 0; row < area.H; row++ {
		i := t.Scroll + row
		if i >= len(v) {
			break
		}
		render(buf, geometry.Rect{X: area.X, Y: area.Y + row, W: area.W, H: 1}, v[i], i == t.Selected, t.IsFocused(), theme)
	}
}

func (t *TreeView) HandleKey(k input.Key) bool {
	v := t.ensureVisible()
	if len(v) == 0 {
		return false
	}
	n := v[t.Selected]
	switch k.Type {
	case input.KeyUp:
		if t.Selected > 0 {
			t.Selected--
		}
		t.ManualScroll = false
		return true
	case input.KeyDown:
		if t.Selected < len(v)-1 {
			t.Selected++
		}
		t.ManualScroll = false
		return true
	case input.KeyEnter:
		if t.OnActivate != nil {
			t.OnActivate(n)
		}
		return true
	case input.KeyRight:
		if n.HasChildren && !n.Expanded {
			t.Model.SetExpanded(n.ID, true)
			t.Invalidate()
		}
		return true
	case input.KeyLeft:
		if n.HasChildren && n.Expanded {
			t.Model.SetExpanded(n.ID, false)
			t.Invalidate()
			return true
		}
	case input.KeyRune:
		if k.Rune == 'j' {
			if t.Selected < len(v)-1 {
				t.Selected++
			}
			t.ManualScroll = false
			return true
		}
		if k.Rune == 'k' {
			if t.Selected > 0 {
				t.Selected--
			}
			t.ManualScroll = false
			return true
		}
	}
	return false
}

func (t *TreeView) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if !area.Contains(ev.X, ev.Y) {
		return false
	}
	if ev.Action == input.MouseWheelUp || ev.Action == input.MouseWheelDown {
		t.ManualScroll = true
		if ev.Action == input.MouseWheelUp {
			t.Scroll -= 3
		} else {
			t.Scroll += 3
		}
		t.normalize(area)
		return true
	}
	if ev.Action != input.MousePress {
		return false
	}
	row := t.Scroll + ev.Y - area.Y
	v := t.ensureVisible()
	if row < 0 || row >= len(v) {
		return true
	}
	t.ManualScroll = false
	t.Selected = row
	n := v[row]
	indentEnd := area.X + 1 + 2*n.Depth + 2
	if n.HasChildren && ev.X <= indentEnd {
		t.Model.SetExpanded(n.ID, !n.Expanded)
		t.Invalidate()
		return true
	}
	if t.OnActivate != nil {
		t.OnActivate(n)
	}
	return true
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	s := make([]byte, n)
	for i := range s {
		s[i] = ' '
	}
	return string(s)
}
