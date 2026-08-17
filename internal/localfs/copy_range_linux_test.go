//go:build linux

package localfs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTryKernelFileRangeCopyChunked(t *testing.T) {
	dir := t.TempDir()
	payload := bytes.Repeat([]byte("wombat badger otter "), 1000) // > one chunk with a tiny chunk size
	srcPath := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	srcFile, err := os.Open(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srcFile.Close() }()

	dstPath := filepath.Join(dir, "dst.bin")
	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dstFile.Close() }()

	var written int64
	ok, err := tryKernelFileRangeCopy(context.Background(), srcFile, dstFile, int64(len(payload)), 64, func(n int64) { written += n })
	if err != nil {
		if errors.Is(err, os.ErrInvalid) {
			t.Skipf("copy_file_range unsupported on this filesystem: %v", err)
		}
		t.Fatalf("tryKernelFileRangeCopy err = %v", err)
	}
	if !ok {
		t.Skip("copy_file_range not supported on this filesystem")
	}
	if written != int64(len(payload)) {
		t.Fatalf("onWritten total = %d, want %d", written, len(payload))
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("copied content mismatch across chunk boundaries")
	}
}

func TestTryKernelFileRangeCopyCanceled(t *testing.T) {
	dir := t.TempDir()
	payload := bytes.Repeat([]byte("x"), 1024)
	srcPath := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(srcPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	srcFile, err := os.Open(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srcFile.Close() }()

	dstPath := filepath.Join(dir, "dst.bin")
	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dstFile.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok, err := tryKernelFileRangeCopy(ctx, srcFile, dstFile, int64(len(payload)), 16, nil)
	if ok || !errors.Is(err, context.Canceled) {
		t.Fatalf("tryKernelFileRangeCopy(canceled) = (%v, %v), want (false, context.Canceled)", ok, err)
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("destination has %d bytes, want 0 (copy should not have started)", len(got))
	}
}
