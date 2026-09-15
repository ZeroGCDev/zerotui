package widget

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/clipboard"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/input"
	"github.com/ZeroGCDev/zerotui/numfmt"
	"github.com/ZeroGCDev/zerotui/style"
)

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// TextEditor is a reusable editable text widget with line numbers, syntax
// highlighting, folding, selection, scrolling, undo/redo and search. It owns
// editing/rendering state but deliberately delegates persistence to OnSave.
type TextEditor struct {
	FocusMixin
	// ThemeOverride optionally replaces the application theme for this source editor.
	// It exposes the richer editor/syntax palette without changing the widget API.
	ThemeOverride *style.EditorTheme
	// Document is the text model. CodeEditor owns presentation and interaction state.
	*Document
	// Rehighlight is an optional presentation hook installed by CodeEditor.
	Rehighlight func(*TextEditor)
	OnSave      func() bool
	// Modified is retained for source compatibility; Document is authoritative.
	Modified  bool
	Tokens    []SyntaxToken
	Folds     map[int]FoldRange
	Collapsed map[int]bool
	hidden    []bool
	Cursor    int
	Col       int
	Scroll    int
	HScroll   int
	ReadOnly  bool
	// ShowStatusBar controls the editor-owned footer. Embedding shells such as
	// the IDE can disable it when they provide their own status bar.
	ShowStatusBar    bool
	Search           string
	Searching        bool
	GotoLine         bool
	searchHit        int
	anchorLine       int
	anchorCol        int
	hasAnchor        bool
	clipboard        string
	contextMenu      bool
	contextMenuX     int
	contextMenuY     int
	contextMenuHover int
	undo             []editSnapshot
	redo             []editSnapshot
	status           string
	dragging         bool
	manualScroll     bool // true while the viewport was moved by the mouse wheel
	viewportH        int  // last rendered source-line viewport height
	viewportW        int  // last rendered source-line viewport width
	editorTheme      style.EditorTheme
	editorThemeBase  style.Theme
	editorThemeValid bool
	observedRevision uint64 // last document revision reflected in this view
	// tokenCache is used by CodeEditor for very large documents. Keeping syntax
	// tokens only for the visible window avoids retaining a second document-sized
	// representation while preserving the public Tokens API for normal files.
	tokenCache           map[int][]SyntaxToken
	prepareVisibleTokens func(startLine, endLine int)
	unsubscribeDoc       func()
	// Typing coalescing keeps a burst of adjacent character inserts as one
	// undo step without changing the document history contract for structural edits.
	typingActive bool
	typingLine   int
	typingEndCol int
	// lastNewline* lets CodeEditor update fold coordinates without a full
	// document scan after an Enter. The old line is retained only for the
	// duration needed by the synchronous document notification.
	lastNewlineOldLine string
	lastNewlineCol     int
	lastEditWasNewline bool
	suppressDocSync    bool
	OnDocumentChange   func(*TextEditor, DocumentChange)
}

type editSnapshot struct {
	Cursor, Col, Scroll, HScroll int
}

func NewTextEditor() *TextEditor {
	v := &TextEditor{
		Folds:         make(map[int]FoldRange),
		Collapsed:     make(map[int]bool),
		ShowStatusBar: true,
	}
	v.SetDocument(NewDocument(""))
	return v
}

// SetLanguage selects syntax/comment conventions without coupling the widget
// to a filesystem. Use names such as "go", "python", "json", or "text".
func (v *TextEditor) SetLanguage(language string) {
	v.Document.SetLanguage(language)
	v.rehighlight()
}

// SetDocumentName sets display metadata used by the editor header and syntax providers.
func (v *TextEditor) SetDocumentName(name string) { v.Document.SetName(name) }

// SetDocument replaces the text model without taking ownership of persistence.
// The editor resets its view/undo state and keeps the document model reusable by other widgets.
func (v *TextEditor) SetDocument(doc *Document) {
	if v.unsubscribeDoc != nil {
		v.unsubscribeDoc()
		v.unsubscribeDoc = nil
	}
	if doc == nil {
		doc = NewDocument("")
	}
	v.Document = doc
	v.Tokens = nil
	v.tokenCache = nil
	v.prepareVisibleTokens = nil
	v.Folds = make(map[int]FoldRange)
	v.Collapsed = make(map[int]bool)
	v.hidden = make([]bool, len(v.Lines))
	v.Cursor, v.Col, v.Scroll, v.HScroll = 0, 0, 0, 0
	v.Searching, v.hasAnchor = false, false
	v.undo, v.redo = nil, nil
	v.typingActive = false
	v.status = ""
	v.rehighlight()
	v.observedRevision = v.Document.Revision
	v.unsubscribeDoc = v.Document.Subscribe(func(change DocumentChange) {
		if v.suppressDocSync || v.Document == nil || change.Revision == v.observedRevision {
			return
		}
		if v.OnDocumentChange != nil {
			v.OnDocumentChange(v, change)
		} else {
			v.rehighlight()
		}
		v.Modified = v.Document.Modified()
		v.observedRevision = change.Revision
		v.clamp()
	})
}
func (v *TextEditor) OwnsBackground() bool { return true }

const editorTabWidth = 4

func (v *TextEditor) rehighlight() {
	if v.Rehighlight != nil {
		v.Rehighlight(v)
	}
	v.rebuildHiddenIndex()
}

// tokenRangeForLine locates a line's tokens without allocating a slice for
// every document line. This keeps large documents cheap to load while still
// making visible-line rendering fast. Providers emit tokens in source order.
func (v *TextEditor) tokenRangeForLine(line int) (int, int) {
	if line < 0 || len(v.Tokens) == 0 {
		return 0, 0
	}
	start := sort.Search(len(v.Tokens), func(i int) bool { return v.Tokens[i].Line >= line })
	if start >= len(v.Tokens) || v.Tokens[start].Line != line {
		return start, start
	}
	end := start + 1
	for end < len(v.Tokens) && v.Tokens[end].Line == line {
		end++
	}
	return start, end
}

// rebuildHiddenIndex converts collapsed fold ranges into an O(1) per-line
// visibility index. A difference array keeps rebuilding O(lines + folds) and
// avoids scanning every fold for every visible line during rendering.
func (v *TextEditor) rebuildHiddenIndex() {
	n := len(v.Lines)
	v.hidden = make([]bool, n)
	if n == 0 || len(v.Collapsed) == 0 {
		return
	}
	diff := make([]int, n+1)
	for start, f := range v.Folds {
		if !v.Collapsed[start] {
			continue
		}
		from := start + 1
		to := f.End + 1
		if from < 0 {
			from = 0
		}
		if from >= n {
			continue
		}
		if to > n {
			to = n
		}
		if to <= from {
			continue
		}
		diff[from]++
		diff[to]--
	}
	active := 0
	for i := 0; i < n; i++ {
		active += diff[i]
		v.hidden[i] = active > 0
	}
}
func (v *TextEditor) snapshot() editSnapshot {
	return editSnapshot{v.Cursor, v.Col, v.Scroll, v.HScroll}
}
func (v *TextEditor) restore(s editSnapshot) {
	v.Cursor = s.Cursor
	v.Col = s.Col
	v.Scroll = s.Scroll
	v.HScroll = s.HScroll
	// Content restoration already emits a DocumentChange synchronously. The
	// document subscriber updates syntax/fold state before Undo/Redo returns,
	// so rehighlighting here would scan the same document range a second time.
	v.clamp()
}
func (v *TextEditor) recordEdit() {
	v.lastEditWasNewline = false
	v.lastNewlineOldLine = ""
	start, end := v.editRangeHint()
	v.Document.BeginEditRange(start, end)
	v.undo = append(v.undo, v.snapshot())
	if len(v.undo) > 100 {
		v.undo = v.undo[1:]
	}
	v.redo = nil
}

