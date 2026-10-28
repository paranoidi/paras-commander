package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fswalk"
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

func TestWalkDirsOnlyLevelOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, d := range []string{"maple/oak/pine", "birch/cedar", "elm"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "birch", "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := startRootWalk(t.Context(), root, WalkOptions{DirsOnly: true})
	var got []string
	for batch := range w.Results() {
		for _, e := range batch {
			if !e.IsDir {
				t.Fatalf("file emitted: %q", e.RelLine)
			}
			got = append(got, e.RelLine)
		}
	}
	<-w.Done()
	prev := 0
	for _, rel := range got {
		depth := strings.Count(rel, "/") + 1
		if depth < prev {
			t.Fatalf("depth %d after %d in %v", depth, prev, got)
		}
		prev = depth
	}
	if len(got) != 6 {
		t.Fatalf("got %v, want 6 dirs", got)
	}
}

func TestWalkIndexesFilesLevelOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "harbor", "lantern"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"harbor/lantern/quilt.txt", "harbor/meadow.txt", "zephyr.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := startRootWalk(t.Context(), root, WalkOptions{})
	var got []string
	for batch := range w.Results() {
		for _, e := range batch {
			got = append(got, e.RelLine)
		}
	}
	<-w.Done()
	want := []string{"harbor", "zephyr.txt", "harbor/lantern", "harbor/meadow.txt", "harbor/lantern/quilt.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWalkProgressReportsLastLevel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "harbor", "lantern", "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := startRootWalk(t.Context(), root, WalkOptions{DirsOnly: true})
	for range w.Results() {
	}
	<-w.Done()
	// Levels read: [root] depth 1, [harbor] 2, [lantern] 3, [meadow] 4. The
	// leaf dir is still read (its frontier is empty), so the walk ends at depth 4.
	depth, done, total := w.Progress()
	if depth != 4 || done != 1 || total != 1 {
		t.Fatalf("Progress = %d,%d,%d, want 4,1,1", depth, done, total)
	}
}

func TestWalkConcurrentMatchesSequentialOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	words := []string{"anchor", "bramble", "cobalt", "dune", "ember", "fjord", "glacier", "heron"}
	for _, a := range words {
		for _, b := range words[:4] {
			dir := filepath.Join(root, a, b)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "willow.txt"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	collect := func(opts WalkOptions) []string {
		w := startRootWalk(t.Context(), root, opts)
		defer w.Close()
		var got []string
		for batch := range w.Results() {
			for _, e := range batch {
				got = append(got, e.RelLine)
			}
		}
		return got
	}
	want := collect(WalkOptions{})
	got := collect(WalkOptions{Walk: fswalk.Params{InitialWorkers: 4, MaxWorkers: 8, AdaptIntervalMS: 1}})
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("concurrent walk differs:\n got %v\nwant %v", got, want)
	}
}
