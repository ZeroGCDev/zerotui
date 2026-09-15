package widget

import "strings"

// Document is the reusable text model consumed by CodeEditor. It deliberately
// has no rendering, input, or filesystem responsibilities.
type Document struct {
	Name          string
	Language      string
	Lines         []string
	Revision      uint64
	SavedRevision uint64
	// HistoryLimit controls document-level undo/redo depth. Zero uses the default.
	HistoryLimit int
	LastChange   DocumentChange
	undo         []documentHistoryEntry
	redo         []documentHistoryEntry
	pending      *documentEdit
	subscribers  map[uint64]func(DocumentChange)
	nextSubID    uint64
}

func NewDocument(text string) *Document {
	d := &Document{subscribers: make(map[uint64]func(DocumentChange))}
	d.SetText(text)
	d.MarkSaved()
	return d
}

func (d *Document) SetName(name string) { d.Name = name }
func (d *Document) SetLanguage(language string) {
	d.Language = strings.ToLower(strings.TrimSpace(language))
}

func (d *Document) SetText(text string) {
	d.pending = nil
	d.Lines = strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(d.Lines) == 0 {
		d.Lines = []string{""}
	}
	d.Revision++
	d.LastChange = DocumentChange{Revision: d.Revision, StartLine: 0, EndLine: len(d.Lines), OldEndLine: 0}
	d.undo = nil
	d.redo = nil
	d.notify(d.LastChange)
}

// CommitEdit records one completed content mutation. Views may mutate the
// document's Lines while performing an edit, but revision/dirty bookkeeping
// stays owned by the model rather than by a particular view.
func (d *Document) CommitEdit() {
	if d == nil {
		return
	}
	if d.pending == nil {
		// Preserve the old API: an unscoped CommitEdit records a whole-document
		// replacement. Callers performing interactive edits should use
		// BeginEditRange so history stays proportional to the changed lines.
		d.BeginEdit()
	}
	p := d.pending
	d.pending = nil
	if len(d.Lines) == 0 {
		d.Lines = []string{""}
	}
	newEnd := p.oldEnd + (len(d.Lines) - p.oldLineCount)
	if newEnd < p.start {
		newEnd = p.start
	}
	if newEnd > len(d.Lines) {
		newEnd = len(d.Lines)
	}
	d.undo = append(d.undo, documentHistoryEntry{Start: p.start, End: newEnd, Lines: p.lines})
	if len(d.undo) > d.historyLimit() {
		d.undo = d.undo[1:]
	}
	d.redo = nil
	d.Revision++
	d.LastChange = DocumentChange{Revision: d.Revision, StartLine: p.start, EndLine: newEnd, OldEndLine: p.oldEnd}
	d.notify(d.LastChange)
}

// Mutate runs a content mutation and commits it as one document revision.
// This is the preferred API for reusable widgets that operate on Documents.
func (d *Document) Mutate(fn func(*Document)) {
	if fn == nil {
		return
	}
	d.BeginEdit()
	fn(d)
	d.CommitEdit()
}

// ReplaceLines replaces the half-open line range [start, end) with lines.
// It records only the affected range for document-level undo.
func (d *Document) SetLine(index int, line string) bool {
	if index < 0 || index >= len(d.Lines) {
		return false
	}
	d.BeginEditRange(index, index+1)
	d.Lines[index] = line
	d.commitChange(index, index+1, index+1)
	return true
}

// InsertLines inserts lines before index and records one revision.
func (d *Document) InsertLines(index int, lines ...string) bool {
	if index < 0 || index > len(d.Lines) || len(lines) == 0 {
		return index >= 0 && index <= len(d.Lines)
	}
	inserted := append([]string(nil), lines...)
	d.BeginEditRange(index, index)
	out := make([]string, 0, len(d.Lines)+len(inserted))
	out = append(out, d.Lines[:index]...)
	out = append(out, inserted...)
	out = append(out, d.Lines[index:]...)
	d.Lines = out
	d.commitChange(index, index+len(inserted), index)
	return true
}

// DeleteLines deletes the half-open line range [start, end) and records one revision.
func (d *Document) DeleteLines(start, end int) bool {
	return d.ReplaceLines(start, end, nil)
}

func (d *Document) ReplaceLines(start, end int, lines []string) bool {
	if start < 0 || end < start || start > len(d.Lines) || end > len(d.Lines) {
		return false
	}
	repl := append([]string(nil), lines...)
	d.BeginEditRange(start, end)
	out := make([]string, 0, len(d.Lines)-(end-start)+len(repl))
	out = append(out, d.Lines[:start]...)
	out = append(out, repl...)
	out = append(out, d.Lines[end:]...)
	if len(out) == 0 {
		out = []string{""}
	}
	d.Lines = out
	d.commitChange(start, start+len(repl), end)
	return true
}

