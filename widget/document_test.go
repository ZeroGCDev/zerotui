package widget

import (
	"strings"
	"testing"
)

func TestDocumentMutationAPIs(t *testing.T) {
	d := NewDocument("one\ntwo")
	r := d.Revision
	if !d.SetLine(1, "TWO") || d.Lines[1] != "TWO" || d.Revision != r+1 {
		t.Fatalf("SetLine failed: %#v rev=%d", d, d.Revision)
	}
	r = d.Revision
	if !d.InsertLines(1, "insert") || d.Text() != "one\ninsert\nTWO" || d.Revision != r+1 {
		t.Fatalf("InsertLines failed: %q rev=%d", d.Text(), d.Revision)
	}
	r = d.Revision
	if !d.DeleteLines(1, 2) || d.Text() != "one\nTWO" || d.Revision != r+1 {
		t.Fatalf("DeleteLines failed: %q rev=%d", d.Text(), d.Revision)
	}
	r = d.Revision
	d.Mutate(func(doc *Document) { doc.Lines[0] = "ONE" })
	if d.Text() != "ONE\nTWO" || d.Revision != r+1 {
		t.Fatalf("Mutate failed: %q rev=%d", d.Text(), d.Revision)
	}
}

func TestCodeEditorsShareDocumentRevisionAndRefresh(t *testing.T) {
	d := NewDocument("alpha")
	a := NewCodeEditor()
	b := NewCodeEditor()
	a.SetDocument(d)
	b.SetDocument(d)
	before := d.Revision
	a.recordEdit()
	a.insertRune('X')
	if b.Lines[0] != "Xalpha" {
		t.Fatalf("shared document did not propagate: %q", b.Lines[0])
	}
	if d.Revision != before+1 || !d.Modified() {
		t.Fatalf("unexpected document state: rev=%d saved=%d", d.Revision, d.SavedRevision)
	}
	b.syncDocument()
	if b.Modified != d.Modified() || b.observedRevision != d.Revision {
		t.Fatalf("view did not synchronize revision: modified=%v observed=%d doc=%d", b.Modified, b.observedRevision, d.Revision)
	}
}

func TestDocumentUndoRedoAndChangeRange(t *testing.T) {
	d := NewDocument("one\ntwo\nthree")
	startRev := d.Revision
	d.BeginEdit()
	if !d.SetLine(1, "TWO") {
		t.Fatal("SetLine failed")
	}
	if d.LastChange.StartLine != 1 || d.LastChange.EndLine != 2 {
		t.Fatalf("unexpected change range: %+v", d.LastChange)
	}
	if !d.CanUndo() {
		t.Fatal("expected undo")
	}
	if !d.UndoContent() {
		t.Fatal("undo failed")
	}
	if got := d.Text(); got != "one\ntwo\nthree" {
		t.Fatalf("undo text = %q", got)
	}
	if d.Revision <= startRev {
		t.Fatal("undo should advance revision")
	}
	if !d.RedoContent() {
		t.Fatal("redo failed")
	}
	if got := d.Text(); got != "one\nTWO\nthree" {
		t.Fatalf("redo text = %q", got)
	}
}

func TestCodeEditorsShareDocumentHistory(t *testing.T) {
	d := NewDocument("hello")
	a := NewCodeEditor()
	b := NewCodeEditor()
	a.SetDocument(d)
	b.SetDocument(d)
	a.insertRune('!')
	if d.Text() != "!hello" {
		t.Fatalf("document = %q", d.Text())
	}
	// Draw synchronizes the second view with the shared model.
	b.syncDocument()
	if b.Text() != "!hello" {
		t.Fatalf("second view = %q", b.Text())
	}
	if !a.Undo() {
		t.Fatal("undo failed")
	}
	b.syncDocument()
	if d.Text() != "hello" || b.Text() != "hello" {
		t.Fatalf("shared undo failed: doc=%q view=%q", d.Text(), b.Text())
	}
	if !a.Redo() {
		t.Fatal("redo failed")
	}
	b.syncDocument()
	if d.Text() != "!hello" || b.Text() != "!hello" {
		t.Fatalf("shared redo failed: doc=%q view=%q", d.Text(), b.Text())
	}
}