func (v *TextEditor) recordEditRange(start, end int) {
	v.lastEditWasNewline = false
	v.lastNewlineOldLine = ""
	v.Document.BeginEditRange(start, end)
	v.undo = append(v.undo, v.snapshot())
	if len(v.undo) > 100 {
		v.undo = v.undo[1:]
	}
	v.redo = nil
}

func (v *TextEditor) editRangeHint() (int, int) {
	n := len(v.Lines)
	if n == 0 {
		return 0, 0
	}
	if sl, _, el, _, ok := v.selectedRange(); ok {
		if el >= n {
			el = n - 1
		}
		return sl, el + 1
	}
	start, end := v.Cursor, v.Cursor+1
	if v.Col == 0 && v.Cursor > 0 {
		start = v.Cursor - 1
	} else if v.Col >= utf8.RuneCountInString(v.Lines[v.Cursor]) && v.Cursor+1 < n {
		end = v.Cursor + 2
	}
	return start, end
}
func (v *TextEditor) markChanged() {
	v.Document.CommitEdit()
	v.Modified = true
	v.clamp()
	v.ensureCursorVisible()
	v.status = "Modified"
}
func (v *TextEditor) Undo() bool {
	if len(v.undo) == 0 {
		return false
	}
	cur := v.snapshot()
	prev := v.undo[len(v.undo)-1]
	v.undo = v.undo[:len(v.undo)-1]
	v.redo = append(v.redo, cur)
	if !v.Document.UndoContent() {
		return false
	}
	v.restore(prev)
	v.Modified = v.Document.Modified()
	v.observedRevision = v.Document.Revision
	v.status = "Undo"
	return true
}
func (v *TextEditor) Redo() bool {
	if len(v.redo) == 0 {
		return false
	}
	cur := v.snapshot()
	next := v.redo[len(v.redo)-1]
	v.redo = v.redo[:len(v.redo)-1]
	v.undo = append(v.undo, cur)
	if !v.Document.RedoContent() {
		return false
	}
	v.restore(next)
	v.Modified = v.Document.Modified()
	v.observedRevision = v.Document.Revision
	v.status = "Redo"
	return true
}

func (v *TextEditor) clamp() {
	if len(v.Lines) == 0 {
		v.Lines = []string{""}
	}
	if v.Cursor < 0 {
		v.Cursor = 0
	}
	if v.Cursor >= len(v.Lines) {
		v.Cursor = len(v.Lines) - 1
	}
	runeCount := utf8.RuneCountInString(v.Lines[v.Cursor])
	if v.Col < 0 {
		v.Col = 0
	}
	if v.Col > runeCount {
		v.Col = runeCount
	}
	if v.Scroll < 0 {
		v.Scroll = 0
	}
	if v.HScroll < 0 {
		v.HScroll = 0
	}
}
func (v *TextEditor) setCursor(line, col int, extend bool) {
	if line < 0 {
		line = 0
	}
	if line >= len(v.Lines) {
		line = len(v.Lines) - 1
	}
	r := []rune(v.Lines[line])
	if col < 0 {
		col = 0
	}
	if col > len(r) {
		col = len(r)
	}
	if extend {
		if !v.hasAnchor {
			v.anchorLine, v.anchorCol = v.Cursor, v.Col
			v.hasAnchor = true
		}
	} else {
		v.hasAnchor = false
	}
	v.Cursor, v.Col = line, col
	v.ensureCursorVisible()
}

// ensureCursorVisible keeps keyboard/mouse editing anchored to the caret.
// Mouse-wheel scrolling deliberately sets manualScroll=true, while any actual
// cursor movement/edit clears it first in HandleKey, so typing near the bottom
// of the viewport follows the caret just like a desktop IDE.
func (v *TextEditor) ensureCursorVisible() {
	h, w := v.viewportH, v.viewportW
	if h <= 0 {
		return
	}
	if v.Cursor < v.Scroll {
		v.Scroll = v.Cursor
	}
	if v.Cursor >= v.Scroll+h {
		v.Scroll = v.Cursor - h + 1
	}
	if w > 0 && v.Cursor >= 0 && v.Cursor < len(v.Lines) {
		cursorX := displayColumnAtRune(v.Lines[v.Cursor], v.Col)
		if cursorX < v.HScroll {
			v.HScroll = cursorX
		}
		if cursorX >= v.HScroll+w {
			v.HScroll = cursorX - w + 1
		}
	}
	if v.Scroll < 0 {
		v.Scroll = 0
	}
	if v.HScroll < 0 {
		v.HScroll = 0
	}
}

func (v *TextEditor) selectedRange() (sl, sc, el, ec int, ok bool) {
	if !v.hasAnchor {
		return 0, 0, 0, 0, false
	}
	sl, sc, el, ec = v.anchorLine, v.anchorCol, v.Cursor, v.Col
	if sl > el || (sl == el && sc > ec) {
		sl, el = el, sl
		sc, ec = ec, sc
	}
	return sl, sc, el, ec, true
}
func (v *TextEditor) deleteSelection() bool {
	sl, sc, el, ec, ok := v.selectedRange()
	if !ok {
		return false
	}
	if sl == el {
		r := []rune(v.Lines[sl])
		v.Lines[sl] = string(append(r[:sc], r[ec:]...))
	} else {
		first := []rune(v.Lines[sl])
		last := []rune(v.Lines[el])
		v.Lines[sl] = string(append(first[:sc], last[ec:]...))
		v.Lines = append(v.Lines[:el], v.Lines[el+1:]...)
		v.setCursor(sl, sc, false)
	}
	v.hasAnchor = false
	return true
}
func (v *TextEditor) finishEdit(typing bool, line, startCol int) {
	v.markChanged()
	if !typing {
		v.typingActive = false
		return
	}
	if v.typingActive && v.typingLine == line && v.typingEndCol == startCol && len(v.undo) >= 2 {
		// The newest document history entry contains the state produced by the
		// previous character. The older entry already contains the state before
		// the whole typing burst, so discard the redundant middle checkpoint.
		v.undo = v.undo[:len(v.undo)-1]
		v.Document.coalesceLastUndo()
	}
	v.typingActive = true
	v.typingLine = line
	v.typingEndCol = v.Col
}

func (v *TextEditor) insertRune(r rune) {
	if v.ReadOnly {
		return
	}
	if v.hasAnchor {
		v.typingActive = false
		v.recordEdit()
		v.deleteSelection()
		line := []rune(v.Lines[v.Cursor])
		line = append(line, 0)
		copy(line[v.Col+1:], line[v.Col:])
		line[v.Col] = r
		v.Lines[v.Cursor] = string(line)
		v.Col++
		v.markChanged()
		return
	}
	// Basic editor conveniences: insert matching pairs and skip over an
	// existing closing pair instead of duplicating it.
	if r == ')' || r == ']' || r == '}' || r == '"' || r == '\'' || r == '`' {
		line := []rune(v.Lines[v.Cursor])
		if v.Col < len(line) && line[v.Col] == r {
			v.typingActive = false
			v.Col++
			return
		}
	}
	lineNo, startCol := v.Cursor, v.Col
	v.recordEdit()
	line := []rune(v.Lines[v.Cursor])
	pair := rune(0)
	switch r {
	case '(':
		pair = ')'
	case '[':
		pair = ']'
	case '{':
		pair = '}'
	case '"':
		pair = '"'
	case '\'':
		pair = '\''
	case '`':
		pair = '`'
	}
	if pair != 0 {
		v.typingActive = false
		line = append(line, 0, 0)
		copy(line[v.Col+2:], line[v.Col:len(line)-2])
		line[v.Col], line[v.Col+1] = r, pair
		v.Lines[v.Cursor] = string(line)
		v.Col++
		v.markChanged()
	} else {
		line = append(line, 0)
		copy(line[v.Col+1:], line[v.Col:])
		line[v.Col] = r
		v.Lines[v.Cursor] = string(line)
		v.Col++
		v.finishEdit(true, lineNo, startCol)
	}
}

