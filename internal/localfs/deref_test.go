package localfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestWalkDirRecursivePlainUnaffectedByDerefRefactor(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "meadow"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "meadow", "otter.txt"), []byte("otter"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "badger.txt"), []byte("badger"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var visited []string
	if err := WalkDirRecursive(root, func(path string, info fs.FileInfo) error {
		visited = append(visited, path)
		return nil
	}); err != nil {
		t.Fatalf("WalkDirRecursive error = %v", err)
	}
	sort.Strings(visited)
	want := []string{
		root,
		filepath.Join(root, "badger.txt"),
		filepath.Join(root, "meadow"),
		filepath.Join(root, "meadow", "otter.txt"),
	}
	sort.Strings(want)
	if len(visited) != len(want) {
		t.Fatalf("visited = %v, want %v", visited, want)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Fatalf("visited = %v, want %v", visited, want)
		}
	}
}

func TestWalkDirRecursiveDerefFollowsSymlinkToDir(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "thicket")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "heron.txt"), []byte("heron"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	visited := map[string]fs.FileInfo{}
	var fallbacks []string
	err := WalkDirRecursiveDeref(linkPath, func(path, reason string) {
		fallbacks = append(fallbacks, path+": "+reason)
	}, func(path string, info fs.FileInfo) error {
		visited[path] = info
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDirRecursiveDeref error = %v", err)
	}
	if len(fallbacks) != 0 {
		t.Fatalf("unexpected fallbacks: %v", fallbacks)
	}
	childPath := filepath.Join(linkPath, "heron.txt")
	childInfo, ok := visited[childPath]
	if !ok {
		t.Fatalf("expected child visited at %q, got %v", childPath, visited)
	}
	if IsSymlink(childInfo) {
		t.Fatalf("child info still reports symlink mode")
	}
	linkInfo, ok := visited[linkPath]
	if !ok {
		t.Fatalf("expected root link visited at %q", linkPath)
	}
	if !linkInfo.IsDir() {
		t.Fatalf("resolved root link info should report IsDir")
	}
}

func TestWalkDirRecursiveDerefFollowsSymlinkToFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	content := []byte("meadowlark")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var gotInfo fs.FileInfo
	err := WalkDirRecursiveDeref(linkPath, func(path, reason string) {
		t.Fatalf("unexpected fallback: %s: %s", path, reason)
	}, func(path string, info fs.FileInfo) error {
		gotInfo = info
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDirRecursiveDeref error = %v", err)
	}
	if gotInfo == nil {
		t.Fatalf("fn was never called")
	}
	if IsSymlink(gotInfo) {
		t.Fatalf("info still reports symlink mode")
	}
	if gotInfo.Size() != int64(len(content)) {
		t.Fatalf("size = %d, want %d", gotInfo.Size(), len(content))
	}
}

func TestWalkDirRecursiveDerefDanglingSymlinkFallsBack(t *testing.T) {
	root := t.TempDir()
	linkPath := filepath.Join(root, "ghost")
	if err := os.Symlink(filepath.Join(root, "does-not-exist"), linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var fallbacks []string
	var visited int
	err := WalkDirRecursiveDeref(linkPath, func(path, reason string) {
		fallbacks = append(fallbacks, path+": "+reason)
	}, func(path string, info fs.FileInfo) error {
		visited++
		if !IsSymlink(info) {
			t.Fatalf("fallback info should still report symlink mode")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDirRecursiveDeref error = %v", err)
	}
	if len(fallbacks) != 1 {
		t.Fatalf("fallbacks = %v, want exactly one", fallbacks)
	}
	if visited != 1 {
		t.Fatalf("visited = %d, want 1", visited)
	}
}

func TestWalkDirRecursiveDerefCycleFallsBack(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "alderbrook")
	dirB := filepath.Join(root, "brackenfen")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("mkdir b: %v", err)
	}
	if err := os.Symlink(dirB, filepath.Join(dirA, "link")); err != nil {
		t.Fatalf("symlink a->b: %v", err)
	}
	if err := os.Symlink(dirA, filepath.Join(dirB, "link")); err != nil {
		t.Fatalf("symlink b->a: %v", err)
	}

	done := make(chan error, 1)
	var fallbacks []string
	go func() {
		done <- WalkDirRecursiveDeref(root, func(path, reason string) {
			fallbacks = append(fallbacks, path+": "+reason)
		}, func(path string, info fs.FileInfo) error {
			return nil
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WalkDirRecursiveDeref error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("WalkDirRecursiveDeref did not terminate (likely infinite recursion)")
	}
	if len(fallbacks) == 0 {
		t.Fatalf("expected at least one cycle fallback, got none")
	}
	for _, f := range fallbacks {
		if !strings.Contains(f, "cycle detected") {
			t.Fatalf("fallback reason = %q, want it to mention a cycle", f)
		}
	}
}
