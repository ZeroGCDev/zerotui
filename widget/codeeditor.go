package widget

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
)

// CodeEditor is the syntax-aware specialization of TextEditor. It adds
// pluggable syntax highlighting and folding while leaving editing, document
// ownership, cursor/selection, scrolling, undo/redo and rendering in the
// reusable TextEditor.
type CodeEditor struct {
	*TextEditor
	Highlighter  SyntaxHighlighter
	FoldProvider FoldProvider
	// Large documents use a viewport token cache. This keeps cold-open memory
	// and steady-state rendering proportional to what is actually on screen.
	largeDocument bool
}

const (
	largeEditorLineThreshold = 100_000
	largeEditorByteThreshold = 4 << 20
	largeEditorTokenContext  = 256
)

// Load initializes the code document from source bytes. It performs no filesystem I/O.
// Persistence remains the responsibility of the host application.
func (c *CodeEditor) Load(path string, data []byte) {
	c.Document.SetName(path)
	c.Document.SetLanguage(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."))
	// Document notifications are synchronous, so select the large-file path
	// before SetText to avoid an accidental full-document highlight on load.
	c.largeDocument = len(data) >= largeEditorByteThreshold || 1+bytes.Count(data, []byte{'\n'}) >= largeEditorLineThreshold
	c.SetText(string(data))

	c.Document.MarkSaved()
	c.Modified = false
}

func NewCodeEditor() *CodeEditor {
	c := &CodeEditor{
		TextEditor:   NewTextEditor(),
		Highlighter:  DefaultSyntaxHighlighter{},
		FoldProvider: DefaultFoldProvider{},
	}
	c.TextEditor.Rehighlight = c.rehighlightText
	c.TextEditor.OnDocumentChange = c.onDocumentChange
	return c
}

func (c *CodeEditor) onDocumentChange(v *TextEditor, change DocumentChange) {
	// Block comments/raw strings can carry lexical state across lines, but an
	// ordinary Enter on a blank/code line does not change that state. Avoid a
	// whole-document lexer/fold scan for the common typing path; only fall back
	// to the conservative full scan when the changed neighbourhood contains a
	// multiline delimiter. This is especially important when Enter is held down
	// in a large source file.
	if !c.largeDocument && (strings.EqualFold(v.Language, "go") || languageHasMultilineSyntax(v.Language)) && c.changeNeedsFullLex(v, change) {
		c.rehighlightText(v)
		return
	}
	if c.largeDocument {
		c.invalidateViewportTokens(v, change)
	} else if c.Highlighter == nil {
		v.Tokens = v.Tokens[:0]
	} else if h, ok := c.Highlighter.(RangeSyntaxHighlighter); ok && change.Revision > 0 {
		c.rehighlightTokenRange(v, h, change)
	} else {
		v.Tokens = c.Highlighter.Highlight(v.Document.Name, v.Language, v.Lines)
	}
	if c.foldsNeedRebuild(v, change) {
		c.rebuildFolds(v)
		v.rebuildHiddenIndex()
	} else if change.EndLine != change.OldEndLine {
		c.shiftFoldsForInsertion(v, change)
		v.rebuildHiddenIndex()
	}
}

func (c *CodeEditor) changeNeedsFullLex(v *TextEditor, change DocumentChange) bool {
	// A pure line insertion (the common Enter path) can be highlighted locally
	// when the newly produced lines contain no multiline delimiter. Deletions
	// and same-size replacements are conservative because the removed text is no
	// longer available here; it may have contained the delimiter that carried
	// lexical state into the following lines.
	delta := change.EndLine - change.OldEndLine
	if delta > 0 {
		for i := change.StartLine; i < change.EndLine && i < len(v.Lines); i++ {
			line := v.Lines[i]
			if strings.Contains(line, "/*") || strings.Contains(line, "*/") || strings.ContainsRune(line, '`') {
				return true
			}
		}
		return false
	}

	start := change.StartLine - 1
	if start < 0 {
		start = 0
	}
	end := change.EndLine + 1
	if end > len(v.Lines) {
		end = len(v.Lines)
	}
	for i := start; i < end; i++ {
		line := v.Lines[i]
		if strings.Contains(line, "/*") || strings.Contains(line, "*/") || strings.ContainsRune(line, '`') {
			return true
		}
	}
	return true
}

func languageHasMultilineSyntax(language string) bool {
	p, ok := languageProfileFor(language)
	return ok && p.blockOpen != "" && p.blockClose != ""
}

func (c *CodeEditor) rehighlightTokenRange(v *TextEditor, h RangeSyntaxHighlighter, change DocumentChange) {
	start := change.StartLine
	oldEnd := change.OldEndLine
	newEnd := change.EndLine
	if start < 0 {
		start = 0
	}
	if oldEnd < start {
		oldEnd = start
	}
	if newEnd < start {
		newEnd = start
	}
	if oldEnd > len(v.Lines) {
		oldEnd = len(v.Lines)
	}
	if newEnd > len(v.Lines) {
		newEnd = len(v.Lines)
	}
	delta := newEnd - oldEnd
	changed := h.HighlightRange(v.Document.Name, v.Language, v.Lines, start, newEnd)
	c.replaceTokenRange(start, oldEnd, newEnd, delta, changed)
}

func (c *CodeEditor) replaceTokenRange(start, oldEnd, newEnd, delta int, changed []SyntaxToken) {
	old := c.Tokens
	lo := sort.Search(len(old), func(i int) bool { return old[i].Line >= start })
	hi := sort.Search(len(old), func(i int) bool { return old[i].Line >= oldEnd })
	shifted := 0
	for i := hi; i < len(old); i++ {
		if old[i].Line+delta >= newEnd {
			shifted++
		}
	}
	newLen := lo + len(changed) + shifted
	if newLen > cap(old) {
		merged := make([]SyntaxToken, 0, newLen)
		merged = append(merged, old[:lo]...)
		merged = append(merged, changed...)
		for i := hi; i < len(old); i++ {
			ts := old[i]
			if ts.Line+delta >= newEnd {
				ts.Line += delta
				merged = append(merged, ts)
			}
		}
		c.Tokens = merged
		return
	}

	// Compact the surviving tail backwards. Backward traversal prevents an
	// earlier write from overwriting a source token that has not been visited.
	tail := old[:newLen]
	write := newLen
	for i := len(old) - 1; i >= hi; i-- {
		ts := old[i]
		if ts.Line+delta < newEnd {
			continue
		}
		ts.Line += delta
		write--
		tail[write] = ts
	}
	copy(tail[lo:lo+len(changed)], changed)
	c.Tokens = tail
}

func (c *CodeEditor) shiftFoldsForInsertion(v *TextEditor, change DocumentChange) {
	delta := change.EndLine - change.OldEndLine
	if delta <= 0 || len(v.Folds) == 0 {
		return
	}
	line := change.StartLine
	info := scanGoSplitBraces(v.lastNewlineOldLine, v.lastNewlineCol)
	// Multiple endpoint braces on the split line cannot be mapped precisely from
	// FoldRange alone (it stores line coordinates, not brace columns). Rebuild in
	// that unusual case rather than risk moving one fold to the wrong side.
	if info.ambiguous {
		c.rebuildFolds(v)
		return
	}
	shifted := make(map[int]FoldRange, len(v.Folds))
	for _, f := range v.Folds {
		switch {
		case f.Start > line:
			f.Start += delta
			f.End += delta
		case f.Start == line:
			if info.lastOpen >= v.lastNewlineCol {
				f.Start += delta
			}
			if f.End > line || (f.End == line && info.lastClose >= v.lastNewlineCol) {
				f.End += delta
			}
		case f.End > line:
			f.End += delta
		case f.End == line && info.lastClose >= v.lastNewlineCol:
			f.End += delta
		}
		shifted[f.Start] = f
	}
	v.Folds = shifted
}

type goSplitBraceInfo struct {
	lastOpen  int
	lastClose int
	ambiguous bool
	crosses   bool
}

// scanGoSplitBraces lexes only the line being split. It is intentionally
// allocation-free. Fold ranges are line-based, so the only information needed
// for an Enter is whether an endpoint brace moves to the new line and whether
// the split creates a brand-new multi-line brace pair.
func scanGoSplitBraces(line string, splitCol int) goSplitBraceInfo {
	var info goSplitBraceInfo
	info.lastOpen, info.lastClose = -1, -1
	var stack [64]int
	sp := 0
	var openBefore, openAfter, closeBefore, closeAfter int
	blockComment, rawString := false, false
	for i := 0; i < len(line); {
		if blockComment {
			if j := strings.Index(line[i:], "*/"); j >= 0 {
				i += j + 2
				blockComment = false
			} else {
				break
			}
			continue
		}
		if rawString {
			if j := strings.IndexByte(line[i:], '`'); j >= 0 {
				i += j + 1
				rawString = false
			} else {
				break
			}
			continue
		}
		switch {
		case line[i] == '/' && i+1 < len(line) && line[i+1] == '/':
			i = len(line)
		case line[i] == '/' && i+1 < len(line) && line[i+1] == '*':
			i += 2
			blockComment = true
		case line[i] == '`':
			rawString = true
			i++
		case line[i] == '"' || line[i] == '\'':
			q := line[i]
			i++
			for i < len(line) {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == q {
					i++
					break
				}
				i++
			}
		case line[i] == '{':
			col := runeColumn(line, i)
			info.lastOpen = col
			if col < splitCol {
				openBefore++
			} else {
				openAfter++
			}
			if sp < len(stack) {
				stack[sp] = col
				sp++
			} else {
				// Extremely brace-dense lines are rare; rebuild conservatively.
				info.crosses = true
			}
			i++
		case line[i] == '}':
			col := runeColumn(line, i)
			info.lastClose = col
			if col < splitCol {
				closeBefore++
			} else {
				closeAfter++
			}
			if sp > 0 {
				sp--
				if stack[sp] < splitCol && col >= splitCol {
					info.crosses = true
				}
			}
			i++
		default:
			i++
		}
	}
	info.ambiguous = (openBefore > 0 && openAfter > 0) || (closeBefore > 0 && closeAfter > 0) || info.crosses && (openBefore > 0 || closeAfter > 0)
	return info
}

func (c *CodeEditor) foldsNeedRebuild(v *TextEditor, change DocumentChange) bool {
	if c.FoldProvider == nil {
		return len(v.Folds) != 0
	}
	// Line insertions/deletions normally only shift existing fold coordinates.
	// Rebuild the fold map only when the changed lines actually contain brace
	// structure; plain Enter on a code/blank line can then update folds in O(folds)
	// rather than scanning the whole document. Deletions remain conservative when
	// the removed text is unavailable to this callback.
	if change.OldEndLine != change.EndLine {
		// Enter splits one existing line. Only rebuild when the split crosses a
		// brace pair (creating a new fold) or makes endpoint mapping ambiguous.
		// Otherwise existing folds can be shifted in O(folds).
		if change.EndLine < change.OldEndLine {
			return true
		}
		if v.lastEditWasNewline {
			info := scanGoSplitBraces(v.lastNewlineOldLine, v.lastNewlineCol)
			return info.crosses || info.ambiguous
		}
		return true
	}
	start, end := change.StartLine, change.EndLine
	if start < 0 {
		start = 0
	}
	if end > len(v.Lines) {
		end = len(v.Lines)
	}
	for i := start; i < end; i++ {
		if strings.ContainsAny(v.Lines[i], "{}") {
			return true
		}
	}
	return false
}

func (c *CodeEditor) rebuildFolds(v *TextEditor) {
	v.Folds = make(map[int]FoldRange)
	if c.FoldProvider == nil {
		return
	}
	for _, f := range c.FoldProvider.Folds(v.Document.Name, v.Language, v.Lines) {
		if f.Start >= 0 && f.End >= f.Start && f.Start < len(v.Lines) {
			if f.End >= len(v.Lines) {
				f.End = len(v.Lines) - 1
			}
			v.Folds[f.Start] = f
		}
	}
}

func (c *CodeEditor) enableViewportTokenCache() {
	if c.TextEditor == nil {
		return
	}
	c.TextEditor.tokenCache = make(map[int][]SyntaxToken)
	c.TextEditor.Tokens = nil
	c.TextEditor.prepareVisibleTokens = c.prepareViewportTokens
}

func (c *CodeEditor) invalidateViewportTokens(v *TextEditor, change DocumentChange) {
	if v.tokenCache == nil {
		c.enableViewportTokenCache()
	}
	// Shift cached logical lines when an insertion/deletion changes the line
	// count. Reuse unaffected entries; only the changed window is discarded.
	oldEnd, newEnd := change.OldEndLine, change.EndLine
	delta := newEnd - oldEnd
	next := make(map[int][]SyntaxToken, len(v.tokenCache))
	for line, tokens := range v.tokenCache {
		switch {
		case line < change.StartLine:
			next[line] = tokens
		case line >= oldEnd:
			nl := line + delta
			if nl >= newEnd {
				for i := range tokens {
					tokens[i].Line = nl
				}
				next[nl] = tokens
			}
		}
	}
	v.tokenCache = next
}

func (c *CodeEditor) prepareViewportTokens(startLine, endLine int) {
	v := c.TextEditor
	if v == nil || c.Highlighter == nil || startLine >= endLine {
		return
	}
	if v.tokenCache == nil {
		v.tokenCache = make(map[int][]SyntaxToken)
	}
	// Include a modest context window. It improves multiline lexical constructs
	// without turning a viewport jump into a whole-document highlight.
	contextStart := startLine - largeEditorTokenContext
	if contextStart < 0 {
		contextStart = 0
	}
	if contextStart >= endLine {
		contextStart = startLine
	}
	// If every requested line is already cached, avoid all lexer work.
	complete := true
	for i := startLine; i < endLine; i++ {
		if _, ok := v.tokenCache[i]; !ok {
			complete = false
			break
		}
	}
	if complete {
		return
	}
	var tokens []SyntaxToken
	if h, ok := c.Highlighter.(RangeSyntaxHighlighter); ok {
		tokens = h.HighlightRange(v.Document.Name, v.Language, v.Lines, contextStart, endLine)
	} else {
		tokens = c.Highlighter.Highlight(v.Document.Name, v.Language, v.Lines)
	}
	// The fallback provider may return the complete document. Do not populate a
	// huge cache accidentally; only retain tokens belonging to the requested
	// source window.
	for _, ts := range tokens {
		if ts.Line < startLine || ts.Line >= endLine {
			continue
		}
		v.tokenCache[ts.Line] = append(v.tokenCache[ts.Line], ts)
	}
	// Empty lines need a cache marker too, otherwise every frame would lex them.
	for i := startLine; i < endLine; i++ {
		if _, ok := v.tokenCache[i]; !ok {
			v.tokenCache[i] = nil
		}
	}
}

func (c *CodeEditor) rehighlightText(v *TextEditor) {
	if c.largeDocument {
		c.enableViewportTokenCache()
		if c.foldsNeedRebuild(v, DocumentChange{StartLine: 0, EndLine: len(v.Lines), OldEndLine: 0}) {
			c.rebuildFolds(v)
		}
		return
	}
	// The built-in Go highlighter and fold provider share the same scanner.
	// Use the fused path so large Go files are assembled/scanned only once.
	if strings.EqualFold(v.Language, "go") && isBuiltInGoHighlighter(c.Highlighter) && isBuiltInGoFoldProvider(c.FoldProvider) {
		var folds []FoldRange
		v.Tokens, folds = highlightGoAndFolds(v.Document.Name, v.Lines, c.FoldProvider != nil)
		v.Folds = make(map[int]FoldRange, len(folds))
		for _, f := range folds {
			v.Folds[f.Start] = f
		}
		return
	}
	v.Tokens = v.Tokens[:0]
	if c.Highlighter != nil {
		v.Tokens = c.Highlighter.Highlight(v.Document.Name, v.Language, v.Lines)
	}
	c.rebuildFolds(v)
}

func isBuiltInGoHighlighter(h SyntaxHighlighter) bool {
	switch h.(type) {
	case DefaultSyntaxHighlighter, GoSyntaxHighlighter:
		return true
	default:
		return false
	}
}

func isBuiltInGoFoldProvider(p FoldProvider) bool {
	switch p.(type) {
	case DefaultFoldProvider, GoFoldProvider:
		return true
	default:
		return p == nil
	}
}

// SetHighlighter installs a syntax presentation provider. A nil provider
// disables highlighting.
func (c *CodeEditor) SetHighlighter(h SyntaxHighlighter) {
	c.Highlighter = h
	c.rehighlight()
}

// SetFoldProvider installs a fold-range provider. A nil provider disables
// automatic folding discovery.
func (c *CodeEditor) SetFoldProvider(p FoldProvider) {
	c.FoldProvider = p
	c.rehighlight()
}

// rehighlight is retained as a small compatibility helper for code that used
// the old CodeEditor implementation internally.
func (c *CodeEditor) rehighlight() { c.TextEditor.rehighlight() }
