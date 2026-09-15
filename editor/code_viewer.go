package editor

import (
	"os"

	"github.com/ZeroGCDev/zerotui/widget"
)

// CodeViewer is the IDE compatibility adapter around the reusable
// widget.CodeEditor. New code should prefer widget.CodeEditor directly;
// Editor owns filesystem persistence.
type CodeViewer struct {
	*widget.CodeEditor
	Path     string
	FileMode os.FileMode
}

func NewCodeViewer() *CodeViewer {
	v := &CodeViewer{
		CodeEditor: widget.NewCodeEditor(),
	}

	v.OnSave = v.saveToDisk

	return v
}

func (v *CodeViewer) Load(path string, data []byte) {
	v.Path = path

	if info, err := os.Stat(path); err == nil {
		v.FileMode = info.Mode().Perm()
	} else {
		v.FileMode = 0644
	}

	v.CodeEditor.Load(path, data)
}

func (v *CodeViewer) saveToDisk() bool {
	if v.ReadOnly || v.Path == "" {
		return false
	}

	mode := v.FileMode

	if mode == 0 {
		mode = 0644
	}

	return os.WriteFile(
		v.Path,
		[]byte(v.Text()),
		mode,
	) == nil
}

// Save is retained for backwards compatibility with the old
// editor.CodeViewer API.
func (v *CodeViewer) Save() bool {
	if v.ReadOnly || v.Path == "" {
		return false
	}

	ok := v.saveToDisk()

	if ok {
		v.Modified = false
		v.SetStatus("Saved")
	} else {
		v.SetStatus("Save failed")
	}

	return ok
}

// Small adapters retained for package-editor tests/source compatibility.
func (v *CodeViewer) toggleComment()         { v.ToggleComment() }
func (v *CodeViewer) duplicateLine()         { v.DuplicateLine() }
func (v *CodeViewer) deleteLine()            { v.DeleteLine() }
func (v *CodeViewer) gotoLine(line int) bool { return v.GotoLineNumber(line) }
