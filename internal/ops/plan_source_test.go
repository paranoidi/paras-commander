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
