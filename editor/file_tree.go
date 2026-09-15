package editor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
	"github.com/ZeroGCDev/zerotui/widget"
)

// FileTree is the IDE filesystem adapter around widget.TreeView. The widget
// owns selection, scrolling and row projection; this type owns filesystem
// loading and file operations.
type FileTree struct {
	*widget.TreeView

	// Root is the directory currently being browsed.
	//
	// Unlike the original implementation, Root is not a permanent workspace
	// boundary. The user can navigate to its parent and continue browsing
	// anywhere on the filesystem.
	Root string

	nodes         []*node
	loaded        bool
	modal         fileTreeModal
	modalValue    []rune
	modalError    string
	confirmDelete bool

	OnOpen   func(path string)
	OnRename func(oldPath, newPath string)
	OnDelete func(path string)
}

type fileTreeModal uint8

const (
	modalNone fileTreeModal = iota
	modalNewFile
	modalNewFolder
	modalRename
)

type node struct {
	name     string
	path     string
	dir      bool
	expanded bool
	loaded   bool
	children []*node
	depth    int

	// parentEntry is the virtual ".." entry displayed at the top of a
	// browsable directory. It is not a real filesystem node.
	parentEntry bool
}

func NewFileTree(root string) *FileTree {
	if root == "" {
		root = "."
	}

	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}

	t := &FileTree{
		Root: root,
	}

	t.TreeView = widget.NewTreeView(t)
	t.TreeView.RowRenderer = t.drawTreeRow
	t.TreeView.OnActivate = func(n widget.TreeNode) {
		t.activateID(n.ID)
	}

	t.reload()

	return t
}

func (t *FileTree) OwnsBackground() bool {
	return true
}

// Roots/Children/SetExpanded implement widget.TreeModel. They contain all
// filesystem-specific behavior while TreeView remains reusable and I/O-free.
func (t *FileTree) Roots() []widget.TreeNode {
	if len(t.nodes) == 0 {
		return nil
	}

	return []widget.TreeNode{
		t.treeNode(t.nodes[0]),
	}
}

func (t *FileTree) Children(id string) []widget.TreeNode {
	n := t.findNode(id)

	if n == nil || !n.dir {
		return nil
	}

	t.load(n)

	out := make([]widget.TreeNode, 0, len(n.children))

	for _, c := range n.children {
		out = append(out, t.treeNode(c))
	}

	return out
}

func (t *FileTree) SetExpanded(id string, expanded bool) bool {
	n := t.findNode(id)

	if n == nil ||
		!n.dir ||
		n.parentEntry {
		return false
	}

	if expanded {
		t.load(n)
	}

	if n.expanded == expanded {
		return false
	}

	n.expanded = expanded
	t.TreeView.Invalidate()

	return true
}

func (t *FileTree) treeNode(n *node) widget.TreeNode {
	return widget.TreeNode{
		ID:          n.path,
		Label:       n.name,
		Depth:       n.depth,
		HasChildren: n.dir && !n.parentEntry,
		Expanded:    n.expanded,
	}
}

func (t *FileTree) findNode(path string) *node {
	var walk func(*node) *node

	walk = func(n *node) *node {
		if n == nil {
			return nil
		}

		if samePath(n.path, path) {
			return n
		}

		for _, c := range n.children {
			if x := walk(c); x != nil {
				return x
			}
		}

		return nil
	}

	if len(t.nodes) > 0 {
		return walk(t.nodes[0])
	}

	return nil
}

func (t *FileTree) selectedNode() *node {
	n, ok := t.TreeView.SelectedNode()

	if !ok {
		return nil
	}

	return t.findNode(n.ID)
}

