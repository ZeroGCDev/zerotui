package widget

// DocumentChange describes the content change represented by a document revision.
// EndLine is exclusive. A nil/zero-value change means no content change has been recorded.
type DocumentChange struct {
	Revision   uint64
	StartLine  int
	EndLine    int
	OldEndLine int
}

type documentHistoryEntry struct {
	Start int
	End   int // range in the document state immediately before the recorded replacement
	Lines []string
}

type documentEdit struct {
	start        int
	oldEnd       int
	oldLineCount int
	lines        []string
}

const defaultDocumentHistoryLimit = 100

// BeginEdit records the whole document as the undo point. It remains useful for
// model-level bulk mutations, but interactive editors should prefer BeginEditRange.
func (d *Document) BeginEdit() {
	if d == nil {
		return
	}
	d.BeginEditRange(0, len(d.Lines))
}

// BeginEditRange records only the lines expected to be changed. This is the
// important large-document path: an edit to one line does not retain a copy of
// a million-line document merely to support undo.
func (d *Document) BeginEditRange(start, end int) {
	if d == nil {
		return
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(d.Lines) {
		start = len(d.Lines)
	}
	if end > len(d.Lines) {
		end = len(d.Lines)
	}
	d.pending = &documentEdit{
		start: start, oldEnd: end, oldLineCount: len(d.Lines),
		lines: append([]string(nil), d.Lines[start:end]...),
	}
}

func (d *Document) UndoContent() bool {
	if d == nil || len(d.undo) == 0 {
		return false
	}
	e := d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	if e.Start < 0 || e.Start > len(d.Lines) || e.End < e.Start || e.End > len(d.Lines) {
		return false
	}
	current := append([]string(nil), d.Lines[e.Start:e.End]...)
	d.Lines = replaceDocumentRange(d.Lines, e.Start, e.End, e.Lines)
	// Redo replaces the restored old range with the state we just removed.
	d.redo = append(d.redo, documentHistoryEntry{Start: e.Start, End: e.Start + len(e.Lines), Lines: current})
	d.Revision++
	d.LastChange = DocumentChange{Revision: d.Revision, StartLine: e.Start, EndLine: e.Start + len(e.Lines), OldEndLine: e.End}
	d.notify(d.LastChange)
	return true
}

func (d *Document) RedoContent() bool {
	if d == nil || len(d.redo) == 0 {
		return false
	}
	e := d.redo[len(d.redo)-1]
	d.redo = d.redo[:len(d.redo)-1]
	if e.Start < 0 || e.Start > len(d.Lines) || e.End < e.Start || e.End > len(d.Lines) {
		return false
	}
	current := append([]string(nil), d.Lines[e.Start:e.End]...)
	d.Lines = replaceDocumentRange(d.Lines, e.Start, e.End, e.Lines)
	// Undo of the redo restores the state that was present immediately before it.
	d.undo = append(d.undo, documentHistoryEntry{Start: e.Start, End: e.Start + len(e.Lines), Lines: current})
	if len(d.undo) > d.historyLimit() {
		d.undo = d.undo[1:]
	}
	d.Revision++
	d.LastChange = DocumentChange{Revision: d.Revision, StartLine: e.Start, EndLine: e.Start + len(e.Lines), OldEndLine: e.End}
	d.notify(d.LastChange)
	return true
}

func replaceDocumentRange(lines []string, start, end int, replacement []string) []string {
	// Most interactive undo/redo operations replace the same number of lines
	// that they originally captured (for example, editing one character in one
	// line). Reuse the existing line slice in that common case instead of
	// allocating and copying the entire document. Structural edits that change
	// the line count still take the general rebuilding path below.
	if end-start == len(replacement) && start >= 0 && end <= len(lines) {
		copy(lines[start:end], replacement)
		return lines
	}
	n := len(lines) - (end - start) + len(replacement)
	if n < 1 {
		n = 1
	}
	out := make([]string, 0, n)
	out = append(out, lines[:start]...)
	out = append(out, replacement...)
	out = append(out, lines[end:]...)
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

func (d *Document) CanUndo() bool { return d != nil && len(d.undo) > 0 }
func (d *Document) CanRedo() bool { return d != nil && len(d.redo) > 0 }

func (d *Document) historyLimit() int {
	if d.HistoryLimit > 0 {
		return d.HistoryLimit
	}
	return defaultDocumentHistoryLimit
}

// coalesceLastUndo removes the newest redundant checkpoint after adjacent text
// insertion has already committed. It is intentionally internal: callers that
// perform structural edits should keep each operation independently undoable.
func (d *Document) coalesceLastUndo() {
	if d == nil || len(d.undo) < 2 {
		return
	}
	a := d.undo[len(d.undo)-2]
	b := d.undo[len(d.undo)-1]
	if a.Start != b.Start || a.End != b.End || len(a.Lines) != len(b.Lines) {
		return
	}
	d.undo = d.undo[:len(d.undo)-1]
}
