package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionStripPathIsDir_fileVsDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if selectionStripPathIsDir(file) {
		t.Fatal("file should not be dir")
	}
	if !selectionStripPathIsDir(dir) {
		t.Fatal("dir should be dir")
	}
	if selectionStripPathIsDir(filepath.Join(dir, "missing")) {
		t.Fatal("missing path should not be dir")
	}
}
