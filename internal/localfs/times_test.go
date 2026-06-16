package localfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileTimesUsesModTimeOnNonLinuxOrMissingSys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	mod := time.Date(2020, 3, 15, 10, 30, 0, 0, time.UTC)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	atime, mtime := FileTimes(info)
	if !mtime.Equal(mod) {
		t.Fatalf("mtime = %v, want %v", mtime, mod)
	}
	if atime.IsZero() {
		t.Fatal("atime is zero")
	}
}