// reload rebuilds the currently browsed directory.
//
// Root is intentionally the current browsing location rather than a fixed
// workspace boundary. The virtual ".." entry is added by load() when a parent
// directory exists.
func (t *FileTree) reload() {
	selectedPath := ""

	if n := t.selectedNode(); n != nil {
		selectedPath = n.path
	}

	t.nodes = t.nodes[:0]

	root := filepath.Clean(t.Root)

	info, err := os.Stat(root)

	if err != nil || !info.IsDir() {
		t.loaded = true
		t.TreeView.Invalidate()
		return
	}

	r := &node{
		name:     info.Name(),
		path:     root,
		dir:      true,
		expanded: true,
		depth:    0,
	}

	// For filesystem roots such as "/" the base name can be empty.
	if r.name == "" {
		r.name = string(filepath.Separator)
	}

	t.nodes = append(t.nodes, r)

	t.load(r)

	r.expanded = true

	t.loaded = true
	t.TreeView.Invalidate()

	if selectedPath != "" {
		v := t.TreeView.Visible(nil)

		for i, x := range v {
			if samePath(x.ID, selectedPath) {
				t.TreeView.Selected = i
				break
			}
		}
	}

	t.TreeView.ManualScroll = false
}

func (t *FileTree) Refresh() {
	t.reload()
}

func (t *FileTree) NewFile(name string) bool {
	return t.createEntry(name, false)
}

func (t *FileTree) NewFolder(name string) bool {
	return t.createEntry(name, true)
}