func (v *TextEditor) insertNewline() {
	v.typingActive = false
	if v.ReadOnly {
		return
	}
	v.lastEditWasNewline = false
	v.recordEdit()
	v.deleteSelection()
	line := []rune(v.Lines[v.Cursor])
	v.lastNewlineOldLine = string(line)
	v.lastNewlineCol = v.Col
	v.lastEditWasNewline = true
	left, right := string(line[:v.Col]), string(line[v.Col:])
	indent := leadingIndent(left)
	trimmed := strings.TrimRight(left, " \t")
	if strings.HasSuffix(trimmed, "{") {
		indent += "    "
	}
	v.Lines[v.Cursor] = left

	// Structural edits are on the hot path when Enter is held down. Grow the
	// line slice only when necessary, then shift the tail in-place. The previous
	// nested append allocated a fresh slice and copied the entire document tail
	// on every Enter, which made sustained editing of large files progressively
	// stall.
	insertAt := v.Cursor + 1
	if len(v.Lines) == cap(v.Lines) {
		n := cap(v.Lines) * 2
		if n < len(v.Lines)+1 {
			n = len(v.Lines) + 1
		}
		lines := make([]string, len(v.Lines), n)
		copy(lines, v.Lines)
		v.Lines = lines
	}
	v.Lines = v.Lines[:len(v.Lines)+1]
	copy(v.Lines[insertAt+1:], v.Lines[insertAt:len(v.Lines)-1])
	v.Lines[insertAt] = indent + right
	v.Cursor++
	v.Col = len([]rune(indent))
	v.markChanged()
}
func (v *TextEditor) backspace() {
	v.typingActive = false
	if v.ReadOnly {
		return
	}
	if v.hasAnchor {
		v.recordEdit()
		v.deleteSelection()
		v.markChanged()
		return
	}
	if v.Col > 0 {
		v.recordEdit()
		r := []rune(v.Lines[v.Cursor])
		v.Lines[v.Cursor] = string(append(r[:v.Col-1], r[v.Col:]...))
		v.Col--
		v.markChanged()
		return
	}
	if v.Cursor > 0 {
		v.recordEdit()
		prev := []rune(v.Lines[v.Cursor-1])
		cur := []rune(v.Lines[v.Cursor])
		pos := len(prev)
		v.Lines[v.Cursor-1] = string(append(prev, cur...))
		v.Lines = append(v.Lines[:v.Cursor], v.Lines[v.Cursor+1:]...)
		v.Cursor--
		v.Col = pos
		v.markChanged()
	}
}
func (v *TextEditor) deleteForward() {
	v.typingActive = false
	if v.ReadOnly {
		return
	}
	if v.hasAnchor {
		v.recordEdit()
		v.deleteSelection()
		v.markChanged()
		return
	}
	r := []rune(v.Lines[v.Cursor])
	if v.Col < len(r) {
		v.recordEdit()
		v.Lines[v.Cursor] = string(append(r[:v.Col], r[v.Col+1:]...))
		v.markChanged()
		return
	}
	if v.Cursor < len(v.Lines)-1 {
		v.recordEdit()
		v.Lines[v.Cursor] = v.Lines[v.Cursor] + v.Lines[v.Cursor+1]
		v.Lines = append(v.Lines[:v.Cursor+1], v.Lines[v.Cursor+2:]...)
		v.markChanged()
	}
}
func (v *TextEditor) copySelection(cut bool) bool {
	sl, sc, el, ec, ok := v.selectedRange()
	if !ok {
		return false
	}
	var parts []string
	if sl == el {
		r := []rune(v.Lines[sl])
		v.clipboard = string(r[sc:ec])
	} else {
		parts = append(parts, string([]rune(v.Lines[sl])[sc:]))
		for i := sl + 1; i < el; i++ {
			parts = append(parts, v.Lines[i])
		}
		parts = append(parts, string([]rune(v.Lines[el])[:ec]))
		v.clipboard = strings.Join(parts, "\n")
	}
	_ = clipboard.Copy(v.clipboard)
	if cut && !v.ReadOnly {
		v.recordEdit()
		v.deleteSelection()
		v.markChanged()
	}
	return true
}
func (v *TextEditor) paste() {
	if v.ReadOnly {
		return
	}
	if external, ok := clipboard.Paste(); ok {
		v.clipboard = external
	}
	v.pasteText(v.clipboard)
}

func (v *TextEditor) pasteText(text string) {
	v.typingActive = false
	if v.ReadOnly || text == "" {
		return
	}
	v.recordEdit()
	v.deleteSelection()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	pieces := strings.Split(text, "\n")
	line := []rune(v.Lines[v.Cursor])
	left, right := string(line[:v.Col]), string(line[v.Col:])
	if len(pieces) == 1 {
		v.Lines[v.Cursor] = left + pieces[0] + right
		v.Col += len([]rune(pieces[0]))
	} else {
		out := make([]string, 0, len(pieces))
		out = append(out, left+pieces[0])
		out = append(out, pieces[1:len(pieces)-1]...)
		out = append(out, pieces[len(pieces)-1]+right)
		v.Lines = append(v.Lines[:v.Cursor], append(out, v.Lines[v.Cursor+1:]...)...)
		v.Cursor += len(pieces) - 1
		v.Col = len([]rune(pieces[len(pieces)-1]))
	}
	v.markChanged()
}

// HandlePaste implements widget.PasteHandler so bracketed paste bypasses the
// key-by-key parser and remains one undoable editor operation.
func (v *TextEditor) HandlePaste(text string) bool {
	if v.ReadOnly || text == "" {
		return false
	}
	v.pasteText(text)
	return true
}

func leadingIndent(s string) string {
	end := 0
	for end < len(s) && (s[end] == ' ' || s[end] == '\t') {
		end++
	}
	return expandTabs(s[:end], editorTabWidth)
}

func (v *TextEditor) outdentLine() {
	v.typingActive = false
	line := []rune(v.Lines[v.Cursor])
	n := 0
	for n < len(line) && n < editorTabWidth && line[n] == ' ' {
		n++
	}
	if n == 0 {
		return
	}
	v.recordEdit()
	v.Lines[v.Cursor] = string(line[n:])
	if v.Col >= n {
		v.Col -= n
	} else {
		v.Col = 0
	}
	v.markChanged()
}

func (v *TextEditor) moveLine(delta int, extend bool) {
	v.setCursor(v.Cursor+delta, v.Col, extend)
	v.clamp()
}
func (v *TextEditor) moveCol(delta int, extend bool) {
	col := v.Col + delta
	line := v.Cursor
	if col < 0 && line > 0 {
		line--
		col = len([]rune(v.Lines[line]))
	}
	if col > len([]rune(v.Lines[line])) && line < len(v.Lines)-1 {
		line++
		col = 0
	}
	v.setCursor(line, col, extend)
}
func (v *TextEditor) home(extend bool)            { v.setCursor(v.Cursor, 0, extend) }
func (v *TextEditor) end(extend bool)             { v.setCursor(v.Cursor, len([]rune(v.Lines[v.Cursor])), extend) }
func (v *TextEditor) page(delta int, extend bool) { v.setCursor(v.Cursor+delta, v.Col, extend) }

func (v *TextEditor) syncDocument() {
	if v.Document == nil {
		return
	}
	if v.observedRevision == v.Document.Revision {
		return
	}
	if v.OnDocumentChange != nil {
		v.OnDocumentChange(v, v.Document.LastChange)
	} else {
		v.rehighlight()
	}
	v.Modified = v.Document.Modified()
	v.observedRevision = v.Document.Revision
	v.clamp()
}

