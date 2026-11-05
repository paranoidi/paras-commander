package dirwatch

import (
	"os"
	"path/filepath"
	"testing"
)

// The portable surface: on platforms without inotify it is inert, on Linux it must still be
// callable in any order without errors or hangs.
func TestNewSetCloseSmoke(t *testing.T) {
	w := New(func(string) {})
	dir := t.TempDir()
	w.Set([]string{dir, filepath.Join(dir, "missing"), ""})
	w.Set(nil)
	w.Close()
	w.Close()
	w.Set([]string{dir})
}

func TestSetToleratesFilePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "amber.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := New(func(string) {})
	defer w.Close()
	w.Set([]string{file})
}
