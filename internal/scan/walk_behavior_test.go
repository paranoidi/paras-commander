package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/gitignore"
)

func collectWalk(t *testing.T, root string, opts WalkOptions) []Entry {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s := startRootWalk(ctx, root, opts)
	defer s.Close()

	var all []Entry
	for {
		select {
		case batch, ok := <-s.Results():
			if !ok {
				return all
			}
			all = append(all, batch...)
		case <-s.Done():
			for batch := range s.Results() {
				all = append(all, batch...)
			}
			return all
		}
	}
}

func TestWalkIndexesTree(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "pkg"))
	mustWrite(t, filepath.Join(root, "pkg", "a.go"), "x")
	mustWrite(t, filepath.Join(root, "README"), "y")

	entries := collectWalk(t, root, WalkOptions{IncludeHidden: false})
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3 (pkg, pkg/a.go, README)", len(entries))
	}
	byRel := map[string]Entry{}
	for _, e := range entries {
		byRel[e.RelLine] = e
	}
	if !byRel["pkg"].IsDir {
		t.Fatal("pkg should be directory")
	}
	if byRel["pkg/a.go"].IsDir {
		t.Fatal("pkg/a.go should be file")
	}
}

func TestWalkSkipsHidden(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".hidden"), "x")
	mustWrite(t, filepath.Join(root, "visible"), "y")
	mustMkdir(t, filepath.Join(root, ".dotdir"))
	mustWrite(t, filepath.Join(root, ".dotdir", "inner.txt"), "z")

	s := startRootWalk(context.Background(), root, WalkOptions{})
	defer s.Close()
	var entries []Entry
	for batch := range s.Results() {
		entries = append(entries, batch...)
	}
	<-s.Done()

	for _, e := range entries {
		if strings.HasPrefix(filepath.Base(e.RelLine), ".") {
			t.Fatalf("unexpected hidden entry %q", e.RelLine)
		}
	}
	dirs := s.SkippedHiddenDirs()
	if len(dirs) != 1 || filepath.Base(dirs[0]) != ".dotdir" {
		t.Fatalf("SkippedHiddenDirs = %v, want [.dotdir]", dirs)
	}
	files := s.SkippedHiddenFiles()
	if len(files) != 1 || filepath.Base(files[0]) != ".hidden" {
		t.Fatalf("SkippedHiddenFiles = %v, want [.hidden]", files)
	}
}

func TestWalkIncludeHiddenIndexesDots(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, ".git", "config"), "x")
	mustWrite(t, filepath.Join(root, ".gitignore"), "y")
	mustWrite(t, filepath.Join(root, "readme"), "z")

	entries := collectWalk(t, root, WalkOptions{IncludeHidden: true})
	byRel := map[string]bool{}
	for _, e := range entries {
		byRel[e.RelLine] = true
	}
	for _, want := range []string{".git", ".git/config", ".gitignore", "readme"} {
		if !byRel[want] {
			t.Fatalf("missing %q in %+v", want, byRel)
		}
	}
}

func TestWalkSkipsGitignoredDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, ".gitignore"), "skipdir/\n")
	mustMkdir(t, filepath.Join(root, "skipdir"))
	mustWrite(t, filepath.Join(root, "skipdir", "inner.txt"), "x")
	mustWrite(t, filepath.Join(root, "outer.txt"), "y")

	cache := gitignore.NewCache()
	entries := collectWalk(t, root, WalkOptions{Gitignore: cache})
	for _, e := range entries {
		if strings.Contains(e.RelLine, "skipdir") {
			t.Fatalf("unexpected gitignored path in index: %q", e.RelLine)
		}
	}
}

func TestWalkShouldSkipDir(t *testing.T) {
	root := t.TempDir()
	skipDir := filepath.Join(root, "skipme")
	mustMkdir(t, skipDir)
	mustWrite(t, filepath.Join(skipDir, "inner.txt"), "x")
	mustWrite(t, filepath.Join(root, "outer.txt"), "y")

	skip := func(abs string) bool {
		return filepath.Clean(abs) == filepath.Clean(skipDir)
	}
	entries := collectWalk(t, root, WalkOptions{ShouldSkipDir: skip})
	for _, e := range entries {
		if strings.Contains(e.RelLine, "inner") {
			t.Fatalf("should not index inside skipme: %q", e.RelLine)
		}
	}
	foundSkip := false
	for _, e := range entries {
		if e.RelLine == "skipme" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatal("skipme directory itself should appear in results")
	}
}

func TestWalkConcurrentFlushStress(t *testing.T) {
	root := t.TempDir()
	for i := range 300 {
		name := filepath.Join(root, fmt.Sprintf("dir%d", i))
		mustMkdir(t, name)
		for j := range 8 {
			mustWrite(t, filepath.Join(name, fmt.Sprintf("f%d.txt", j)), "x")
		}
	}
	entries := collectWalk(t, root, WalkOptions{})
	if len(entries) < 300*8 {
		t.Fatalf("expected many entries, got %d", len(entries))
	}
}

func TestWalkCancel(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "a", "b", "c"))
	ctx := context.Background()
	s := startRootWalk(ctx, root, WalkOptions{})
	s.Close()
	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("session did not finish after Close")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkDoesNotStatSizeDuringIndex(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "sized.txt")
	content := []byte("hello-find-size")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sess := startRootWalk(context.Background(), root, WalkOptions{})
	defer sess.Close()
	var got int64
	for batch := range sess.Results() {
		for _, e := range batch {
			if e.RelLine == "sized.txt" {
				got = e.Size
			}
		}
	}
	<-sess.Done()
	if got != 0 {
		t.Fatalf("Size during walk = %d, want 0 (lazy stat on mark)", got)
	}
}
