package widget

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/style"
)

// FilePicker is a searchable interactive file picker. It indexes a directory
// on SetRoot and only exposes regular text/code/log files.
type filePickerEntry struct {
	path  string
	rel   string
	lower string
}

type FilePicker struct {
	ThemeOverride *style.Theme
	FocusMixin
	Root             string
	Query            string
	queryLower       string
	queryLowerSource string
	Files            []string
	Filtered         []string
	Selected         int
	OnSelect         func(path string)
	OnCancel         func()
	Background       *color.Color
	scroll           int
	entries          []filePickerEntry
	filteredRel      []string
}

func NewFilePicker(root string) *FilePicker { p := &FilePicker{}; p.SetRoot(root); return p }
func (p *FilePicker) OwnsBackground() bool  { return p.Background != nil }
func (p *FilePicker) SetRoot(root string) {
	p.Root = root
	p.Files = p.Files[:0]
	p.entries = p.entries[:0]
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if isTextEditorFile(d.Name()) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			p.Files = append(p.Files, path)
			p.entries = append(p.entries, filePickerEntry{path: path, rel: rel, lower: strings.ToLower(rel)})
		}
		return nil
	})
	sort.SliceStable(p.entries, func(i, j int) bool { return p.entries[i].path < p.entries[j].path })
	for i := range p.entries {
		p.Files[i] = p.entries[i].path
	}
	p.refresh()
}
func (p *FilePicker) refresh() {
	p.Filtered = p.Filtered[:0]
	p.filteredRel = p.filteredRel[:0]
	if p.queryLowerSource != p.Query {
		p.queryLower = strings.ToLower(p.Query)
		p.queryLowerSource = p.Query
	}
	q := p.queryLower
	for _, e := range p.entries {
		if q == "" || strings.Contains(e.lower, q) {
			p.Filtered = append(p.Filtered, e.path)
			p.filteredRel = append(p.filteredRel, e.rel)
		}
	}
	p.Selected = 0
	p.scroll = 0
}

// SetQuery updates the picker query and caches its case-folded representation.
// This is useful for programmatic search and keeps repeated filtering allocation-free.
func (p *FilePicker) SetQuery(query string) {
	p.Query = query
	p.queryLower = strings.ToLower(query)
	p.queryLowerSource = query
	p.refresh()
}

func (p *FilePicker) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if p.ThemeOverride != nil {
		theme = p.ThemeOverride
	}
	if area.W < 1 || area.H < 1 {
		return
	}
	if p.Background != nil {
		buf.FillRect(area.X, area.Y, area.W, area.H, ' ', bgOr(theme.Panel, p.Background))
	}
	buf.SetString(area.X, area.Y, "FILES  ", bgOr(theme.Title, p.Background))
	if p.Query != "" {
		buf.SetString(area.X+8, area.Y, p.Query, bgOr(theme.Title, p.Background))
	}
	for r := 1; r < area.H; r++ {
		i := p.scroll + r - 1
		if i >= len(p.filteredRel) {
			break
		}
		st := bgOr(theme.Text, p.Background)
		if i == p.Selected {
			st = theme.Selected
		}
		buf.SetString(area.X, area.Y+r, p.filteredRel[i], st)
	}
}
func (p *FilePicker) HandleKey(k input.Key) bool {
	switch k.Type {
	case input.KeyUp:
		if p.Selected > 0 {
			p.Selected--
		}
		return true
	case input.KeyDown:
		if p.Selected < len(p.Filtered)-1 {
			p.Selected++
		}
		return true
	case input.KeyEnter:
		if len(p.Filtered) > 0 && p.OnSelect != nil {
			p.OnSelect(p.Filtered[p.Selected])
		}
		return true
	case input.KeyEsc:
		if p.OnCancel != nil {
			p.OnCancel()
		}
		return true
	case input.KeyBackspace:
		if len(p.Query) > 0 {
			r := []rune(p.Query)
			p.Query = string(r[:len(r)-1])
			p.queryLower = strings.ToLower(p.Query)
			p.refresh()
		}
		return true
	case input.KeyRune, input.KeySpace:
		p.Query += string(k.Rune)
		p.queryLower = strings.ToLower(p.Query)
		p.queryLowerSource = p.Query
		p.refresh()
		return true
	}
	return false
}
func (p *FilePicker) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if ev.Action != input.MousePress || !area.Contains(ev.X, ev.Y) {
		return false
	}
	i := p.scroll + ev.Y - area.Y - 1
	if i >= 0 && i < len(p.Filtered) {
		p.Selected = i
		if p.OnSelect != nil {
			p.OnSelect(p.Filtered[i])
		}
	}
	return true
}

func isTextEditorFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".rs", ".c", ".h", ".cpp", ".cc", ".hpp", ".java", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".php", ".swift", ".kt", ".kts", ".sh", ".bash", ".zsh", ".fish", ".sql", ".json", ".yaml", ".yml", ".toml", ".xml", ".html", ".htm", ".css", ".scss", ".md", ".markdown", ".txt", ".log", ".conf", ".ini", ".env", ".properties", ".proto", ".graphql", ".vue", ".svelte":
		return true
	}
	return false
}
