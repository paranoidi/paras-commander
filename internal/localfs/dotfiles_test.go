package localfs

import (
	"path/filepath"
	"testing"
)

func TestDirHasDotfileNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	has, err := DirHasDotfileNames(dir)
	if err != nil {
		t.Fatalf("DirHasDotfileNames() error = %v", err)
	}
	if has {
		t.Fatal("empty dir: has = true, want false")
	}

	mustWriteFile(t, filepath.Join(dir, ".hidden"))
	has, err = DirHasDotfileNames(dir)
	if err != nil {
		t.Fatalf("DirHasDotfileNames() error = %v", err)
	}
	if !has {
		t.Fatal("has = false, want true after creating .hidden")
	}
}
