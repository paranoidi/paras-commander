package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestSummarizePlanForSource(t *testing.T) {
	root := MustPath("/tmp/garden/alpha")
	plan := []PlanItem{
		{Src: MustPath("/tmp/garden/alpha"), IsDir: true},
		{Src: MustPath("/tmp/garden/alpha/notes.txt"), FileSize: 100},
		{Src: MustPath("/tmp/garden/alpha/link"), IsSymlink: true},
		{Src: MustPath("/tmp/garden/beta/file.txt"), FileSize: 50},
	}
	items, bytes := SummarizePlanForSource(plan, root)
	if items != 3 {
		t.Fatalf("items = %d want 3", items)
	}
	if bytes != 100 {
		t.Fatalf("bytes = %d want 100", bytes)
	}
}

func TestMoveRenameProgressIncrementsPerSourceWithPlan(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	const n = 12
	sources := make([]pathloc.Path, 0, n)
	for i := range n {
		name := fmt.Sprintf("ledger_%d.txt", i)
		path := filepath.Join(srcDir, name)
		if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
		sources = append(sources, MustPath(path))
	}
	plan, _, _, _, err := BuildCopyPlanWithTotalsCtx(context.Background(), sources, MustPath(dstDir), PlanBuildOptions{})
	if err != nil {
		t.Fatalf("BuildCopyPlanWithTotalsCtx: %v", err)
	}
	var counts []int
	progress := func(_, _ string, doneFiles int, _ int64) {
		counts = append(counts, doneFiles)
	}
	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteMoveWithPlan(context.Background(), plan, sources, MustPath(dstDir), opts, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMoveWithPlan: %v", err)
	}
	if done != n {
		t.Fatalf("done files = %d, want %d", done, n)
	}
	if len(counts) != n {
		t.Fatalf("progress emits = %d, want %d", len(counts), n)
	}
	for i, c := range counts {
		want := i + 1
		if c != want {
			t.Fatalf("progress[%d] = %d, want %d", i, c, want)
		}
	}
}

func TestMoveRenameProgressSkipsPostRenameWalkWithPlan(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	tree := filepath.Join(srcDir, "archive")
	if err := os.Mkdir(tree, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	for _, name := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
		if err := os.WriteFile(filepath.Join(tree, name), []byte("payload"), 0o644); err != nil {
			t.Fatalf("write child: %v", err)
		}
	}
	sources := MustPaths(tree)
	plan, tf, _, _, err := BuildCopyPlanWithTotalsCtx(context.Background(), sources, MustPath(dstDir), PlanBuildOptions{})
	if err != nil {
		t.Fatalf("BuildCopyPlanWithTotalsCtx: %v", err)
	}
	if tf < 4 {
		t.Fatalf("total files = %d, want at least 4", tf)
	}
	var counts []int
	progress := func(_, _ string, doneFiles int, _ int64) {
		counts = append(counts, doneFiles)
	}
	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteMoveWithPlan(context.Background(), plan, sources, MustPath(dstDir), opts, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMoveWithPlan: %v", err)
	}
	if done != tf {
		t.Fatalf("done files = %d, want %d", done, tf)
	}
	if len(counts) != 1 {
		t.Fatalf("progress emits = %d, want 1 (plan-based credit, no post-rename walk)", len(counts))
	}
	if counts[0] != tf {
		t.Fatalf("progress done = %d, want %d", counts[0], tf)
	}
}

func TestMoveRenameProgressIncrementsWithoutPlan(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	a := filepath.Join(srcDir, "anchor.txt")
	b := filepath.Join(srcDir, "beacon.txt")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	var counts []int
	progress := func(_, _ string, doneFiles int, _ int64) {
		counts = append(counts, doneFiles)
	}
	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteMove(context.Background(), MustPaths(a, b), MustPath(dstDir), opts, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if done != 2 {
		t.Fatalf("done files = %d, want 2", done)
	}
	if len(counts) < 2 {
		t.Fatalf("progress emits = %d, want at least 2", len(counts))
	}
	last := counts[len(counts)-1]
	if last != 2 {
		t.Fatalf("final progress = %d, want 2", last)
	}
}

func TestMoveRenameProgressReportsBytesWithPlan(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "cargo.txt")
	payload := []byte("shipment-contents")
	if err := os.WriteFile(srcFile, payload, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	sources := MustPaths(srcFile)
	plan, _, _, _, err := BuildCopyPlanWithTotalsCtx(context.Background(), sources, MustPath(dstDir), PlanBuildOptions{})
	if err != nil {
		t.Fatalf("BuildCopyPlanWithTotalsCtx: %v", err)
	}
	var gotBytes int64
	progress := func(_, _ string, doneFiles int, doneBytes int64) {
		if doneFiles != 1 {
			t.Fatalf("doneFiles = %d, want 1", doneFiles)
		}
		gotBytes = doneBytes
	}
	opts := Options{CopyBufferKiB: 4}
	_, doneBytes, err := ExecuteMoveWithPlan(context.Background(), plan, sources, MustPath(dstDir), opts, ProgressEmitThrottle{}, progress, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMoveWithPlan: %v", err)
	}
	wantBytes := int64(len(payload))
	if gotBytes != wantBytes {
		t.Fatalf("progress bytes = %d, want %d", gotBytes, wantBytes)
	}
	if doneBytes != wantBytes {
		t.Fatalf("returned bytes = %d, want %d", doneBytes, wantBytes)
	}
}
