package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestCopyFileTransferRemovesPartialOnFailure(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(srcPath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstPath := filepath.Join(dir, "dst.bin")
	src, err := pathloc.File(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := pathloc.File(dstPath)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = copyFileTransfer(ctx, src, dst, DefaultOptions(), nil, make([]byte, 4096), nil)
	if err == nil {
		t.Fatal("copyFileTransfer() error = nil, want error")
	}
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Fatalf("partial destination %q should be removed after failure, stat err=%v", dstPath, statErr)
	}
}
