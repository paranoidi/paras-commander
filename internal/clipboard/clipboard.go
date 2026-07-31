// Package clipboard writes text to the system clipboard when a helper tool is available.
package clipboard

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync"
)

var (
	mu      sync.Mutex
	lastSet string
)

// LastSet returns the most recent text passed to Set (in-process fallback).
func LastSet() string {
	mu.Lock()
	defer mu.Unlock()
	return lastSet
}

// Reset clears the in-process clipboard store (for tests).
func Reset() {
	mu.Lock()
	lastSet = ""
	mu.Unlock()
}

// Set stores text and attempts to copy it to the OS clipboard.
// Returns nil when an external tool succeeds; otherwise returns an error after
// storing the value in-process (callers may still treat the copy as best-effort).
func Set(text string) error {
	mu.Lock()
	lastSet = text
	mu.Unlock()

	if err := setOS(text); err != nil {
		return fmt.Errorf("clipboard: %w", err)
	}
	return nil
}

func setOS(text string) error {
	tools := []struct {
		name string
		args []string
	}{
		{"wl-copy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"-ib"}},
		{"pbcopy", nil},
	}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.name); err != nil {
			continue
		}
		cmd := exec.Command(tool.name, tool.args...)
		cmd.Stdin = bytes.NewReader([]byte(text))
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard tool available")
}
