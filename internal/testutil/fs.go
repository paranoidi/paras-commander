package testutil

import (
	"os"
	"testing"
)

// WriteFile creates path with default test content ("content", mode 0644).
func WriteFile(t *testing.T, path string) {
	t.Helper()
	WriteFileBytes(t, path, []byte("content"))
}

// WriteFileBytes creates path with the given content (mode 0644).
func WriteFileBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
