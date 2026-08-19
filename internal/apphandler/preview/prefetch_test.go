package preview

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// configCountingHost wraps fakeHost to count Config() calls. SchedulePrefetchFromActivePanel
// always calls Config() once via ensurePrefetch and once via prefetchSurfaceActive; a rebuild
// (queue not skipped) reads it a third time for PrefetchWindow. Counting calls per invocation
// distinguishes the short-circuit path (2 calls) from a rebuild (3 calls) without needing to
// peek into the unexported prefetch.Engine queue.
type configCountingHost struct {
	*fakeHost
	configCalls int
}

func (h *configCountingHost) Config() config.Config {
	h.configCalls++
	return h.fakeHost.Config()
}

func TestSchedulePrefetchFromActivePanel_SkipsRebuildWhenUnchanged(t *testing.T) {
	handler, fh := newTestHandler(t, 80, 24)
	ch := &configCountingHost{fakeHost: fh}
	handler.host = ch
	t.Cleanup(handler.stopPrefetch)

	root := t.TempDir()
	handler.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "a.jpg", Path: filepath.Join(root, "a.jpg"), Type: localfs.EntryFile},
			{Name: "b.png", Path: filepath.Join(root, "b.png"), Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	handler.model.ActivePanel = ui.PrimaryPanel
	handler.model.QuickViewEnabled = true

	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 3 {
		t.Fatalf("first call (path/cursor changed): Config() called %d times, want 3 (rebuild)", ch.configCalls)
	}

	ch.configCalls = 0
	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 2 {
		t.Fatalf("second call (nothing changed): Config() called %d times, want 2 (short-circuit)", ch.configCalls)
	}

	ch.configCalls = 0
	handler.model.Primary.Cursor = 1
	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 3 {
		t.Fatalf("cursor moved: Config() called %d times, want 3 (rebuild)", ch.configCalls)
	}
}

// TestSchedulePrefetchFromActivePanel_RebuildsWhenEntryCountChanges covers a listing change that
// moves neither the cursor nor the path — e.g. M-. revealing hidden/gitignored files. Without
// comparing VisibleEntryCount too, the skip-rebuild check would treat this as "nothing changed"
// and never (re)schedule the near-caret window for the newly-visible entries.
func TestSchedulePrefetchFromActivePanel_RebuildsWhenEntryCountChanges(t *testing.T) {
	handler, fh := newTestHandler(t, 80, 24)
	ch := &configCountingHost{fakeHost: fh}
	handler.host = ch
	t.Cleanup(handler.stopPrefetch)

	root := t.TempDir()
	handler.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "a.jpg", Path: filepath.Join(root, "a.jpg"), Type: localfs.EntryFile},
			{Name: "b.png", Path: filepath.Join(root, "b.png"), Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	handler.model.ActivePanel = ui.PrimaryPanel
	handler.model.QuickViewEnabled = true

	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 3 {
		t.Fatalf("first call: Config() called %d times, want 3 (rebuild)", ch.configCalls)
	}

	// Same path, same cursor — only the listing grew (e.g. a hidden file became visible).
	ch.configCalls = 0
	handler.model.Primary.Entries = append(handler.model.Primary.Entries,
		localfs.Entry{Name: ".hidden.jpg", Path: filepath.Join(root, ".hidden.jpg"), Type: localfs.EntryFile})
	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 3 {
		t.Fatalf("entry count changed: Config() called %d times, want 3 (rebuild, not skipped)", ch.configCalls)
	}
}

func TestSchedulePrefetchFromActivePanel_HeldDuringNavCoalesce(t *testing.T) {
	handler, fh := newTestHandler(t, 80, 24)
	ch := &configCountingHost{fakeHost: fh}
	handler.host = ch
	t.Cleanup(handler.stopPrefetch)

	root := t.TempDir()
	handler.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "a.jpg", Path: filepath.Join(root, "a.jpg"), Type: localfs.EntryFile},
			{Name: "b.png", Path: filepath.Join(root, "b.png"), Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	handler.model.ActivePanel = ui.PrimaryPanel
	handler.model.QuickViewEnabled = true

	handler.SchedulePrefetchFromActivePanel()
	ch.configCalls = 0

	// Simulate rapid key-repeat scrolling: the caret moves but the quick-view nav-coalesce
	// debounce hasn't fired yet. No rebuild — and no engine.Config() read past ensurePrefetch —
	// should happen until the debounce clears the flag.
	handler.quickViewNavSkipReconcile.Store(true)
	handler.model.Primary.Cursor = 1
	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 1 {
		t.Fatalf("during nav coalesce: Config() called %d times, want 1 (held, no rebuild)", ch.configCalls)
	}
	if handler.prefetchLastCursor != 0 {
		t.Fatalf("during nav coalesce: prefetchLastCursor = %d, want unchanged at 0", handler.prefetchLastCursor)
	}

	// Debounce fires: flag clears, next call rebuilds for the settled position.
	ch.configCalls = 0
	handler.quickViewNavSkipReconcile.Store(false)
	handler.SchedulePrefetchFromActivePanel()
	if ch.configCalls != 3 {
		t.Fatalf("after nav coalesce clears: Config() called %d times, want 3 (rebuild)", ch.configCalls)
	}
	if handler.prefetchLastCursor != 1 {
		t.Fatalf("after nav coalesce clears: prefetchLastCursor = %d, want 1", handler.prefetchLastCursor)
	}
}

