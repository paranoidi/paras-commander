package scan

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fswalk"
)

func makeDepthTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "maple", "birch", "cedar", "willow"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func relLines(entries []Entry) map[string]bool {
	out := map[string]bool{}
	for _, e := range entries {
		out[e.RelLine] = true
	}
	return out
}

func TestWalkMaxDepthStopsAtLevel(t *testing.T) {
	t.Parallel()
	root := makeDepthTree(t)
	w := startRootWalk(t.Context(), root, WalkOptions{MaxDepth: 2})
	var entries []Entry
	for batch := range w.Results() {
		entries = append(entries, batch...)
	}
	<-w.Done()
	got := relLines(entries)
	if len(got) != 2 || !got["maple"] || !got["maple/birch"] {
		t.Fatalf("entries = %v, want maple and maple/birch only", got)
	}
}

func TestCoordinatorSetMaxDepthPrunes(t *testing.T) {
	t.Parallel()
	root := makeDepthTree(t)
	var mu sync.Mutex
	var finished bool
	var replaced []Entry
	var sawReplaced bool
	coord := NewCoordinator(fswalk.Params{InitialWorkers: 1, MaxWorkers: 2, AdaptIntervalMS: 500}, func(ev Event) {
		mu.Lock()
		defer mu.Unlock()
		if ev.IndexFinished {
			finished = true
		}
		if ev.IndexReplaced {
			sawReplaced = true
			replaced = ev.ReplacedEntries
		}
	})
	coord.Start(StartOpts{Gen: 1, DisplayRoot: root, Roots: []string{root}})
	wait := func(cond func() bool) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			mu.Lock()
			ok := cond()
			mu.Unlock()
			if ok {
				return
			}
		}
		t.Fatal("timed out")
	}
	wait(func() bool { return finished })
	coord.SetMaxDepth(1)
	wait(func() bool { return sawReplaced })
	mu.Lock()
	defer mu.Unlock()
	if got := relLines(replaced); len(got) != 1 || !got["maple"] {
		t.Fatalf("replaced = %v, want maple only", got)
	}
}
