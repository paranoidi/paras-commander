package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func readFileContent(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(b)
}

func TestExecuteCopyUsingPlanChanParityWithExecuteCopyUsingPlan(t *testing.T) {
	srcSlice := buildStreamFixtureTree(t)
	srcChan := buildStreamFixtureTree(t)
	dstSlice := filepath.Join(t.TempDir(), "orchard")
	dstChan := filepath.Join(t.TempDir(), "orchard")

	plan, err := BuildPlanCtx(context.Background(), MustPaths(srcSlice), MustPath(dstSlice), true, PlanBuildOptions{})
	if err != nil {
		t.Fatalf("BuildPlanCtx error = %v", err)
	}
	if err := os.MkdirAll(dstSlice, 0o755); err != nil {
		t.Fatalf("mkdir dstSlice: %v", err)
	}
	sliceFiles, sliceBytes, err := ExecuteCopyUsingPlan(context.Background(), plan, MustPaths(srcSlice), MustPath(dstSlice), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("ExecuteCopyUsingPlan error = %v", err)
	}

	planCh := make(chan PlanItem, 8)
	go func() {
		if err := BuildPlanStreamCtx(context.Background(), MustPaths(srcChan), MustPath(dstChan), true, PlanBuildOptions{}, planCh); err != nil {
			t.Errorf("BuildPlanStreamCtx error = %v", err)
		}
	}()
	// Deliberately no MkdirAll(dstChan) here: dstChan must not exist yet when the walk resolves
	// the top-level source's destination name (a single-directory-source copy to a new name
	// relies on that — see BuildPlanStreamCtx's ordering comment); copyDirItem creates dstChan
	// lazily once the walk's first item (the top-level directory) reaches it.
	chanFiles, chanBytes, err := ExecuteCopyUsingPlanChan(context.Background(), planCh, nil, MustPaths(srcChan), MustPath(dstChan), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("ExecuteCopyUsingPlanChan error = %v", err)
	}

	if chanFiles != sliceFiles {
		t.Fatalf("doneFiles = %d, want %d (slice-based)", chanFiles, sliceFiles)
	}
	if chanBytes != sliceBytes {
		t.Fatalf("doneBytes = %d, want %d (slice-based)", chanBytes, sliceBytes)
	}
	want := readFileContent(t, filepath.Join(dstSlice, "harbor.txt"))
	got := readFileContent(t, filepath.Join(dstChan, "harbor.txt"))
	if got != want {
		t.Fatalf("harbor.txt content = %q, want %q", got, want)
	}
	want = readFileContent(t, filepath.Join(dstSlice, "lantern", "copper.txt"))
	got = readFileContent(t, filepath.Join(dstChan, "lantern", "copper.txt"))
	if got != want {
		t.Fatalf("lantern/copper.txt content = %q, want %q", got, want)
	}
}

