//go:build linux

package localfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCopyFileSparsePreservesHole(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sparse.bin")
	dst := filepath.Join(dir, "copy.bin")

	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("head")); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(context.Background(), src, dst, 4096, false, false, false, false, CopyFileOpts{SparseCopy: true}, nil); err != nil {
		t.Fatalf("CopyFile sparse: %v", err)
	}

	out, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	hole, err := unix.Seek(int(out.Fd()), 4096, unix.SEEK_HOLE)
	if err != nil {
		t.Fatalf("SEEK_HOLE on copy: %v", err)
	}
	if hole == 0 {
		t.Fatal("expected a hole after copied header on sparse-aware filesystem")
	}
}