func (v *TextEditor) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	v.syncDocument()
	var et style.EditorTheme
	if v.ThemeOverride != nil {
		et = *v.ThemeOverride
		theme = &et.Theme
	} else if !v.editorThemeValid || v.editorThemeBase != *theme {
		et = *style.NewEditorTheme(theme)
		v.editorTheme = et
		v.editorThemeBase = *theme
		v.editorThemeValid = true
	} else {
		et = v.editorTheme
		theme = &et.Theme
	}
	buf.FillRect(area.X, area.Y, area.W, area.H, ' ', et.EditorBackground)
	if area.W < 8 || area.H < 2 {
		return
	}
	header := filepath.Base(v.Document.Name)
	if header == "." || header == "" {
		header = "EDITOR"
	}
	buf.SetString(area.X+1, area.Y, header, et.EditorTitle)
	if v.Modified {
		buf.SetString(area.X+1+utf8.RuneCountInString(header), area.Y, " •", et.EditorTitle)
	}
	if v.Searching {
		x := area.X + 1 + utf8.RuneCountInString(header) + 3
		prefix := "/"
		if v.GotoLine {
			prefix = ":"
		}
		buf.SetString(x, area.Y, prefix, et.EditorWarning)
		buf.SetString(x+1, area.Y, v.Search, et.EditorWarning)
	} else if !v.IsFocused() && v.status != "" {
		x := area.X + 1 + utf8.RuneCountInString(header) + 3
		buf.SetString(x, area.Y, v.status, et.EditorMuted)
	}
	numWidth := decimalWidth(len(v.Lines))
	if numWidth < 2 {
		numWidth = 2
	}
	codeX := area.X + numWidth + 4
	visible := area.H - 1
	if v.ShowStatusBar {
		visible = area.H - 2
	}
	if visible <= 0 {
		return
	}
	v.viewportH = visible
	v.viewportW = area.W - (codeX - area.X) - 1
	v.ensureScroll(visible, v.viewportW)
	if v.prepareVisibleTokens != nil {
		// Ask the presentation layer for a small source window. The callback is
		// deliberately invoked after scrolling is settled so mouse-wheel and
		// keyboard navigation never cause work for lines outside the viewport.
		end := v.Scroll + visible
		if end > len(v.Lines) {
			end = len(v.Lines)
		}
		v.prepareVisibleTokens(v.Scroll, end)
	}
	line := 0
	var numBuf [32]byte
	for i := v.Scroll; i < len(v.Lines) && line < visible; {
		if v.isHidden(i) {
			i++
			continue
		}
		y := area.Y + 1 + line
		st := et.Primary
		if i == v.Cursor {
			buf.FillRect(codeX, y, v.viewportW, 1, ' ', et.ActiveLine)
			st = st.WithBg(et.ActiveLine.Bg)
		}
		numBuf = [32]byte{}
		digits := numfmt.AppendInt(numBuf[:0], int64(i+1))
		if len(digits) < numWidth {
			pad := numWidth - len(digits)
			copy(numBuf[pad:pad+len(digits)], digits)
			for j := 0; j < pad; j++ {
				numBuf[j] = ' '
			}
			digits = numBuf[:numWidth]
		}
		buf.SetBytes(area.X+1, y, digits, func() style.Style {
			if i == v.Cursor {
				return et.ActiveLineNumber
			}
			return et.Gutter
		}())
		if _, ok := v.Folds[i]; ok {
			mark := "▾"
			if v.Collapsed[i] {
				mark = "▸"
			}
			buf.SetString(area.X+numWidth+2, y, mark, et.EditorInfo)
		}
		buf.SetString(area.X+numWidth+3, y, "│", et.WrapGuide)
		v.drawLine(buf, codeX, y, area.W-(codeX-area.X)-1, i, st, &et)
		line++
		// A collapsed fold hides an arbitrarily large contiguous range. Jump
		// directly over it instead of scanning every hidden source line.
		if f, ok := v.Folds[i]; ok && v.Collapsed[i] && f.End >= i {
			i = f.End + 1
		} else {
			i++
		}
	}

	// Draw the editor cursor as an ordinary highlighted cell. Do not use the
	// terminal's hardware cursor or SGR blink: zerotui owns the screen buffer,
	// and a software cursor works consistently across terminal emulators. The
	// Selected role is intentionally high-contrast in every built-in theme.
	if v.IsFocused() && CursorBlinkVisible() && v.Cursor >= v.Scroll {
		row := 0
		cursorRow := -1
		for i := v.Scroll; i < len(v.Lines) && row < visible; i++ {
			if v.isHidden(i) {
				continue
			}
			if i == v.Cursor {
				cursorRow = row
				break
			}
			row++
		}
		if cursorRow >= 0 {
			text := v.Lines[v.Cursor]
			cx := displayColumnAtRune(text, v.Col) - v.HScroll
			codeW := area.W - (codeX - area.X) - 1
			if codeW > 0 && cx >= 0 && cx < codeW {
				ch := runeAtString(text, v.Col)
				if ch == 0 {
					ch = ' '
				}
				buf.Set(codeX+cx, area.Y+1+cursorRow, ch, et.Cursor)
			}
		}
	}
	if v.ShowStatusBar {
		if v.status != "" {
			buf.SetString(area.X+1, area.Y+area.H-1, v.status, et.EditorMuted)
			if v.Modified {
				buf.SetString(area.X+1+utf8.RuneCountInString(v.status), area.Y+area.H-1, "  • unsaved", et.EditorMuted)
			}
		}
		position := "Ln " + strconv.Itoa(v.Cursor+1) + ", Col " + strconv.Itoa(v.Col+1)
		px := area.X + area.W - utf8.RuneCountInString(position) - 1
		if px > area.X {
			buf.SetString(px, area.Y+area.H-1, position, et.EditorMuted)
		}
	}
	v.drawContextMenu(buf, area, &et)
}

func decimalWidth(n int) int {
	if n < 1 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	return w
}

func byteOffsetForRune(s string, runeCol int) int {
	if runeCol <= 0 {
		return 0
	}
	count := 0
	for i := range s {
		if count == runeCol {
			return i
		}
		count++
	}
	return len(s)
}

func runeAtString(s string, runeCol int) rune {
	if runeCol < 0 {
		return 0
	}
	count := 0
	for _, r := range s {
		if count == runeCol {
			return r
		}
		count++
	}
	return 0
}

func runeSubstring(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	bs := byteOffsetForRune(s, start)
	be := byteOffsetForRune(s, end)
	return s[bs:be]
}

func displayColumnAtRune(s string, runeCol int) int {
	if runeCol <= 0 {
		return 0
	}
	col, n := 0, 0
	for _, r := range s {
		if n >= runeCol {
			break
		}
		w := buffer.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		col += w
		n++
	}
	return col
}

func runeAtDisplayColumn(s string, cellCol int) int {
	if cellCol <= 0 {
		return 0
	}
	col, n := 0, 0
	for _, r := range s {
		w := buffer.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		if cellCol < col+w {
			return n
		}
		col += w
		n++
	}
	return n
}

func visibleRuneRange(s string, hscroll, width int) (start, end int) {
	if hscroll < 0 {
		hscroll = 0
	}
	if width <= 0 {
		return 0, 0
	}
	// Source files are overwhelmingly ASCII. In that common case, rune index
	// and terminal column are the same, so avoid Unicode decoding and width
	// classification entirely. Tabs/control bytes retain the historical
	// one-cell fallback used by this function.
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		start = hscroll
		if start > len(s) {
			start = len(s)
		}
		end = start + width
		if end > len(s) {
			end = len(s)
		}
		return start, end
	}
	col, i := 0, 0
	startSet := false
	for _, r := range s {
		w := buffer.RuneWidth(r)
		if w <= 0 {
			w = 1
		}
		if !startSet && col+w > hscroll {
			start = i
			startSet = true
		}
		if startSet && col >= hscroll+width {
			break
		}
		col += w
		i++
	}
	if !startSet {
		return i, i
	}
	return start, i
}

