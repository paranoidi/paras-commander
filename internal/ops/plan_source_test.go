package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestExecuteCopyFromDispatchesByPlanSource checks ExecuteCopyFrom picks the Execute* variant
// matching src (Chan / Slice / zero-value-builds-fresh) and that all three produce the same
// transfer result for the same source tree.
func TestExecuteCopyFromDispatchesByPlanSource(t *testing.T) {
	runCopy := func(t *testing.T, build func(srcRoot, dst string) PlanSource) (int, int64) {
		t.Helper()
		srcRoot := buildStreamFixtureTree(t)
		dst := filepath.Join(t.TempDir(), "orchard")
		src := build(srcRoot, dst)
		files, bytes, err := ExecuteCopyFrom(context.Background(), src, MustPaths(srcRoot), MustPath(dst), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
		if err != nil {
			t.Fatalf("ExecuteCopyFrom error = %v", err)
		}
		return files, bytes
	}

	wantFiles, wantBytes := runCopy(t, func(string, string) PlanSource {
		return PlanSource{} // zero value: ExecuteCopy builds the plan itself
	})
	if wantFiles == 0 {
		t.Fatalf("baseline doneFiles = 0, fixture tree produced nothing")
	}

	sliceFiles, sliceBytes := runCopy(t, func(srcRoot, dst string) PlanSource {
		plan, err := BuildPlanCtx(context.Background(), MustPaths(srcRoot), MustPath(dst), true, PlanBuildOptions{})
		if err != nil {
			t.Fatalf("BuildPlanCtx: %v", err)
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatalf("mkdir dst: %v", err)
		}
		return PlanSource{Slice: plan}
	})
	if sliceFiles != wantFiles || sliceBytes != wantBytes {
		t.Fatalf("Slice dispatch = (%d, %d), want (%d, %d)", sliceFiles, sliceBytes, wantFiles, wantBytes)
	}

	chanFiles, chanBytes := runCopy(t, func(srcRoot, dst string) PlanSource {
		ch := make(chan PlanItem, 8)
		go func() {
			if err := BuildPlanStreamCtx(context.Background(), MustPaths(srcRoot), MustPath(dst), true, PlanBuildOptions{}, ch); err != nil {
				t.Errorf("BuildPlanStreamCtx: %v", err)
			}
		}()
		return PlanSource{Chan: ch}
	})
	if chanFiles != wantFiles || chanBytes != wantBytes {
		t.Fatalf("Chan dispatch = (%d, %d), want (%d, %d)", chanFiles, chanBytes, wantFiles, wantBytes)
	}
}

// TestExecuteMoveFromDispatchesByPlanSource is the ExecuteCopyFrom test's move counterpart. The
// source tree fits on one filesystem so RenameFastPath succeeds and the copy-fallback phase that
// actually consumes the plan never runs (see TestMoveCopyFallbackChanParity in
// execute_chan_test.go, which exercises that phase's machinery directly since triggering a real
// cross-device fallback needs a second filesystem this environment doesn't have). What this test
// verifies instead is that ExecuteMoveFrom routes to the matching Execute* variant: only the
// Slice case (ExecuteMoveWithPlan) forwards its plan into the rename phase's counting, so it's
// the only one with a populated doneBytes (see ExecuteMoveWithPlanChan/ExecuteMove, both of which
// pass a nil plan into executeMoveRenamePhase same as TestExecuteMoveWithPlanChanRenameFastPathDrainsChannel
// documents) — doneFiles is what all three must agree on.
func TestExecuteMoveFromDispatchesByPlanSource(t *testing.T) {
	runMove := func(t *testing.T, build func(srcRoot, dst string) PlanSource) (int, int64) {
		t.Helper()
		srcRoot := buildStreamFixtureTree(t)
		dst := filepath.Join(t.TempDir(), "orchard")
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatalf("mkdir dst: %v", err)
		}
		src := build(srcRoot, dst)
		files, bytes, err := ExecuteMoveFrom(context.Background(), src, MustPaths(srcRoot), MustPath(dst), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
		if err != nil {
			t.Fatalf("ExecuteMoveFrom error = %v", err)
		}
		return files, bytes
	}

	wantFiles, _ := runMove(t, func(string, string) PlanSource {
		return PlanSource{}
	})
	if wantFiles == 0 {
		t.Fatalf("baseline doneFiles = 0, fixture tree produced nothing")
	}

	sliceFiles, sliceBytes := runMove(t, func(srcRoot, dst string) PlanSource {
		plan, err := BuildPlanCtx(context.Background(), MustPaths(srcRoot), MustPath(dst), true, PlanBuildOptions{})
		if err != nil {
			t.Fatalf("BuildPlanCtx: %v", err)
		}
		return PlanSource{Slice: plan}
	})
	if sliceFiles != wantFiles {
		t.Fatalf("Slice dispatch doneFiles = %d, want %d", sliceFiles, wantFiles)
	}
	if sliceBytes == 0 {
		t.Fatalf("Slice dispatch doneBytes = 0, want > 0 (plan-aware counting branch)")
	}

	chanFiles, _ := runMove(t, func(srcRoot, dst string) PlanSource {
		ch := make(chan PlanItem, 8)
		go func() {
			// The rename fast path can move srcRoot out from under this walk before it starts;
			// same as TestExecuteMoveWithPlanChanRenameFastPathDrainsChannel, the producer's own
			// error is not the thing under test here, so it's discarded rather than asserted on.
			_ = BuildPlanStreamCtx(context.Background(), MustPaths(srcRoot), MustPath(dst), true, PlanBuildOptions{}, ch)
		}()
		return PlanSource{Chan: ch}
	})
	if chanFiles != wantFiles {
		t.Fatalf("Chan dispatch doneFiles = %d, want %d", chanFiles, wantFiles)
	}
}