func TestDocumentSubscriptions(t *testing.T) {
	d := NewDocument("one\ntwo")
	var changes []DocumentChange
	unsub := d.Subscribe(func(ch DocumentChange) { changes = append(changes, ch) })
	d.SetLine(1, "TWO")
	if len(changes) != 1 || changes[0].StartLine != 1 || changes[0].EndLine != 2 || changes[0].OldEndLine != 2 {
		t.Fatalf("unexpected subscription change: %#v", changes)
	}
	unsub()
	d.SetLine(0, "ONE")
	if len(changes) != 1 {
		t.Fatalf("unsubscribe failed: %#v", changes)
	}
}

func TestDocumentSubscriptionReportsLineCountChange(t *testing.T) {
	d := NewDocument("a\nb\nc")
	var ch DocumentChange
	d.Subscribe(func(v DocumentChange) { ch = v })
	if !d.DeleteLines(1, 3) {
		t.Fatal("delete failed")
	}
	if ch.StartLine != 1 || ch.OldEndLine != 3 || ch.EndLine != 1 {
		t.Fatalf("unexpected deletion range: %#v", ch)
	}
}

func TestDocumentRangeHistoryDoesNotCopyWholeDocument(t *testing.T) {
	d := NewDocument(strings.Repeat("line\n", 100000))
	oldCap := cap(d.undo)
	_ = oldCap
	d.SetLine(50000, "changed")
	if !d.UndoContent() {
		t.Fatal("undo failed")
	}
	if d.Lines[50000] != "line" {
		t.Fatalf("undo line=%q", d.Lines[50000])
	}
	if !d.RedoContent() {
		t.Fatal("redo failed")
	}
	if len(d.undo[0].Lines) != 1 {
		t.Fatalf("history retained %d lines", len(d.undo[0].Lines))
	}
}

func TestDocumentBytePositions(t *testing.T) {
	d := NewDocument("héllo\n世界")
	if got := d.ByteOffset(0, 2); got != 3 {
		t.Fatalf("byte offset=%d, want 3", got)
	}
	line, col := d.PositionAtByteOffset(3)
	if line != 0 || col != 2 {
		t.Fatalf("position=(%d,%d), want (0,2)", line, col)
	}
	line, col = d.PositionAtByteOffset(len("héllo") + 1 + len("世界"))
	if line != 1 || col != 2 {
		t.Fatalf("end position=(%d,%d), want (1,2)", line, col)
	}
}

func TestTextEditorUnicodeSearchAndReplace(t *testing.T) {
	e := NewTextEditor()
	e.SetText("alpha\nHELLO 世界\nhello")
	e.Search = "hello"
	e.searchHit = 0
	if !e.findNext() || e.Cursor != 1 || e.Col != 0 {
		t.Fatalf("first search cursor=(%d,%d)", e.Cursor, e.Col)
	}
	e.Search = "世界"
	e.searchHit = 0
	if !e.findNext() || e.Cursor != 1 || e.Col != 6 {
		t.Fatalf("unicode search cursor=(%d,%d)", e.Cursor, e.Col)
	}
	if got := e.ReplaceAll("hello", "hi"); got != 2 {
		t.Fatalf("replace count=%d", got)
	}
	if e.Lines[1] != "hi 世界" || e.Lines[2] != "hi" {
		t.Fatalf("replace result=%q / %q", e.Lines[1], e.Lines[2])
	}
	if !e.Undo() || e.Lines[1] != "HELLO 世界" || e.Lines[2] != "hello" {
		t.Fatalf("bulk replace undo failed: %q / %q", e.Lines[1], e.Lines[2])
	}
}

func TestTextEditorWideRuneViewportUsesTerminalCells(t *testing.T) {
	e := NewTextEditor()
	e.SetText("界A")
	e.viewportH, e.viewportW = 5, 2
	e.Cursor, e.Col = 0, 2
	e.ensureScroll(5, 2)
	if e.HScroll != 1 {
		t.Fatalf("HScroll=%d, want 1 cell before caret", e.HScroll)
	}
	if got := runeAtDisplayColumn("界A", 1); got != 0 {
		t.Fatalf("mouse mapping=%d", got)
	}
	if got := displayColumnAtRune("界A", 1); got != 2 {
		t.Fatalf("display column=%d", got)
	}
}