func (v *TextEditor) ensureScroll(height, width int) {
	if height < 1 {
		return
	}
	if !v.manualScroll {
		if v.Cursor < v.Scroll {
			v.Scroll = v.Cursor
		}
		if v.Cursor >= v.Scroll+height {
			v.Scroll = v.Cursor - height + 1
		}
	}
	if width < 1 {
		return
	}
	if v.Col < v.HScroll {
		v.HScroll = v.Col
	}
	if v.Col >= v.HScroll+width {
		v.HScroll = v.Col - width + 1
	}
	if v.Scroll < 0 {
		v.Scroll = 0
	}
	if v.HScroll < 0 {
		v.HScroll = 0
	}
}
func (v *TextEditor) drawLine(buf *buffer.Buffer, x, y, w, line int, base style.Style, theme *style.EditorTheme) {
	if w <= 0 || line >= len(v.Lines) {
		return
	}
	text := v.Lines[line]
	start, end := visibleRuneRange(text, v.HScroll, w)
	shown := runeSubstring(text, start, end)
	buf.SetString(x, y, shown, base)
	var lineTokens []SyntaxToken
	if v.tokenCache != nil {
		lineTokens = v.tokenCache[line]
	} else {
		startToken, endToken := v.tokenRangeForLine(line)
		lineTokens = v.Tokens[startToken:endToken]
	}
	for _, ts := range lineTokens {
		if ts.Col < end && ts.Col+utf8.RuneCountInString(ts.Text) > start {
			overlapStart := ts.Col
			if overlapStart < start {
				overlapStart = start
			}
			overlapEnd := ts.Col + utf8.RuneCountInString(ts.Text)
			if overlapEnd > end {
				overlapEnd = end
			}
			if overlapEnd > overlapStart {
				st := base
				switch ts.Kind {
				case SyntaxKeyword:
					st = theme.Keyword
				case SyntaxString:
					st = theme.String
				case SyntaxComment:
					st = theme.Comment
				case SyntaxNumber:
					st = theme.Number
				case SyntaxType:
					st = theme.Type
				case SyntaxFunction:
					st = theme.Function
				case SyntaxConstant:
					st = theme.Constant
				case SyntaxBoolean:
					st = theme.Boolean
				case SyntaxOperator:
					st = theme.Operator
				case SyntaxPunctuation:
					st = theme.Punctuation
				}
				buf.SetString(x+displayColumnAtRune(text, overlapStart)-v.HScroll, y, runeSubstring(text, overlapStart, overlapEnd), st)
			}
		}
	}
	if sl, sc, el, ec, ok := v.selectedRange(); ok && line >= sl && line <= el {
		selectionStart := start
		selectionEnd := end
		if line == sl && sc > selectionStart {
			selectionStart = sc
		}
		if line == el && ec < selectionEnd {
			selectionEnd = ec
		}
		if selectionEnd > selectionStart {
			buf.SetString(x+displayColumnAtRune(text, selectionStart)-v.HScroll, y, runeSubstring(text, selectionStart, selectionEnd), base.WithBg(theme.Selection.Bg).WithFg(theme.Selection.Fg))
		}
	}
}

func (v *TextEditor) isHidden(line int) bool {
	return line >= 0 && line < len(v.hidden) && v.hidden[line]
}

func (v *TextEditor) findNext() bool {
	if v.Search == "" || len(v.Lines) == 0 {
		return false
	}
	start := v.searchHit
	if start < 0 {
		start = 0
	}
	for n := 0; n < len(v.Lines); n++ {
		i := (start + n) % len(v.Lines)
		if p := indexFold(v.Lines[i], v.Search); p >= 0 {
			v.Cursor = i
			v.Col = runeColumn(v.Lines[i], p)
			v.ensureCursorVisible()
			v.searchHit = i + 1
			v.status = "Found"
			return true
		}
	}
	v.status = "Not found"
	return false
}

// indexFold returns a byte offset for a case-insensitive substring match.
// ASCII search stays on a tight byte loop with no temporary strings. Unicode
// uses simple Unicode case folding at rune boundaries, preserving correctness
// without allocating lower-cased copies of every document line.
func indexFold(s, needle string) int {
	p, _ := indexFoldN(s, needle)
	return p
}