func (t *FileTree) createEntry(name string, dir bool) bool {
	name = strings.TrimSpace(name)

	if name == "" ||
		name == "." ||
		name == ".." ||
		strings.ContainsAny(name, `/\`) {
		return false
	}

	n := t.selectedNode()

	if n == nil || n.parentEntry {
		return false
	}

	parent := n.path

	if !n.dir {
		parent = filepath.Dir(parent)
	}

	path := filepath.Join(parent, name)

	var err error

	if dir {
		err = os.Mkdir(path, 0755)
	} else {
		err = os.WriteFile(path, nil, 0644)
	}

	if err != nil {
		return false
	}

	t.Refresh()

	if !dir && t.OnOpen != nil {
		t.OnOpen(path)
	}

	return true
}

func (t *FileTree) RenameSelected(name string) bool {
	n := t.selectedNode()

	name = strings.TrimSpace(name)

	if n == nil ||
		n.parentEntry ||
		name == "" ||
		name == "." ||
		name == ".." ||
		strings.ContainsAny(name, `/\`) {
		return false
	}

	path := filepath.Join(filepath.Dir(n.path), name)

	if err := os.Rename(n.path, path); err != nil {
		return false
	}

	oldPath := n.path

	t.Refresh()

	if t.OnRename != nil {
		t.OnRename(oldPath, path)
	}

	if !n.dir && t.OnOpen != nil {
		t.OnOpen(path)
	}

	return true
}

func (t *FileTree) DeleteSelected() bool {
	n := t.selectedNode()

	if n == nil || n.parentEntry {
		return false
	}

	deleted := n.path

	if err := os.RemoveAll(deleted); err != nil {
		return false
	}

	t.Refresh()

	if t.OnDelete != nil {
		t.OnDelete(deleted)
	}

	return true
}

func (t *FileTree) load(n *node) {
	if !n.dir ||
		n.parentEntry ||
		n.loaded {
		return
	}

	entries, err := os.ReadDir(n.path)

	if err != nil {
		n.loaded = true
		return
	}

	n.children = n.children[:0]

	// Add the virtual parent entry first when a parent directory exists.
	parent := filepath.Dir(n.path)

	if !samePath(parent, n.path) {
		n.children = append(n.children, &node{
			name:        "..",
			path:        parent,
			dir:         true,
			depth:       n.depth + 1,
			parentEntry: true,
		})
	}

	for _, e := range entries {
		name := e.Name()

		if strings.HasPrefix(name, ".git") ||
			name == "node_modules" ||
			name == "vendor" {
			continue
		}

		if e.IsDir() {
			n.children = append(n.children, &node{
				name:  name,
				path:  filepath.Join(n.path, name),
				dir:   true,
				depth: n.depth + 1,
			})
		} else if isTextFile(name) {
			n.children = append(n.children, &node{
				name:  name,
				path:  filepath.Join(n.path, name),
				depth: n.depth + 1,
			})
		}
	}

	// Keep ".." at the top, then directories, then files.
	sort.SliceStable(n.children, func(i, j int) bool {
		a := n.children[i]
		b := n.children[j]

		if a.parentEntry != b.parentEntry {
			return a.parentEntry
		}

		if a.dir != b.dir {
			return a.dir
		}

		return strings.ToLower(a.name) <
			strings.ToLower(b.name)
	})

	n.loaded = true
}

// visible is retained as a package-level compatibility/testing helper.
func (t *FileTree) visible(dst []*node) []*node {
	v := t.TreeView.Visible(nil)

	dst = dst[:0]

	for _, x := range v {
		if n := t.findNode(x.ID); n != nil {
			dst = append(dst, n)
		}
	}

	return dst
}

func (t *FileTree) drawTreeRow(
	buf *buffer.Buffer,
	area geometry.Rect,
	n widget.TreeNode,
	selected,
	focused bool,
	theme *style.Theme,
) {
	st := theme.Text

	if selected {
		if focused {
			st = theme.Selected
		} else {
			st = theme.Info
		}
	}

	buf.FillRect(
		area.X,
		area.Y,
		area.W,
		1,
		' ',
		st,
	)

	icon := fileIcon(n.Label)

	if n.Label == ".." {
		icon = "↰"
	} else if n.HasChildren {
		icon = "▸"

		if n.Expanded {
			icon = "▾"
		}
	}

	buf.SetString(
		area.X+1,
		area.Y,
		strings.Repeat("  ", n.Depth)+icon+" "+n.Label,
		st,
	)
}

func (t *FileTree) Draw(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	buf.FillRect(
		area.X,
		area.Y,
		area.W,
		area.H,
		' ',
		theme.Panel,
	)

	if area.W < 3 || area.H < 1 {
		return
	}

	buf.SetString(
		area.X+1,
		area.Y,
		"EXPLORER",
		theme.Title,
	)

	toolbar := []struct {
		label string
		mode  fileTreeModal
	}{
		{"[R]", modalNone},
		{"[+F]", modalNewFile},
		{"[+D]", modalNewFolder},
		{"[F2]", modalRename},
		{"[X]", modalNone},
	}

	x := area.X + 1

	for i, b := range toolbar {
		st := theme.TextMuted

		if i == 0 && t.IsFocused() {
			st = theme.Info
		}

		if t.modal != modalNone &&
			b.mode == t.modal {
			st = theme.Selected
		}

		buf.SetString(
			x,
			area.Y+1,
			b.label,
			st,
		)

		x += len([]rune(b.label)) + 1

		if x >= area.X+area.W-2 {
			break
		}
	}

	if area.H <= 2 {
		return
	}

	t.TreeView.Draw(
		buf,
		geometry.Rect{
			X: area.X,
			Y: area.Y + 2,
			W: area.W,
			H: area.H - 2,
		},
		theme,
	)

	if t.modal != modalNone || t.confirmDelete {
		t.drawModal(buf, area, theme)
	}
}

func (t *FileTree) drawModal(
	buf *buffer.Buffer,
	area geometry.Rect,
	theme *style.Theme,
) {
	w := area.W - 4

	if w > 42 {
		w = 42
	}

	if w < 12 {
		w = area.W - 2
	}

	h := 5

	x := area.X + (area.W-w)/2
	y := area.Y + (area.H-h)/2

	if x < area.X+1 {
		x = area.X + 1
	}

	if y < area.Y+1 {
		y = area.Y + 1
	}

	fill := theme.Panel

	buffer.DrawBorder(
		buf,
		x,
		y,
		w,
		h,
		"",
		theme.BorderFocus,
		theme.Title,
		fill,
		true,
	)

	title := ""

	switch t.modal {
	case modalNewFile:
		title = " New File "
	case modalNewFolder:
		title = " New Folder "
	case modalRename:
		title = " Rename "
	default:
		title = " Delete "
	}

	buf.SetString(
		x+2,
		y,
		title,
		theme.Title,
	)

	if t.confirmDelete {
		buf.SetString(
			x+2,
			y+2,
			"Delete selected item?",
			theme.Warning,
		)

		buf.SetString(
			x+2,
			y+3,
			"Y = delete   N / Esc = cancel",
			theme.TextMuted,
		)

		return
	}

	buf.SetString(
		x+2,
		y+2,
		string(t.modalValue),
		theme.Text,
	)

	buf.Set(
		x+2+len(t.modalValue),
		y+2,
		' ',
		theme.Selected,
	)

	buf.SetString(
		x+2,
		y+3,
		"Enter = confirm   Esc = cancel",
		theme.TextMuted,
	)

	if t.modalError != "" {
		buf.SetString(
			x+2,
			y+4,
			t.modalError,
			theme.Negative,
		)
	}
}

func (t *FileTree) HandleKey(k input.Key) bool {
	if t.modal != modalNone || t.confirmDelete {
		return t.handleModalKey(k)
	}

	if k.Type == input.KeyRune {
		if k.Mods&input.ModCtrl != 0 && k.Rune == 'r' {
			t.Refresh()
			return true
		}

		switch k.Rune {
		case 'r':
			t.Refresh()
			return true

		case 'n', 'f':
			t.beginModal(modalNewFile)
			return true

		case 'd':
			t.beginModal(modalNewFolder)
			return true

		case 'x':
			t.beginDelete()
			return true
		}
	}

	if k.Type == input.KeyDelete {
		t.beginDelete()
		return true
	}

	if k.Type == input.KeyF2 {
		t.beginModal(modalRename)
		return true
	}

	return t.TreeView.HandleKey(k)
}

func (t *FileTree) beginModal(m fileTreeModal) {
	n := t.selectedNode()

	if n == nil || n.parentEntry {
		return
	}

	t.modal = m
	t.modalValue = nil
	t.modalError = ""

	if m == modalRename && n != nil {
		t.modalValue = []rune(n.name)
	}
}

func (t *FileTree) beginDelete() {
	n := t.selectedNode()

	if n == nil || n.parentEntry {
		return
	}

	t.confirmDelete = true
	t.modalError = ""
}

func (t *FileTree) handleModalKey(k input.Key) bool {
	if t.confirmDelete {
		if k.Type == input.KeyEsc {
			t.confirmDelete = false
			return true
		}

		if k.Type == input.KeyRune &&
			(k.Rune == 'y' || k.Rune == 'Y') {
			t.deleteSelected()
			return true
		}

		if k.Type == input.KeyRune &&
			(k.Rune == 'n' || k.Rune == 'N') {
			t.confirmDelete = false
			return true
		}

		return true
	}

	if k.Type == input.KeyEsc {
		t.modal = modalNone
		t.modalValue = nil
		t.modalError = ""
		return true
	}

	switch k.Type {
	case input.KeyBackspace:
		if len(t.modalValue) > 0 {
			t.modalValue = t.modalValue[:len(t.modalValue)-1]
		}

		return true

	case input.KeyEnter:
		t.submitModal()
		return true

	case input.KeyRune, input.KeySpace:
		if k.Rune >= 32 &&
			k.Rune != '/' &&
			k.Rune != '\\' &&
			len(t.modalValue) < 160 {
			t.modalValue = append(t.modalValue, k.Rune)
		}

		return true
	}

	return true
}

func (t *FileTree) submitModal() {
	name := strings.TrimSpace(string(t.modalValue))

	if name == "" ||
		name == "." ||
		name == ".." {
		t.modalError = "Enter a valid name"
		return
	}

	if strings.ContainsAny(name, `/\`) {
		t.modalError = "Use a single file/folder name"
		return
	}

	if t.selectedNode() == nil {
		t.modalError = "Nothing selected"
		return
	}

	var ok bool

	switch t.modal {
	case modalNewFile:
		ok = t.NewFile(name)

	case modalNewFolder:
		ok = t.NewFolder(name)

	case modalRename:
		ok = t.RenameSelected(name)
	}

	if !ok {
		t.modalError = "Could not complete operation"
		return
	}

	t.modal = modalNone
	t.modalValue = nil
	t.modalError = ""
}

func (t *FileTree) deleteSelected() {
	if !t.DeleteSelected() {
		t.modalError = "Could not delete selected item"
		t.confirmDelete = true
		return
	}

	t.confirmDelete = false
	t.modalError = ""
}

func (t *FileTree) activateID(id string) {
	t.activate(t.findNode(id))
}

func (t *FileTree) activate(n *node) bool {
	if n == nil {
		return false
	}

	// The virtual ".." entry navigates directly to the parent
	// directory represented by this entry.
	if n.parentEntry {
		if samePath(n.path, t.Root) {
			return true
		}

		t.Root = filepath.Clean(n.path)
		t.reload()

		if t.TreeView != nil {
			t.TreeView.Selected = 0
		}

		return true
	}

	if n.dir {
		t.load(n)
		n.expanded = !n.expanded
		t.TreeView.Invalidate()
		return true
	}

	if t.OnOpen != nil {
		t.OnOpen(n.path)
	}

	return true
}

func (t *FileTree) HandleMouse(
	ev input.MouseEvent,
	area geometry.Rect,
) bool {
	if !area.Contains(ev.X, ev.Y) {
		return false
	}

	if t.modal != modalNone || t.confirmDelete {
		return true
	}

	if ev.Action == input.MousePress &&
		ev.Y == area.Y+1 {
		x := area.X + 1

		tool := []struct {
			w      int
			action int
		}{
			{3, 0},
			{4, 1},
			{4, 2},
			{4, 3},
			{3, 4},
		}

		for _, b := range tool {
			if ev.X >= x && ev.X < x+b.w {
				switch b.action {
				case 0:
					t.Refresh()

				case 1:
					t.beginModal(modalNewFile)

				case 2:
					t.beginModal(modalNewFolder)

				case 3:
					t.beginModal(modalRename)

				case 4:
					t.beginDelete()
				}

				return true
			}

			x += b.w + 1

			if x >= area.X+area.W {
				break
			}
		}

		return true
	}

	if ev.Y < area.Y+2 {
		return true
	}

	return t.TreeView.HandleMouse(
		ev,
		geometry.Rect{
			X: area.X,
			Y: area.Y + 2,
			W: area.W,
			H: area.H - 2,
		},
	)
}

func (t *FileTree) firstFile() *node {
	var walk func(*node) *node

	walk = func(n *node) *node {
		if n == nil {
			return nil
		}

		if n.parentEntry {
			return nil
		}

		if !n.dir {
			return n
		}

		t.load(n)

		for _, c := range n.children {
			if c.parentEntry {
				continue
			}

			if x := walk(c); x != nil {
				return x
			}
		}

		return nil
	}

	if len(t.nodes) > 0 {
		return walk(t.nodes[0])
	}

	return nil
}
func fileIcon(name string) string {
	switch strings.ToLower(filepath.Base(name)) {
	case "dockerfile", "dockerfile.dev":
		return "🐳"
	case "makefile", "cmakelists.txt":
		return "🏗️"
	case ".gitignore", ".gitattributes", ".gitmodules":
		return "🌿"
	case "license", "license.txt", "license.md":
		return "📜"
	case ".env", ".env.local", ".env.example":
		return "🔑"
	}
	switch strings.ToLower(
		filepath.Ext(name),
	) {
	case ".go":
		return "🐹"
	case ".py", ".pyw":
		return "🐍"
	case ".js", ".cjs", ".mjs":
		return "🟨"
	case ".ts", ".mts", ".cts":
		return "🟦"
	case ".jsx", ".tsx":
		return "⚛️"
	case ".java", ".class", ".jar":
		return "☕"
	case ".c", ".h":
		return "⚙️"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "⚡"
	case ".cs":
		return "🎯"
	case ".rs":
		return "🦀"
	case ".php":
		return "🐘"
	case ".rb":
		return "💎"
	case ".swift":
		return "🍊"
	case ".kt", ".kts":
		return "🟣"
	case ".dart":
		return "🎯"
	case ".zig":
		return "🦎"
	case ".lua":
		return "🌙"
	case ".r", ".rmd":
		return "📊"
	case ".jl":
		return "🔬"
	case ".ex", ".exs":
		return "🧪"
	case ".pl":
		return "🐪"

	// Web & Styling
	case ".html", ".htm":
		return "🌐"
	case ".css", ".scss", ".sass", ".less":
		return "🎨"
	case ".vue":
		return "💚"
	case ".svelte", ".astro":
		return "🔥"

	// Shell & Command Scripts
	case ".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd":
		return "🐚"

	default:
		return "·"
	}
}
