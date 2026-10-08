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
