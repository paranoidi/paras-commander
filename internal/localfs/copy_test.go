package localfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileContextCanceled(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := CopyFile(ctx, src, dst, 4096, false, false, false, false, CopyFileOpts{}, nil)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyFile err = %v, want context.Canceled", err)
	}
}