func TestExecuteCopyUsingPlanChanCancellationMidStream(t *testing.T) {
	src := buildStreamFixtureTree(t)
	dst := filepath.Join(t.TempDir(), "orchard")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	planCh := make(chan PlanItem) // unbuffered: producer blocks on send until we read
	go func() {
		_ = BuildPlanStreamCtx(ctx, MustPaths(src), MustPath(dst), true, PlanBuildOptions{}, planCh)
	}()

	// Cancel immediately: the executor must observe ctx cancellation instead of hanging on a
	// blocked receive.
	cancel()
	done := make(chan struct{})
	var err error
	go func() {
		_, _, err = ExecuteCopyUsingPlanChan(ctx, planCh, nil, MustPaths(src), MustPath(dst), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for ExecuteCopyUsingPlanChan to return after cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestExecuteCopyUsingPlanChanProducerErrorAfterPartialTransfer(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "orchard")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	fileA := filepath.Join(srcDir, "meadow.txt")
	if err := os.WriteFile(fileA, []byte("meadow-content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srcLoc, err := pathloc.File(fileA)
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}
	dstLoc, err := pathloc.File(filepath.Join(dstDir, "meadow.txt"))
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}

	planCh := make(chan PlanItem, 1)
	planCh <- PlanItem{Src: srcLoc, Dst: dstLoc, FileSize: int64(len("meadow-content"))}
	close(planCh)
	wantErr := errors.New("simulated enumeration failure deep in the tree")
	planErr := func() error { return wantErr }

	doneFiles, _, err := ExecuteCopyUsingPlanChan(context.Background(), planCh, planErr, MustPaths(srcDir), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrapping %v", err, wantErr)
	}
	if doneFiles != 1 {
		t.Fatalf("doneFiles = %d, want 1 (the item transferred before the producer error surfaced)", doneFiles)
	}
	if got := readFileContent(t, filepath.Join(dstDir, "meadow.txt")); got != "meadow-content" {
		t.Fatalf("partial transfer not kept: content = %q", got)
	}
}

func TestExecuteMoveWithPlanChanRenameFastPathDrainsChannel(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "orchard")
	fileA := filepath.Join(srcDir, "meadow.txt")
	if err := os.WriteFile(fileA, []byte("meadow-content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx := context.Background()
	planCh := make(chan PlanItem, 8)
	go func() {
		// This producer's items are never needed by the rename fast path (same filesystem,
		// no fallback), but ExecuteMoveWithPlanChan must still drain and let it finish instead
		// of leaving it blocked on a full channel.
		_ = BuildPlanStreamCtx(ctx, MustPaths(srcDir), MustPath(dstDir), true, PlanBuildOptions{}, planCh)
	}()

	done := make(chan struct{})
	var doneFiles int
	var err error
	go func() {
		doneFiles, _, err = ExecuteMoveWithPlanChan(ctx, planCh, nil, MustPaths(srcDir), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: ExecuteMoveWithPlanChan should return promptly once rename succeeds, not block on draining the channel")
	}
	if err != nil {
		t.Fatalf("ExecuteMoveWithPlanChan error = %v", err)
	}
	// The rename fast path renames srcDir wholesale in one os.Rename, then (since it wasn't
	// given a pre-built plan — mirroring ExecuteMove's own behavior) walks the destination to
	// count nodes for progress: the renamed directory itself plus meadow.txt, so 2.
	if doneFiles != 2 {
		t.Fatalf("doneFiles = %d, want 2 (renamed dir + meadow.txt)", doneFiles)
	}
	if _, statErr := os.Stat(fileA); !os.IsNotExist(statErr) {
		t.Fatalf("source should be gone after rename: statErr = %v", statErr)
	}
	if got := readFileContent(t, filepath.Join(dstDir, "meadow.txt")); got != "meadow-content" {
		t.Fatalf("moved content = %q, want meadow-content", got)
	}
}

// TestMoveCopyFallbackChanParity exercises the same executeCopyIter+finishMoveCopyPhase
// machinery ExecuteMoveWithPlanChan uses for its cross-device copy-fallback phase (which needs a
// real second filesystem to trigger via the public API — not available in this test
// environment), verifying it behaves like the slice-backed executeMoveCopyPhase on an identical
// fixture: files land at destination and sources are removed.
func TestMoveCopyFallbackChanParity(t *testing.T) {
	srcSlice := buildStreamFixtureTree(t)
	srcChan := buildStreamFixtureTree(t)
	dstSlice := filepath.Join(t.TempDir(), "orchard")
	dstChan := filepath.Join(t.TempDir(), "orchard")

	sliceFiles, sliceBytes, err := executeMoveCopyPhase(context.Background(), nil, MustPaths(srcSlice), MustPath(dstSlice), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("executeMoveCopyPhase error = %v", err)
	}

	planCh := make(chan PlanItem, 8)
	go func() {
		_ = BuildPlanStreamCtx(context.Background(), MustPaths(srcChan), MustPath(dstChan), true, PlanBuildOptions{}, planCh)
	}()
	copyFiles, copyBytes, transferred, err := executeCopyIter(context.Background(), planIterChan(context.Background(), planCh, nil), MustPath(dstChan), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("executeCopyIter error = %v", err)
	}
	chanFiles, chanBytes, err := finishMoveCopyPhase(context.Background(), MustPaths(srcChan), transferred, copyFiles, copyBytes)
	if err != nil {
		t.Fatalf("finishMoveCopyPhase error = %v", err)
	}

	if chanFiles != sliceFiles || chanBytes != sliceBytes {
		t.Fatalf("chan(files=%d,bytes=%d) != slice(files=%d,bytes=%d)", chanFiles, chanBytes, sliceFiles, sliceBytes)
	}
	if _, statErr := os.Stat(srcChan); !os.IsNotExist(statErr) {
		t.Fatalf("move source should be removed: statErr = %v", statErr)
	}
	if got := readFileContent(t, filepath.Join(dstChan, "harbor.txt")); got != "harbor-content" {
		t.Fatalf("harbor.txt content = %q", got)
	}
}
