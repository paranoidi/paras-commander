package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/gitignore"
)

func TestWalkRecordsGitignoreSkipsForHiddenReplay(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("skipdir/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skipdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skipdir", "inner.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outer.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := startRootWalk(t.Context(), root, WalkOptions{Gitignore: gitignore.NewCache()})
	var entries []Entry
	for batch := range w.Results() {
		entries = append(entries, batch...)
	}
	<-w.Done()

	for _, e := range entries {
		if strings.Contains(e.RelLine, "skipdir") {
			t.Fatalf("unexpected gitignored path in index: %q", e.RelLine)
		}
	}
	dirs := w.SkippedHiddenDirs()
	found := false
	for _, d := range dirs {
		if filepath.Base(d) == "skipdir" {
			found = true
		}
	}
	if !found {
		t.Fatalf("SkippedHiddenDirs = %v, want it to include skipdir", dirs)
	}
}
