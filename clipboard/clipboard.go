// Package clipboard provides the ZeroTUI process-local clipboard.
//
// ZeroTUI deliberately does not read from or write to the host operating
// system clipboard. Copy/paste stays inside the running application, which
// avoids invoking desktop clipboard helpers or emitting clipboard protocols
// such as OSC 52.
package clipboard

import "sync"

var clipboard struct {
	sync.RWMutex
	text  string
	valid bool
}

// Copy stores text in ZeroTUI's process-local clipboard.
// It never accesses the host OS clipboard.
func Copy(s string) bool {
	clipboard.Lock()
	clipboard.text = s
	clipboard.valid = true
	clipboard.Unlock()
	return true
}

// Paste returns the most recently copied text from this ZeroTUI process.
// It never reads the host OS clipboard.
func Paste() (string, bool) {
	clipboard.RLock()
	text, valid := clipboard.text, clipboard.valid
	clipboard.RUnlock()
	return text, valid
}
