package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestMoveRenameProgressIncrementsPerSource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	const n = 12
	sources := make([]pathloc.Path, 0, n)
	for i := range n {
		path := filepath.Join(srcDir, fmt.Sprintf("ledger_%d", i))
		if err := os.MkdirAll(filepath.Join(path, "nested"), 0o755); err != nil {
			t.Fatalf("mkdir source: %v", err)
		}
		if err := os.WriteFile(filepath.Join(path, "nested", "payload.txt"), []byte("payload"), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
		sources = append(sources, MustPath(path))
	}
	var counts []int
	progress := func(_, _ string, doneFiles int, _ int64) {
		counts = append(counts, doneFiles)
	}
	done, _, err := ExecuteMove(context.Background(), sources, MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if done != n {
		t.Fatalf("done files = %d, want %d (one per top-level source)", done, n)
	}
	if len(counts) != n {
		t.Fatalf("progress emits = %d, want %d (no per-node walk)", len(counts), n)
	}
	for i, c := range counts {
		if c != i+1 {
			t.Fatalf("progress[%d] = %d, want %d", i, c, i+1)
		}
	}
	for i := range n {
		if got := readFileContent(t, filepath.Join(dstDir, fmt.Sprintf("ledger_%d", i), "nested", "payload.txt")); got != "payload" {
			t.Fatalf("moved content = %q", got)
		}
	}
}