// TestSyncPrefetchLoadingMarksRefreshesWarmMap guards against the bug where a file that finished
// warming in the background between cursor moves kept showing a stale (cold) icon tint until the
// next actual cursor movement forced a full SchedulePrefetchFromActivePanel rebuild.
// syncPrefetchLoadingMarks (the Cache's OnChange callback) must refresh PreviewPrefetchWarm itself,
// from the cached prefetchLastEntries/prefetchLastBox snapshot, with no cursor movement involved.
func TestSyncPrefetchLoadingMarksRefreshesWarmMap(t *testing.T) {
	handler, fh := newTestHandler(t, 80, 24)
	handler.host = fh
	t.Cleanup(handler.stopPrefetch)

	root := t.TempDir()
	aPath := filepath.Join(root, "a.jpg")
	if err := os.WriteFile(aPath, []byte("fake-jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(aPath)
	if err != nil {
		t.Fatal(err)
	}
	mtime, size := fi.ModTime().UnixNano(), fi.Size()

	handler.model.Primary = panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "a.jpg", Path: aPath, Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	handler.model.ActivePanel = ui.PrimaryPanel
	handler.model.QuickViewEnabled = true

	handler.SchedulePrefetchFromActivePanel()
	if _, warm := handler.model.PreviewPrefetchWarm[aPath]; warm {
		t.Fatalf("PreviewPrefetchWarm[%q] = true before any decode; want false", aPath)
	}

	// Simulate a background decode finishing (Cache's OnChange fires this, from a worker
	// goroutine) without any cursor movement in between. maxEdge must match what ensurePrefetch
	// configured the engine with, or the cache key won't line up with what IsEntryWarm looks up.
	cfg := fh.Config().Preview
	protocol := previewrun.ResolveImageProtocol(cfg, os.Getenv)
	maxEdge := previewrun.EffectiveStillMaxEdge(cfg, protocol, os.Getenv("TMUX") != "")
	if _, _, err := handler.prefetch.Cache().LoadStill(context.Background(), aPath, mtime, size, maxEdge,
		func(context.Context) ([]byte, string, error) { return []byte("still"), "", nil }); err != nil {
		t.Fatal(err)
	}
	handler.syncPrefetchLoadingMarks()

	// syncPrefetchLoadingMarks debounces the warm-map rebuild (prefetchWarmMapDebounceDelay), so
	// poll rather than asserting immediately.
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		handler.mu.RLock()
		_, warm := handler.model.PreviewPrefetchWarm[aPath]
		handler.mu.RUnlock()
		if warm {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("PreviewPrefetchWarm[%q] = false after background warm + syncPrefetchLoadingMarks; want true (no cursor move needed)", aPath)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestEffectivePrefetchWorkers(t *testing.T) {
	cases := []struct {
		configured, gomaxprocs, want int
	}{
		{configured: 4, gomaxprocs: 8, want: 4}, // plenty of cores: configured value wins
		{configured: 4, gomaxprocs: 4, want: 3}, // leaves one core free
		{configured: 4, gomaxprocs: 2, want: 1}, // low core count: capped hard
		{configured: 4, gomaxprocs: 1, want: 1}, // single core: floor of 1, never 0
		{configured: 1, gomaxprocs: 8, want: 1}, // configured value still the upper bound
	}
	for _, c := range cases {
		if got := effectivePrefetchWorkers(c.configured, c.gomaxprocs); got != c.want {
			t.Errorf("effectivePrefetchWorkers(%d, %d) = %d, want %d", c.configured, c.gomaxprocs, got, c.want)
		}
	}
}
