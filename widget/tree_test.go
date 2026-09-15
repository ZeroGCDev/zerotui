package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

type testTreeModel struct{ expanded map[string]bool }

func (m *testTreeModel) Roots() []TreeNode {
	return []TreeNode{{ID: "root", Label: "root", HasChildren: true, Expanded: m.expanded["root"]}}
}
func (m *testTreeModel) Children(id string) []TreeNode {
	if id != "root" {
		return nil
	}
	return []TreeNode{{ID: "a", Label: "a", Depth: 1}, {ID: "b", Label: "b", Depth: 1}}
}
func (m *testTreeModel) SetExpanded(id string, expanded bool) bool {
	if m.expanded[id] == expanded {
		return false
	}
	m.expanded[id] = expanded
	return true
}
func TestTreeViewUsesModelForLazyProjection(t *testing.T) {
	m := &testTreeModel{expanded: map[string]bool{"root": true}}
	tr := NewTreeView(m)
	if got := len(tr.Visible(nil)); got != 3 {
		t.Fatalf("visible=%d want 3", got)
	}
	m.expanded["root"] = false
	tr.Invalidate()
	if got := len(tr.Visible(nil)); got != 1 {
		t.Fatalf("collapsed visible=%d want 1", got)
	}
}
func TestTreeViewSteadyStateDrawUsesCachedProjection(t *testing.T) {
	m := &testTreeModel{expanded: map[string]bool{"root": true}}
	tr := NewTreeView(m)
	b := buffer.New(40, 4)
	tr.Draw(b, geometry.Rect{W: 40, H: 4}, style.NordTheme())
	if got := len(tr.visible); got != 3 {
		t.Fatalf("projection=%d", got)
	}
}
