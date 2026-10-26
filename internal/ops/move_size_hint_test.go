package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestExecuteMoveCreditsSourceSizeHints checks that a rename credits its source's estimated
// bytes to the done total, an unknown hint (-1) credits nothing, and progress reports the credit.
func TestExecuteMoveCreditsSourceSizeHints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lantern := filepath.Join(root, "lantern-harvest")
	if err := os.Mkdir(lantern, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lantern, "meadow-quiet.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	pebble := filepath.Join(root, "pebble-orchard.log")
	if err := os.WriteFile(pebble, []byte("xyz"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "harbor")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	var lastBytes int64
	progress := func(_, _ string, _ int, b int64) { lastBytes = b }
	opts := Options{CopyBufferKiB: 4, SourceSizeHints: []int64{4096, -1}}
	files, bytes, err := ExecuteMove(context.Background(), MustPaths(lantern, pebble), MustPath(dst), opts, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if files != 2 {
		t.Fatalf("files = %d, want 2", files)
	}
	if bytes != 4096 {
		t.Fatalf("bytes = %d, want 4096 (hint of the renamed dir, nothing for the unknown hint)", bytes)
	}
	if lastBytes != 4096 {
		t.Fatalf("last progress bytes = %d, want 4096", lastBytes)
	}
}

// TestExecuteMoveIgnoresMisalignedHints checks that hints not aligned with sources are ignored.
func TestExecuteMoveIgnoresMisalignedHints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(root, "copper-willow.md")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "valley")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{CopyBufferKiB: 4, SourceSizeHints: []int64{10, 20}}
	_, bytes, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dst), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if bytes != 0 {
		t.Fatalf("bytes = %d, want 0 for misaligned hints", bytes)
	}
}