func (d *Document) commitChange(start, end, oldEnd int) {
	if d.pending == nil {
		d.BeginEditRange(start, oldEnd)
	}
	p := d.pending
	d.pending = nil
	if len(d.Lines) == 0 {
		d.Lines = []string{""}
	}
	if end < start {
		end = start
	}
	if end > len(d.Lines) {
		end = len(d.Lines)
	}
	d.undo = append(d.undo, documentHistoryEntry{Start: p.start, End: end, Lines: p.lines})
	if len(d.undo) > d.historyLimit() {
		d.undo = d.undo[1:]
	}
	d.redo = nil
	d.Revision++
	d.LastChange = DocumentChange{Revision: d.Revision, StartLine: start, EndLine: end, OldEndLine: oldEnd}
	d.notify(d.LastChange)
}

// Subscribe registers a document change listener. The returned function removes
// the listener and is safe to call more than once. Notifications are delivered
// synchronously after a committed mutation.
func (d *Document) Subscribe(fn func(DocumentChange)) func() {
	if d == nil || fn == nil {
		return func() {}
	}
	if d.subscribers == nil {
		d.subscribers = make(map[uint64]func(DocumentChange))
	}
	d.nextSubID++
	id := d.nextSubID
	d.subscribers[id] = fn
	removed := false
	return func() {
		if removed {
			return
		}
		removed = true
		delete(d.subscribers, id)
	}
}

func (d *Document) notify(change DocumentChange) {
	if d == nil || len(d.subscribers) == 0 {
		return
	}
	// TextEditor normally has exactly one document subscriber. Avoid creating
	// a temporary listener slice for every keystroke; retain the slice path only
	// for the uncommon multi-subscriber case.
	if len(d.subscribers) == 1 {
		for _, fn := range d.subscribers {
			fn(change)
		}
		return
	}
	listeners := make([]func(DocumentChange), 0, len(d.subscribers))
	for _, fn := range d.subscribers {
		listeners = append(listeners, fn)
	}
	for _, fn := range listeners {
		fn(change)
	}
}

func diffLineRange(oldLines, newLines []string) (start, oldEnd, newEnd int) {
	if oldLines == nil {
		return 0, 0, len(newLines)
	}
	start = 0
	for start < len(oldLines) && start < len(newLines) && oldLines[start] == newLines[start] {
		start++
	}
	oldEnd, newEnd = len(oldLines), len(newLines)
	for oldEnd > start && newEnd > start && oldLines[oldEnd-1] == newLines[newEnd-1] {
		oldEnd--
		newEnd--
	}
	return
}

// LineCount returns the current logical line count without constructing any view state.
func (d *Document) LineCount() int {
	if d == nil {
		return 0
	}
	return len(d.Lines)
}

// Line returns a logical line by zero-based index.
func (d *Document) Line(index int) (string, bool) {
	if d == nil || index < 0 || index >= len(d.Lines) {
		return "", false
	}
	return d.Lines[index], true
}

// ByteOffset returns the byte offset of a zero-based (line, rune-column)
// position in the document text. Newlines count as one byte.
func (d *Document) ByteOffset(line, col int) int {
	if d == nil || len(d.Lines) == 0 {
		return 0
	}
	if line < 0 {
		line = 0
	}
	if line >= len(d.Lines) {
		line = len(d.Lines) - 1
	}
	if col < 0 {
		col = 0
	}
	lineText := d.Lines[line]
	off := 0
	for i := 0; i < line; i++ {
		off += len(d.Lines[i]) + 1
	}
	count := 0
	for i := range lineText {
		if count == col {
			return off + i
		}
		count++
	}
	return off + len(lineText)
}

// PositionAtByteOffset converts a byte offset in Document.Text() into a
// zero-based line/rune-column position. Offsets are clamped to document bounds.
func (d *Document) PositionAtByteOffset(offset int) (line, col int) {
	if d == nil || len(d.Lines) == 0 {
		return 0, 0
	}
	if offset < 0 {
		offset = 0
	}
	for i, text := range d.Lines {
		if offset <= len(text) {
			if offset == len(text) {
				return i, len([]rune(text))
			}
			return i, len([]rune(text[:offset]))
		}
		offset -= len(text) + 1
	}
	last := len(d.Lines) - 1
	return last, len([]rune(d.Lines[last]))
}

func (d *Document) Text() string   { return strings.Join(d.Lines, "\n") }
func (d *Document) Modified() bool { return d.Revision != d.SavedRevision }
func (d *Document) MarkSaved()     { d.SavedRevision = d.Revision }
