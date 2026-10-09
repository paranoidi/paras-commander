package scan

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/gitignore"
)

// TestSetIncludeHiddenTogglesFullyIgnoredRootBackOff reproduces the scenario
// where the search root itself is excluded by an ancestor .gitignore rule:
// the initial walk indexes nothing, enabling Include Hidden reveals the
// content, and disabling it again must hide that same content — not leave
// it stuck in the results.
func TestSetIncludeHiddenTogglesFullyIgnoredRootBackOff(t *testing.T) {
	t.Parallel()
	outer := t.TempDir()
	if err := os.Mkdir(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, ".gitignore"), []byte("root_dir/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(outer, "root_dir")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "deep.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var lastCount int
	var indexFinished, replaced atomic.Bool
	coord := NewCoordinator(fswalk.Params{InitialWorkers: 1, MaxWorkers: 2, AdaptIntervalMS: 500}, func(ev Event) {
		if ev.CountUpdate {
			mu.Lock()
			lastCount = ev.Count
			mu.Unlock()
		}
		if ev.IndexFinished {
			indexFinished.Store(true)
		}
		if ev.IndexReplaced {
			replaced.Store(true)
		}
	})

	coord.Start(StartOpts{
		Gen:         1,
		DisplayRoot: root,
		Roots:       []string{root},
		Gitignore:   gitignore.NewCache(),
		Walk:        fswalk.Params{InitialWorkers: 1, MaxWorkers: 2, AdaptIntervalMS: 500},
	})

	waitUntilTrue(t, 3*time.Second, indexFinished.Load)
	mu.Lock()
	initialCount := lastCount
	mu.Unlock()
	if initialCount != 0 {
		t.Fatalf("initial index count = %d, want 0 (root itself is gitignored)", initialCount)
	}

	coord.SetIncludeHidden(true)
	waitUntilTrue(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastCount >= 3 // root_dir, file.txt, sub, sub/deep.txt (at least 3)
	})

	replaced.Store(false)
	coord.SetIncludeHidden(false)
	waitUntilTrue(t, 2*time.Second, replaced.Load)
	mu.Lock()
	finalCount := lastCount
	mu.Unlock()
	if finalCount != 0 {
		t.Fatalf("count after disabling include-hidden = %d, want 0 (gitignored content should be hidden again)", finalCount)
	}

	coord.Cancel()
}

func waitUntilTrue(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

// TestNarrowSelectionWalksSelectedRoots checks that narrowing to a selected
// directory the panel walk never entered (here: gitignored) walks it, drops
// everything outside it, and that widening brings the rest back.
func TestNarrowSelectionWalksSelectedRoots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("lantern/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"meadow/pebble.txt", "lantern/harbor.txt"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var finished atomic.Bool
	walk := fswalk.Params{InitialWorkers: 1, MaxWorkers: 2, AdaptIntervalMS: 500}
	coord := NewCoordinator(walk, func(ev Event) {
		if ev.IndexFinished {
			finished.Store(true)
		}
	})
	defer coord.Cancel()
	has := func(rel string) bool {
		found := false
		coord.activeSession().idx.View(func(entries []Entry, _ int) {
			for _, e := range entries {
				if e.RelLine == rel {
					found = true
				}
			}
		})
		return found
	}

	selected := filepath.Join(root, "lantern")
	coord.Start(StartOpts{
		Gen:            1,
		DisplayRoot:    root,
		Roots:          []string{root},
		Gitignore:      gitignore.NewCache(),
		SelectionRoots: []string{selected},
		Walk:           walk,
	})
	waitUntilTrue(t, 3*time.Second, finished.Load)
	if has("lantern/harbor.txt") || !has("meadow/pebble.txt") {
		t.Fatal("unexpected initial index")
	}

	finished.Store(false)
	coord.NarrowSelection([]string{selected})
	waitUntilTrue(t, 3*time.Second, finished.Load)
	if !has("lantern/harbor.txt") {
		t.Fatal("selected dir not walked after narrowing")
	}
	if has("meadow/pebble.txt") {
		t.Fatal("out-of-scope entry kept after narrowing")
	}

	finished.Store(false)
	coord.Widen()
	waitUntilTrue(t, 3*time.Second, finished.Load)
	if !has("meadow/pebble.txt") {
		t.Fatal("out-of-scope entry not restored after widening")
	}
}
