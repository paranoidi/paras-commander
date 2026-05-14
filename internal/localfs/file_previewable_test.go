package localfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFilePreviewableText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.txt")
	if err := os.WriteFile(p, []byte("hello 世界\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckFilePreviewable(p); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFilePreviewableBinaryNUL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(p, []byte("a\x00b"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckFilePreviewable(p)
	if !errors.Is(err, ErrFilePreviewBinary) {
		t.Fatalf("err = %v want ErrFilePreviewBinary", err)
	}
}

func TestCheckFilePreviewableDirectory(t *testing.T) {
	dir := t.TempDir()
	err := CheckFilePreviewable(dir)
	if !errors.Is(err, ErrFilePreviewIsDir) {
		t.Fatalf("err = %v want ErrFilePreviewIsDir", err)
	}
}
