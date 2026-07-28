package find

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

func collectSession(t *testing.T, root string, opts Options) []Entry {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s := Start(ctx, root, opts)
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

func TestSessionIndexesTree(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "pkg"))
	mustWrite(t, filepath.Join(root, "pkg", "a.go"), "x")
	mustWrite(t, filepath.Join(root, "README"), "y")

	entries := collectSession(t, root, Options{IncludeHidden: false})
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

func TestSessionSkipsHidden(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".hidden"), "x")
	mustWrite(t, filepath.Join(root, "visible"), "y")
	mustMkdir(t, filepath.Join(root, ".dotdir"))
	mustWrite(t, filepath.Join(root, ".dotdir", "inner.txt"), "z")

	s := Start(context.Background(), root, Options{})
	defer s.Close()
	var entries []Entry
	for batch := range s.Results() {
		entries = append(entries, batch...)
	}
	<-s.Done()

	for _, e := range entries {
		if strings.HasPrefix(filepath.Base(e.Path), ".") {
			t.Fatalf("unexpected hidden entry %q", e.Path)
		}
	}
	dirs := s.SkippedHiddenDirs()
	if len(dirs) != 1 || filepath.Base(dirs[0]) != ".dotdir" {
		t.Fatalf("SkippedHiddenDirs = %v, want [.dotdir]", dirs)
	}
	files := s.SkippedHiddenFiles()
	if len(files) != 1 || filepath.Base(files[0].Path) != ".hidden" {
		t.Fatalf("SkippedHiddenFiles = %v, want [.hidden]", files)
	}
}

func TestSessionIncludeHiddenIndexesDots(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, ".git", "config"), "x")
	mustWrite(t, filepath.Join(root, ".gitignore"), "y")
	mustWrite(t, filepath.Join(root, "readme"), "z")

	entries := collectSession(t, root, Options{IncludeHidden: true})
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

func TestSessionSkipsGitignoredDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, ".gitignore"), "skipdir/\n")
	mustMkdir(t, filepath.Join(root, "skipdir"))
	mustWrite(t, filepath.Join(root, "skipdir", "inner.txt"), "x")
	mustWrite(t, filepath.Join(root, "outer.txt"), "y")

	cache := gitignore.NewCache()
	entries := collectSession(t, root, Options{Gitignore: cache})
	for _, e := range entries {
		if strings.Contains(e.RelLine, "skipdir") {
			t.Fatalf("unexpected gitignored path in index: %q", e.RelLine)
		}
	}
}

func TestSessionShouldSkipDir(t *testing.T) {
	root := t.TempDir()
	skipDir := filepath.Join(root, "skipme")
	mustMkdir(t, skipDir)
	mustWrite(t, filepath.Join(skipDir, "inner.txt"), "x")
	mustWrite(t, filepath.Join(root, "outer.txt"), "y")

	skip := func(abs string) bool {
		return filepath.Clean(abs) == filepath.Clean(skipDir)
	}
	entries := collectSession(t, root, Options{ShouldSkipDir: skip})
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

func TestSessionConcurrentFlushStress(t *testing.T) {
	root := t.TempDir()
	for i := range 300 {
		name := filepath.Join(root, fmt.Sprintf("dir%d", i))
		mustMkdir(t, name)
		for j := range 8 {
			mustWrite(t, filepath.Join(name, fmt.Sprintf("f%d.txt", j)), "x")
		}
	}
	entries := collectSession(t, root, Options{})
	if len(entries) < 300*8 {
		t.Fatalf("expected many entries, got %d", len(entries))
	}
}

func TestSessionCancel(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "a", "b", "c"))
	ctx := context.Background()
	s := Start(ctx, root, Options{})
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