func indexFoldN(s, needle string) (pos, matchLen int) {
	if needle == "" {
		return 0, 0
	}
	ascii := true
	for i := 0; i < len(needle); i++ {
		if needle[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		if len(needle) > len(s) {
			return -1, 0
		}
		for i := 0; i <= len(s)-len(needle); i++ {
			matched := true
			for j := 0; j < len(needle); j++ {
				a, b := s[i+j], needle[j]
				if a >= 'A' && a <= 'Z' {
					a += 'a' - 'A'
				}
				if b >= 'A' && b <= 'Z' {
					b += 'a' - 'A'
				}
				if a != b {
					matched = false
					break
				}
			}
			if matched {
				return i, len(needle)
			}
		}
		return -1, 0
	}
	for i := 0; i < len(s); {
		if n, ok := matchFoldAt(s[i:], needle); ok {
			return i, n
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		if n == 0 {
			break
		}
		i += n
	}
	return -1, 0
}

func matchFoldAt(s, needle string) (int, bool) {
	if needle == "" {
		return 0, true
	}
	si, ni := 0, 0
	for ni < len(needle) {
		sr, sn := utf8.DecodeRuneInString(s[si:])
		nr, nn := utf8.DecodeRuneInString(needle[ni:])
		if sn == 0 || nn == 0 || !runeEqualFold(sr, nr) {
			return 0, false
		}
		si += sn
		ni += nn
	}
	return si, true
}

func runeEqualFold(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

func (v *TextEditor) selectedText() string {
	sl, sc, el, ec, ok := v.selectedRange()
	if !ok {
		return ""
	}
	if sl == el {
		return runeSubstring(v.Lines[sl], sc, ec)
	}
	parts := make([]string, 0, el-sl+1)
	parts = append(parts, runeSubstring(v.Lines[sl], sc, len([]rune(v.Lines[sl]))))
	for i := sl + 1; i < el; i++ {
		parts = append(parts, v.Lines[i])
	}
	parts = append(parts, runeSubstring(v.Lines[el], 0, ec))
	return strings.Join(parts, "\n")
}

func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func (v *TextEditor) moveWord(delta int, extend bool) {
	line, col := v.Cursor, v.Col
	if delta > 0 {
		r := []rune(v.Lines[line])
		for col < len(r) && !isWordRune(r[col]) {
			col++
		}
		for col < len(r) && isWordRune(r[col]) {
			col++
		}
		if col == len(r) && line < len(v.Lines)-1 {
			line++
			col = 0
		}
	} else {
		if col == 0 && line > 0 {
			line--
			col = len([]rune(v.Lines[line]))
		}
		r := []rune(v.Lines[line])
		for col > 0 && !isWordRune(r[col-1]) {
			col--
		}
		for col > 0 && isWordRune(r[col-1]) {
			col--
		}
	}
	v.setCursor(line, col, extend)
}

func (v *TextEditor) deleteWordBackward() {
	v.typingActive = false
	if v.ReadOnly {
		return
	}
	if v.hasAnchor {
		v.recordEdit()
		v.deleteSelection()
		v.markChanged()
		return
	}
	if v.Col == 0 {
		v.backspace()
		return
	}
	v.recordEdit()
	r := []rune(v.Lines[v.Cursor])
	end := v.Col
	start := end
	for start > 0 && !isWordRune(r[start-1]) {
		start--
	}
	for start > 0 && isWordRune(r[start-1]) {
		start--
	}
	v.Lines[v.Cursor] = string(append(r[:start], r[end:]...))
	v.Col = start
	v.markChanged()
}

func (v *TextEditor) deleteLine() {
	v.typingActive = false
	if v.ReadOnly || len(v.Lines) == 0 {
		return
	}
	v.recordEdit()
	if len(v.Lines) == 1 {
		v.Lines[0] = ""
		v.Col = 0
	} else {
		v.Lines = append(v.Lines[:v.Cursor], v.Lines[v.Cursor+1:]...)
		if v.Cursor >= len(v.Lines) {
			v.Cursor = len(v.Lines) - 1
		}
		v.Col = 0
	}
	v.hasAnchor = false
	v.markChanged()
}

func (v *TextEditor) duplicateLine() {
	v.typingActive = false
	if v.ReadOnly {
		return
	}
	v.recordEdit()
	if sl, _, el, _, ok := v.selectedRange(); ok {
		text := append([]string(nil), v.Lines[sl:el+1]...)
		at := el + 1
		v.Lines = append(v.Lines[:at], append(text, v.Lines[at:]...)...)
		v.Cursor += el - sl + 1
		v.hasAnchor = false
	} else {
		line := v.Lines[v.Cursor]
		v.Lines = append(v.Lines[:v.Cursor+1], append([]string{line}, v.Lines[v.Cursor+1:]...)...)
		v.Cursor++
	}
	v.markChanged()
}

func (v *TextEditor) commentPrefix() string {
	switch v.Language {
	case "py", "python", "rb", "sh", "bash", "zsh", "fish", "yaml", "yml", "toml", "ini", "conf", "env":
		return "#"
	case "sql":
		return "--"
	case "html", "htm", "xml":
		return "<!--"
	case "css":
		return "/*"
	default:
		return "//"
	}
}

func (v *TextEditor) toggleComment() {
	v.typingActive = false
	if v.ReadOnly {
		return
	}
	start, _, end, _, ok := v.selectedRange()
	if !ok {
		start, end = v.Cursor, v.Cursor
	}
	prefix := v.commentPrefix()
	v.recordEdit()
	allCommented := true
	for i := start; i <= end; i++ {
		trim := strings.TrimSpace(v.Lines[i])
		if trim != "" && !strings.HasPrefix(trim, prefix) {
			allCommented = false
			break
		}
	}
	for i := start; i <= end; i++ {
		line := v.Lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if allCommented {
			if idx := strings.Index(line[indent:], prefix); idx >= 0 {
				idx += indent
				line = line[:idx] + line[idx+len(prefix):]
				if idx < len(line) && line[idx] == ' ' {
					line = line[:idx] + line[idx+1:]
				}
			}
		} else {
			line = line[:indent] + prefix + " " + line[indent:]
		}
		v.Lines[i] = line
	}
	v.markChanged()
}

func (v *TextEditor) gotoLine(line int) bool {
	if line < 1 || line > len(v.Lines) {
		v.status = "Line out of range"
		return false
	}
	v.Cursor = line - 1
	v.Col = 0
	v.hasAnchor = false
	v.ensureCursorVisible()
	v.status = "Line " + strconv.Itoa(line)
	return true
}

func (v *TextEditor) HandleKey(k input.Key) bool {
	v.manualScroll = false
	// Files may be empty (or a caller may construct a viewer before loading).
	// Keep cursor/edit operations safe even when Lines has not been initialized.
	v.clamp()
	if v.Searching || v.GotoLine {
		if k.Type == input.KeyEsc {
			v.Searching = false
			v.GotoLine = false
			v.status = ""
			return true
		}
		if k.Type == input.KeyBackspace {
			r := []rune(v.Search)
			if len(r) > 0 {
				v.Search = string(r[:len(r)-1])
			}
			return true
		}
		if k.Type == input.KeyEnter {
			if v.GotoLine {
				line, err := strconv.Atoi(v.Search)
				if err == nil {
					v.gotoLine(line)
				} else {
					v.status = "Invalid line"
				}
				v.GotoLine = false
			} else {
				v.Searching = false
				v.searchHit = v.Cursor
				v.findNext()
			}
			return true
		}
		if k.Type == input.KeyRune || k.Type == input.KeySpace {
			v.Search += string(k.Rune)
			return true
		}
		return true
	}
	if k.Type == input.KeyTab || k.Type == input.KeyShiftTab {
		if v.ReadOnly {
			return true
		}
		if k.Type == input.KeyTab {
			for i := 0; i < editorTabWidth; i++ {
				v.insertRune(' ')
			}
			return true
		}
		v.outdentLine()
		return true
	}

	if k.Type == input.KeyCtrlC {
		// Ctrl+C belongs to the editor while it is focused. With no selection
		// it is a harmless no-op rather than falling through to App's global
		// Ctrl+C-to-quit behavior.
		v.copySelection(false)
		return true
	}
	ctrl := k.Mods&input.ModCtrl != 0
	shift := k.Mods&input.ModShift != 0
	if ctrl && k.Type == input.KeyLeft {
		v.moveWord(-1, shift)
		return true
	}
	if ctrl && k.Type == input.KeyRight {
		v.moveWord(1, shift)
		return true
	}
	if ctrl && k.Type == input.KeyBackspace {
		v.deleteWordBackward()
		return true
	}
	if ctrl && k.Type == input.KeyHome {
		v.setCursor(0, 0, shift)
		return true
	}
	if ctrl && k.Type == input.KeyEnd {
		v.setCursor(len(v.Lines)-1, len([]rune(v.Lines[len(v.Lines)-1])), shift)
		return true
	}
	if k.Mods&input.ModAlt != 0 && (k.Type == input.KeyUp || k.Type == input.KeyDown) && !v.ReadOnly {
		d := -1
		if k.Type == input.KeyDown {
			d = 1
		}
		n := v.Cursor + d
		if n >= 0 && n < len(v.Lines) {
			v.recordEditRange(minInt(v.Cursor, n), maxInt(v.Cursor, n)+1)
			v.Lines[v.Cursor], v.Lines[n] = v.Lines[n], v.Lines[v.Cursor]
			v.Cursor = n
			v.markChanged()
		}
		return true
	}
	if ctrl {
		switch k.Rune {
		case 's':
			return v.Save()
		case 'z':
			return v.Undo()
		case 'y':
			return v.Redo()
		case 'f':
			v.Searching = true
			v.GotoLine = false
			v.Search = ""
			v.searchHit = v.Cursor
			return true
		case 'g':
			v.Searching = true
			v.GotoLine = true
			v.Search = ""
			return true
		case 'a':
			v.anchorLine, v.anchorCol = 0, 0
			v.Cursor = len(v.Lines) - 1
			v.Col = len([]rune(v.Lines[v.Cursor]))
			v.hasAnchor = true
			return true
		case 'c':
			if !v.hasAnchor {
				return true
			}
			return v.copySelection(false)
		case 'x':
			return v.copySelection(true)
		case 'v':
			v.paste()
			return true
		case 'd':
			v.duplicateLine()
			return true
		case '/':
			v.toggleComment()
			return true
		case 'k':
			if shift {
				v.deleteLine()
				return true
			}
		case 'b':
			return false
		}
	}
	if ctrl && k.Type == input.KeyLeft {
		v.moveWord(-1, shift)
		return true
	}
	if ctrl && k.Type == input.KeyRight {
		v.moveWord(1, shift)
		return true
	}
	if ctrl && k.Type == input.KeyBackspace {
		v.deleteWordBackward()
		return true
	}
	if ctrl && k.Type == input.KeyHome {
		v.setCursor(0, 0, shift)
		return true
	}
	if ctrl && k.Type == input.KeyEnd {
		v.setCursor(len(v.Lines)-1, len([]rune(v.Lines[len(v.Lines)-1])), shift)
		return true
	}
	if k.Mods&input.ModAlt != 0 && (k.Type == input.KeyUp || k.Type == input.KeyDown) && !v.ReadOnly {
		d := -1
		if k.Type == input.KeyDown {
			d = 1
		}
		n := v.Cursor + d
		if n >= 0 && n < len(v.Lines) {
			v.recordEditRange(minInt(v.Cursor, n), maxInt(v.Cursor, n)+1)
			v.Lines[v.Cursor], v.Lines[n] = v.Lines[n], v.Lines[v.Cursor]
			v.Cursor = n
			v.markChanged()
		}
		return true
	}

	if k.Type == input.KeyBackspace {
		v.backspace()
		return true
	}
	if k.Type == input.KeyDelete {
		v.deleteForward()
		return true
	}
	if k.Type == input.KeyEnter {
		v.insertNewline()
		return true
	}
	if k.Type == input.KeyUp {
		v.moveLine(-1, shift)
		return true
	}
	if k.Type == input.KeyDown {
		v.moveLine(1, shift)
		return true
	}
	if k.Type == input.KeyLeft {
		v.moveCol(-1, shift)
		return true
	}
	if k.Type == input.KeyRight {
		v.moveCol(1, shift)
		return true
	}
	if k.Type == input.KeyHome {
		v.home(shift)
		return true
	}
	if k.Type == input.KeyEnd {
		v.end(shift)
		return true
	}
	if k.Type == input.KeyPageUp {
		v.page(-10, shift)
		return true
	}
	if k.Type == input.KeyPageDown {
		v.page(10, shift)
		return true
	}
	if k.Type == input.KeyRune || k.Type == input.KeySpace {
		if !ctrl {
			v.insertRune(k.Rune)
			return true
		}
	}
	if k.Type == input.KeyEnter {
		return true
	}
	if k.Rune == 'z' {
		return v.ToggleFold()
	}
	if k.Rune == 'Z' {
		for s := range v.Collapsed {
			v.Collapsed[s] = false
		}
		v.rebuildHiddenIndex()
		return true
	}
	return false
}
func (v *TextEditor) ToggleFold() bool {
	if _, ok := v.Folds[v.Cursor]; !ok {
		return false
	}
	v.Collapsed[v.Cursor] = !v.Collapsed[v.Cursor]
	v.rebuildHiddenIndex()
	return true
}
func (v *TextEditor) contextMenuRect(area geometry.Rect) geometry.Rect {
	// The menu is a small editor-native surface, not a terminal Reverse block.
	// A border plus theme-aware fills keeps it polished on dark and light themes.
	w, h := 24, 6
	if w > area.W-2 {
		w = area.W - 2
	}
	if h > area.H-2 {
		h = area.H - 2
	}
	if w < 12 {
		w = area.W
	}
	if h < 3 {
		h = area.H
	}
	x, y := v.contextMenuX, v.contextMenuY
	if x+w > area.X+area.W {
		x = area.X + area.W - w
	}
	if y+h > area.Y+area.H {
		y = area.Y + area.H - h
	}
	if x < area.X {
		x = area.X
	}
	if y < area.Y {
		y = area.Y
	}
	return geometry.Rect{X: x, Y: y, W: w, H: h}
}

func (v *TextEditor) handleContextMenuMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if !v.contextMenu {
		return false
	}
	if ev.Action != input.MousePress {
		return true
	}
	menu := v.contextMenuRect(area)
	if !menu.Contains(ev.X, ev.Y) {
		v.contextMenu = false
		return true
	}
	row := ev.Y - menu.Y - 1
	if row < 0 || row >= 4 || ev.X <= menu.X || ev.X >= menu.X+menu.W-1 {
		v.contextMenu = false
		return true
	}
	v.contextMenuHover = row
	v.contextMenu = false
	switch row {
	case 0:
		v.paste()
	case 1:
		v.copySelection(false)
	case 2:
		v.copySelection(true)
	case 3:
		v.anchorLine, v.anchorCol = 0, 0
		v.Cursor = len(v.Lines) - 1
		v.Col = len([]rune(v.Lines[v.Cursor]))
		v.hasAnchor = true
	}
	return true
}

func (v *TextEditor) drawContextMenu(buf *buffer.Buffer, area geometry.Rect, theme *style.EditorTheme) {
	if !v.contextMenu || area.W <= 0 || area.H <= 0 {
		return
	}
	menu := v.contextMenuRect(area)
	if menu.W < 12 || menu.H < 3 {
		return
	}
	buffer.DrawBorder(buf, menu.X, menu.Y, menu.W, menu.H, "", theme.ContextMenuBorder, theme.TitleSyntax, theme.ContextMenu, true)
	labels := [...]string{"Paste", "Copy", "Cut", "Select All"}
	shortcuts := [...]string{"Ctrl+V", "Ctrl+C", "Ctrl+X", "Ctrl+A"}
	for i, label := range labels {
		y := menu.Y + 1 + i
		st := theme.ContextMenu
		if i == v.contextMenuHover {
			st = theme.ContextMenuSelected
		}
		buf.FillRect(menu.X+1, y, menu.W-2, 1, ' ', st)
		buf.SetString(menu.X+3, y, label, st.WithAttr(style.Bold))
		hint := theme.ContextMenuHint
		hx := menu.X + menu.W - utf8.RuneCountInString(shortcuts[i]) - 3
		if hx > menu.X+8 {
			buf.SetString(hx, y, shortcuts[i], hint)
		}
	}
}

func (v *TextEditor) HandleMouse(ev input.MouseEvent, area geometry.Rect) bool {
	if v.contextMenu && v.handleContextMenuMouse(ev, area) {
		return true
	}
	if !area.Contains(ev.X, ev.Y) && ev.Action != input.MouseDrag {
		return false
	}
	// Mouse-wheel scrolling is handled directly by the editor viewport. Keep the
	// cursor where it is, just like a conventional code editor, and clamp after
	// every wheel event so repeated scrolling can never produce invalid state.
	if ev.Action == input.MouseWheelUp || ev.Action == input.MouseWheelDown {
		v.manualScroll = true
		step := 3
		if ev.Action == input.MouseWheelUp {
			v.Scroll -= step
		} else {
			v.Scroll += step
		}
		// The viewport has a header and status row, so only area.H-2 rows are
		// available for source lines. Do not let Draw/ensureScroll immediately
		// snap a mouse-scrolled viewport back to the cursor line.
		visible := area.H - 1
		if v.ShowStatusBar {
			visible = area.H - 2
		}
		visible = maxInt(visible, 1)
		maxScroll := len(v.Lines) - visible
		if maxScroll < 0 {
			maxScroll = 0
		}
		if v.Scroll < 0 {
			v.Scroll = 0
		}
		if v.Scroll > maxScroll {
			v.Scroll = maxScroll
		}
		return true
	}
	row := ev.Y - area.Y - 1
	if row < 0 {
		return ev.Action == input.MousePress
	}
	count := 0
	lineAt := -1
	for i := v.Scroll; i < len(v.Lines); i++ {
		if v.isHidden(i) {
			continue
		}
		if count == row {
			lineAt = i
			break
		}
		count++
	}
	if lineAt < 0 {
		return true
	}
	numWidth := decimalWidth(len(v.Lines))
	codeX := area.X + numWidth + 4
	if ev.X < codeX-1 {
		if ev.Action == input.MousePress {
			if _, ok := v.Folds[lineAt]; ok {
				v.Collapsed[lineAt] = !v.Collapsed[lineAt]
			}
		}
		return true
	}
	cellCol := v.HScroll + ev.X - codeX
	if cellCol < 0 {
		cellCol = 0
	}
	col := runeAtDisplayColumn(v.Lines[lineAt], cellCol)
	if ev.Button == input.MouseRight && ev.Action == input.MousePress {
		// Put the caret at the context-click location unless the user is
		// right-clicking an existing selection. This makes Paste behave like a
		// normal desktop editor instead of inserting at an unrelated old caret.
		if sl, sc, el, ec, selected := v.selectedRange(); !selected || lineAt < sl || lineAt > el || (lineAt == sl && col < sc) || (lineAt == el && col > ec) {
			v.Cursor, v.Col = lineAt, col
			v.hasAnchor = false
		}
		v.contextMenu = true
		v.contextMenuHover = 0
		v.contextMenuX, v.contextMenuY = ev.X, ev.Y
		return true
	}
	switch ev.Action {
	case input.MousePress:
		v.manualScroll = false
		v.Cursor, v.Col = lineAt, col
		v.anchorLine, v.anchorCol = lineAt, col
		v.hasAnchor = false
		v.dragging = true
	case input.MouseDrag:
		if v.dragging {
			v.Cursor, v.Col = lineAt, col
			v.hasAnchor = v.anchorLine != v.Cursor || v.anchorCol != v.Col
		}
	case input.MouseRelease:
		if v.dragging {
			v.Cursor, v.Col = lineAt, col
			v.hasAnchor = v.anchorLine != v.Cursor || v.anchorCol != v.Col
		}
		v.dragging = false
	}
	return true
}

// ReplaceAll replaces every case-insensitive occurrence in the document and
// returns the number of replacements. It is an explicit bulk-edit operation,
// so allocations are acceptable here; interactive steady-state rendering is
// unaffected. A single document history entry makes the whole operation undoable.
func (v *TextEditor) ReplaceAll(search, replacement string) int {
	if v.ReadOnly || search == "" || len(v.Lines) == 0 {
		return 0
	}
	v.Document.BeginEdit()
	v.undo = append(v.undo, v.snapshot())
	if len(v.undo) > 100 {
		v.undo = v.undo[1:]
	}
	v.redo = nil
	count := 0
	for i, line := range v.Lines {
		pos := 0
		var b strings.Builder
		matchedLine := false
		for pos < len(line) {
			p, matchLen := indexFoldN(line[pos:], search)
			if p < 0 {
				break
			}
			p += pos
			if !matchedLine {
				b.Grow(len(line) + len(replacement))
				matchedLine = true
			}
			b.WriteString(line[pos:p])
			b.WriteString(replacement)
			pos = p + matchLen
			count++
		}
		if matchedLine {
			b.WriteString(line[pos:])
			v.Lines[i] = b.String()
		}
	}
	if count == 0 {
		v.Document.pending = nil
		return 0
	}
	v.markChanged()
	return count
}

// SetText replaces the document contents without touching the filesystem.
func (v *TextEditor) SetText(text string) {
	display := expandTabs(strings.ReplaceAll(text, "\r\n", "\n"), editorTabWidth)
	v.suppressDocSync = true
	v.Document.SetText(display)
	v.suppressDocSync = false
	v.Document.MarkSaved()
	v.Modified = false
	v.Tokens = v.Tokens[:0]
	v.Folds = make(map[int]FoldRange)
	v.Collapsed = make(map[int]bool)
	v.Cursor, v.Col, v.Scroll, v.HScroll = 0, 0, 0, 0
	v.Searching, v.hasAnchor = false, false
	v.undo, v.redo = nil, nil
	v.typingActive = false
	v.status = ""
	v.rehighlight()
	v.observedRevision = v.Document.Revision
}

// Text returns the complete document contents. Persistence is deliberately
// owned by the application/editor layer, not this reusable widget.
func (v *TextEditor) Text() string { return v.Document.Text() }

// Save invokes the host-provided save callback. This keeps Ctrl-S useful while
// ensuring the reusable widget has no filesystem dependency.
func (v *TextEditor) Save() bool {
	if v.ReadOnly || v.OnSave == nil {
		return false
	}
	ok := v.OnSave()
	if ok {
		v.Document.MarkSaved()
		v.Modified = false
		v.status = "Saved"
	} else {
		v.status = "Save failed"
	}
	return ok
}

// SetViewport records the available editor viewport used for caret scrolling.
func (v *TextEditor) SetViewport(height, width int) { v.viewportH, v.viewportW = height, width }

// ViewportHeight returns the last configured viewport height.
func (v *TextEditor) ViewportHeight() int { return v.viewportH }

// ViewportWidth returns the last configured viewport width.
func (v *TextEditor) ViewportWidth() int { return v.viewportW }

// EnsureScroll applies caret/viewport scrolling rules without rendering.
func (v *TextEditor) EnsureScroll(height, width int) { v.ensureScroll(height, width) }

// SetManualScroll controls whether the viewport is temporarily decoupled from the caret.
func (v *TextEditor) SetManualScroll(manual bool) { v.manualScroll = manual }

// ManualScroll reports whether wheel scrolling has decoupled the viewport.
func (v *TextEditor) ManualScroll() bool { return v.manualScroll }

// MoveLine moves the caret by a line delta, optionally extending the selection.
func (v *TextEditor) MoveLine(delta int, extend bool) { v.moveLine(delta, extend) }

// HasSelection reports whether the editor currently has an active selection.
func (v *TextEditor) HasSelection() bool { return v.hasAnchor }

// SetStatus sets the transient status text shown by the editor.
func (v *TextEditor) SetStatus(status string) { v.status = status }

// Status returns the current transient status text.
func (v *TextEditor) Status() string { return v.status }

// SetSearchHit records the line from which the next search starts.
func (v *TextEditor) SetSearchHit(line int) { v.searchHit = line }

// LineTokenCount reports the number of source lines available to the editor.
// It is retained for compatibility with the original indexed-token API.
func (v *TextEditor) LineTokenCount() int { return len(v.Lines) }

// SetSelectionAnchor starts a selection from the supplied rune position.
func (v *TextEditor) SetSelectionAnchor(line, col int) {
	v.anchorLine, v.anchorCol = line, col
	v.hasAnchor = true
}

// SelectedRange returns the normalized selection bounds.
func (v *TextEditor) SelectedRange() (sl, sc, el, ec int, ok bool) {
	return v.selectedRange()
}

// CopySelection copies the current selection and optionally removes it.
func (v *TextEditor) CopySelection(cut bool) bool { return v.copySelection(cut) }

// Paste inserts clipboard contents at the current caret.
func (v *TextEditor) Paste() { v.paste() }

// DuplicateLine duplicates the current line or selected range.
func (v *TextEditor) DuplicateLine() { v.duplicateLine() }

// DeleteLine deletes the current line or selected range.
func (v *TextEditor) DeleteLine() { v.deleteLine() }

// ToggleComment toggles line comments for the current selection.
func (v *TextEditor) ToggleComment() { v.toggleComment() }

// GotoLineNumber moves the caret to a 1-based line number.
func (v *TextEditor) GotoLineNumber(line int) bool { return v.gotoLine(line) }

func (v *TextEditor) Focus(f bool)    { v.FocusMixin.Focus(f) }
func (v *TextEditor) IsFocused() bool { return v.FocusMixin.IsFocused() }
func (v *TextEditor) VisibleLines(areaH int) []int {
	out := make([]int, 0, areaH)
	for i := v.Scroll; i < len(v.Lines) && len(out) < areaH; i++ {
		if !v.isHidden(i) {
			out = append(out, i)
		}
	}
	return out
}

func expandTabs(s string, tabWidth int) string {
	if tabWidth <= 0 {
		tabWidth = editorTabWidth
	}
	if !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	// This is an upper bound for the common tab expansion case and avoids
	// repeated builder growth on large source files.
	b.Grow(len(s) + strings.Count(s, "\t")*(tabWidth-1))
	spaces := strings.Repeat(" ", tabWidth)
	for _, r := range s {
		if r == '\t' {
			b.WriteString(spaces)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
