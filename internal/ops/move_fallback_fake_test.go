package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// refuseRenameFrom makes osRename fail with EXDEV for sources under any prefix.
// Tests using it must not call t.Parallel.
func refuseRenameFrom(t *testing.T, prefixes ...string) {
	t.Helper()
	orig := osRename
	osRename = func(oldpath, newpath string) error {
		for _, p := range prefixes {
			if oldpath == p || strings.HasPrefix(oldpath, p+string(filepath.Separator)) {
				return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EXDEV}
			}
		}
		return orig(oldpath, newpath)
	}
	t.Cleanup(func() { osRename = orig })
}

func TestMoveFallbackMixedSourcesFake(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	copied, renamed := filepath.Join(base, "lantern"), filepath.Join(base, "meadow")
	writeTree(t, copied, map[string]string{"quill.txt": "q", "ember/raven.txt": "r"})
	writeTree(t, renamed, map[string]string{"tulip.txt": "t"})
	refuseRenameFrom(t, copied)

	var last int
	progress := func(_, _ string, n int, _ int64) { last = n }
	done, _, err := ExecuteMove(context.Background(), MustPaths(copied, renamed), MustPath(dst), Options{CopyBufferKiB: 4, FlatDestNames: true}, ProgressEmitThrottle{}, progress, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if done != 5 || last != done { // 1 renamed + 4 copied entries (2 dirs, 2 files)
		t.Fatalf("done = %d, last = %d; want 5", done, last)
	}
	for p, want := range map[string]string{"lantern/quill.txt": "q", "lantern/ember/raven.txt": "r", "meadow/tulip.txt": "t"} {
		if got := readFileContent(t, filepath.Join(dst, p)); got != want {
			t.Fatalf("%s = %q", p, got)
		}
	}
	if exists(copied) || exists(renamed) {
		t.Fatal("sources should be gone")
	}
}

func TestMoveMergeChildFallbackFake(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "a")
	writeTree(t, src, map[string]string{"plain.txt": "p", "sub/acorn.txt": "s"})
	writeTree(t, dst, map[string]string{"a/keep.txt": "k"})
	refuseRenameFrom(t, filepath.Join(src, "sub"))

	var calls []string
	runMove(t, src, dst, countingResolver(true, &calls))
	if len(calls) != 0 {
		t.Fatalf("resolver called: %v", calls)
	}
	for p, want := range map[string]string{"a/keep.txt": "k", "a/plain.txt": "p", "a/sub/acorn.txt": "s"} {
		if got := readFileContent(t, filepath.Join(dst, p)); got != want {
			t.Fatalf("%s = %q", p, got)
		}
	}
	if exists(src) {
		t.Fatal("source dir should be gone")
	}
}

func TestFlattenFallbackFake(t *testing.T) {
	base := t.TempDir()
	root, dest := filepath.Join(base, "forest"), filepath.Join(base, "clearing")
	writeTestFile(t, filepath.Join(root, "birch.txt"), "1")
	writeTestFile(t, filepath.Join(root, "moss", "fern.txt"), "2")
	writeTestFile(t, filepath.Join(root, "moss", "deep", "willow.txt"), "3")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	refuseRenameFrom(t, filepath.Join(root, "moss"))

	if _, err := runFlatten(t, root, dest, true, true, nil); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"birch.txt": "1", "fern.txt": "2", "willow.txt": "3"} {
		if got := readFileContent(t, filepath.Join(dest, name)); got != want {
			t.Fatalf("%s = %q", name, got)
		}
	}
	if exists(root) {
		t.Fatal("root should be removed")
	}
}

// TestMoveCopyPhaseTotalsMatchProgressFake moves a directory that merges into an existing one
// with every child copied (cross device) plus a source that renames. The copy phase's count plus
// the settled units must equal the final done totals, and the source-removal hook must be off
// whenever bytes are being copied.
func TestMoveCopyPhaseTotalsMatchProgressFake(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	merged, renamed := filepath.Join(base, "harvest"), filepath.Join(base, "orchard")
	writeTree(t, merged, map[string]string{"barley.bin": "barley-grain", "rye.bin": "rye", "silo/oats.bin": "oats-sack"})
	writeTree(t, renamed, map[string]string{"plum.txt": "p"})
	writeTree(t, dst, map[string]string{"harvest/keep.txt": "k"})
	refuseRenameFrom(t, merged)

	var phase *MoveCopyPhase
	removing, removals := false, 0
	var lastBytes int64
	progress := func(_, _ string, _ int, b int64) {
		if removing && b > lastBytes {
			t.Fatalf("bytes copied (%d -> %d) while sources were being removed", lastBytes, b)
		}
		lastBytes = b
	}
	opts := Options{
		CopyBufferKiB:   4,
		SourceSizeHints: []int64{-1, 100},
		OnCopyPhase:     func(p MoveCopyPhase) { phase = &p },
		OnRemoveSources: func(on bool) {
			if on == removing {
				t.Fatalf("OnRemoveSources(%v) twice in a row", on)
			}
			removing = on
			if on {
				removals++
			}
		},
	}
	done, doneBytes, err := ExecuteMove(context.Background(), MustPaths(merged, renamed), MustPath(dst), opts, ProgressEmitThrottle{}, progress, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if removing || removals != 3 { // one per copied child: barley.bin, rye.bin, silo
		t.Fatalf("removing = %v, removals = %d; want false, 3", removing, removals)
	}
	if phase == nil || phase.Count == nil {
		t.Fatal("OnCopyPhase not called with a count")
	}
	if phase.Items != 2 || phase.Bytes != 100 { // orchard renamed (+its hint), harvest merged
		t.Fatalf("phase = %d items, %d bytes; want 2, 100", phase.Items, phase.Bytes)
	}
	// The sources are gone now, so count a copy of the same layout with the same tasks.
	writeTree(t, merged, map[string]string{"barley.bin": "barley-grain", "rye.bin": "rye", "silo/oats.bin": "oats-sack"})
	var items int
	var bytes int64
	if err := phase.Count(context.Background(), PlanBuildOptions{}, func(it PlanItem) {
		items++
		if !it.IsDir {
			bytes += it.FileSize
		}
	}); err != nil {
		t.Fatalf("Count: %v", err)
	}
	if phase.Items+items != done || phase.Bytes+bytes != doneBytes {
		t.Fatalf("totals %d items / %d bytes, done %d / %d", phase.Items+items, phase.Bytes+bytes, done, doneBytes)
	}
}

// TestMoveRenameOnlyHasNoCount checks that a move with nothing to copy settles its totals in the
// rename phase.
func TestMoveRenameOnlyHasNoCount(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "meadow")
	writeTree(t, src, map[string]string{"tulip.txt": "t"})
	var phase *MoveCopyPhase
	opts := Options{CopyBufferKiB: 4, OnCopyPhase: func(p MoveCopyPhase) { phase = &p }}
	if _, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dst), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if phase == nil || phase.Count != nil || phase.Items != 1 {
		t.Fatalf("phase = %+v; want 1 item and no count", phase)
	}
}
