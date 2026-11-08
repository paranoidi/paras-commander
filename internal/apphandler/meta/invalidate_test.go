package meta

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// cachedColumnFixture is a resolved panel whose rows all came from the cache, so any pending cell
// after a call is a command that call dispatched.
func cachedColumnFixture(t *testing.T) (screen tcell.Screen, h *Handler, p *panel.State, harbor, meadow string) {
	t.Helper()
	screen = tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	dir := t.TempDir()
	harbor = filepath.Join(dir, "harbor.txt")
	meadow = filepath.Join(dir, "meadow.txt")
	p = testPanel(t, dir, []localfs.Entry{
		{Name: "harbor.txt", Path: harbor, Type: localfs.EntryFile},
		{Name: "meadow.txt", Path: meadow, Type: localfs.EntryFile},
	})
	h = &Handler{screen: screen, host: &fakeHost{panels: [2]*panel.State{p}}, model: &ui.Model{}, config: config.Default()}
	h.cache = map[string]map[string]string{"words": {harbor: "3", meadow: "5"}}
	def := metacmds.MetaEntry{Name: "words", File: "true", Cache: true}
	h.runForPanel(0, []metacmds.MetaEntry{def}, []ui.MetaColumnState{{EntryName: "words"}})
	return screen, h, p, harbor, meadow
}

// cachedWord reads the column's cache as the dispatch goroutines do, under cacheMu.
func cachedWord(h *Handler, path string) (string, bool) {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	v, ok := h.cache["words"][path]
	return v, ok
}

func setCachedWord(h *Handler, path, value string) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	h.cache["words"][path] = value
}

func dropEntry(p *panel.State, path string) {
	for i, e := range p.Entries {
		if e.Path == path {
			p.Entries = append(p.Entries[:i], p.Entries[i+1:]...)
			p.ListingEpoch++
			return
		}
	}
}

func TestApplyFSChanges_removedDropsOnlyThatRow(t *testing.T) {
	_, h, p, harbor, meadow := cachedColumnFixture(t)

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Removed, Path: harbor}})
	h.ReconcileForPanel(0)

	col := &h.model.MetaResults[0][0]
	if _, ok := cachedWord(h, harbor); ok {
		t.Fatal("removed path is still cached")
	}
	if v, _ := cachedWord(h, meadow); v != "5" {
		t.Fatalf("sibling cache = %q, want untouched", v)
	}
	// The reload has not landed: the row is still listed and must not run on a deleted file.
	if col.PendingCount != 0 || col.Results[harbor] != "3" {
		t.Fatalf("PendingCount = %d, harbor = %q; want no dispatch and the old cell", col.PendingCount, col.Results[harbor])
	}

	dropEntry(p, harbor)
	h.ReconcileForPanel(0)

	if _, ok := col.Results[harbor]; ok {
		t.Fatal("the cell outlived the row")
	}
	if col.Results[meadow] != "5" || col.PendingCount != 0 {
		t.Fatalf("meadow = %q, PendingCount = %d; want untouched, no dispatch", col.Results[meadow], col.PendingCount)
	}
	if len(h.gone[0]) != 0 {
		t.Fatalf("gone = %v, want empty", h.gone[0])
	}
}

func TestApplyFSChanges_listingAlreadyLostTheRow(t *testing.T) {
	_, h, p, harbor, _ := cachedColumnFixture(t)
	// The job's reload landed and was reconciled before the report arrived.
	dropEntry(p, harbor)
	h.ReconcileForPanel(0)

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Removed, Path: harbor}})
	h.ReconcileForPanel(0)

	if _, ok := h.model.MetaResults[0][0].Results[harbor]; ok {
		t.Fatal("a report after the reload left the cell behind")
	}
}

func TestApplyFSChanges_renameRecomputesOnlyTheNewPath(t *testing.T) {
	_, h, p, harbor, meadow := cachedColumnFixture(t)
	gen := h.runGen[0]
	thistle := filepath.Join(p.PathString(), "thistle.txt")

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Renamed, From: harbor, Path: thistle}})
	p.Entries[0] = localfs.Entry{Name: "thistle.txt", Path: thistle, Type: localfs.EntryFile}
	p.ListingEpoch++
	h.ReconcileForPanel(0)

	col := h.model.MetaResults[0][0]
	if _, ok := cachedWord(h, harbor); ok {
		t.Fatal("old name is still cached")
	}
	if col.PendingCount != 1 || col.Results[thistle] != "*" {
		t.Fatalf("PendingCount = %d, thistle = %q; want only the new path dispatched", col.PendingCount, col.Results[thistle])
	}
	if col.Results[meadow] != "5" {
		t.Fatalf("meadow = %q, want untouched", col.Results[meadow])
	}
	if h.runGen[0] != gen || h.loadPending[0] {
		t.Fatal("a rename restarted the run")
	}
}

func TestApplyFSChanges_createdRecomputesAListedRow(t *testing.T) {
	_, h, _, harbor, meadow := cachedColumnFixture(t)

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Created, Path: harbor}})
	h.ReconcileForPanel(0)

	col := h.model.MetaResults[0][0]
	if col.PendingCount != 1 || col.Results[harbor] != "*" {
		t.Fatalf("PendingCount = %d, harbor = %q; want the overwritten row dispatched", col.PendingCount, col.Results[harbor])
	}
	if col.Results[meadow] != "5" {
		t.Fatalf("meadow = %q, want untouched", col.Results[meadow])
	}
}

func TestApplyFSChanges_pendingRowIsNotCountedTwice(t *testing.T) {
	screen, h, _, harbor, _ := cachedColumnFixture(t)
	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Created, Path: harbor}})
	h.ReconcileForPanel(0)

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Created, Path: harbor}})
	h.ReconcileForPanel(0)

	if got := h.model.MetaResults[0][0].PendingCount; got != 1 {
		t.Fatalf("PendingCount = %d, want 1", got)
	}
	pumpWakes(t, screen, h, 0)
}

func TestApplyFSChanges_goneRowWithPendingCellReleasesItsCount(t *testing.T) {
	_, h, p, harbor, _ := cachedColumnFixture(t)
	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Created, Path: harbor}})
	h.ReconcileForPanel(0)

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Removed, Path: harbor}})
	dropEntry(p, harbor)
	h.ReconcileForPanel(0)

	if got := h.model.MetaResults[0][0].PendingCount; got != 0 {
		t.Fatalf("PendingCount = %d, want 0 after the pending row left", got)
	}
}

func TestApplyFSChanges_directoryCoversItsSubtreeOnly(t *testing.T) {
	_, h, p, _, _ := cachedColumnFixture(t)
	root := p.PathString()
	orchard := filepath.Join(root, "orchard")
	willow := filepath.Join(orchard, "willow.txt")
	lookalike := filepath.Join(root, "orchardist.txt")
	col := &h.model.MetaResults[0][0]
	for _, path := range []string{orchard, willow, lookalike} {
		setCachedWord(h, path, "7")
		col.Results[path] = "7"
	}

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Removed, Path: orchard, IsDir: true}})
	h.ReconcileForPanel(0)

	for _, path := range []string{orchard, willow} {
		if _, ok := cachedWord(h, path); ok {
			t.Fatalf("%s is still cached", path)
		}
		if _, ok := col.Results[path]; ok {
			t.Fatalf("%s kept its cell (not listed)", path)
		}
	}
	if v, _ := cachedWord(h, lookalike); v != "7" || col.Results[lookalike] != "7" {
		t.Fatal("a sibling sharing the name prefix was invalidated")
	}
}

func TestApplyFSChanges_recreatedBeforeTheListingChanged(t *testing.T) {
	_, h, _, harbor, _ := cachedColumnFixture(t)

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Removed, Path: harbor}})
	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Created, Path: harbor}})
	h.ReconcileForPanel(0)

	col := h.model.MetaResults[0][0]
	if len(h.gone[0]) != 0 {
		t.Fatalf("gone = %v, want the recreated path cleared", h.gone[0])
	}
	if col.Results[harbor] != "*" || col.PendingCount != 1 {
		t.Fatalf("harbor = %q, PendingCount = %d; want it recomputed", col.Results[harbor], col.PendingCount)
	}
}

func TestApplyFSChanges_sameBatchLastReportWins(t *testing.T) {
	_, h, _, harbor, _ := cachedColumnFixture(t)

	h.ApplyFSChanges([]fschange.Change{
		{Kind: fschange.Removed, Path: harbor},
		{Kind: fschange.Created, Path: harbor},
	})

	if _, ok := h.gone[0][harbor]; ok {
		t.Fatal("a recreated path was recorded as gone")
	}
}

func TestApplyFSChanges_matchesFileIndexRows(t *testing.T) {
	_, h, _, _, _ := cachedColumnFixture(t)
	indexed := "fileindex:///vol/orchard/willow.txt"
	setCachedWord(h, indexed, "9")

	h.ApplyFSChanges([]fschange.Change{{Kind: fschange.Removed, Path: "/vol/orchard/willow.txt"}})

	if _, ok := cachedWord(h, indexed); ok {
		t.Fatal("a fileindex:// row was not matched by its host path")
	}
}

func TestRefreshProviders_leavesShellColumnsAndCacheAlone(t *testing.T) {
	var calls atomic.Int32
	p := &metacmds.Provider{Name: "lantern.tags", Values: func(_ context.Context, _ string, entries []localfs.Entry) (map[string]string, error) {
		n := calls.Add(1)
		out := map[string]string{}
		for _, e := range entries {
			out[e.Path] = "tag" + string(rune('0'+n))
		}
		return out, nil
	}}
	screen, h, _, entries := providerFixture(t, p)
	path := entries[0].Path
	h.cache = map[string]map[string]string{"words": {}}
	for _, e := range entries {
		h.cache["words"][e.Path] = "3"
	}
	defs := []metacmds.MetaEntry{
		{Name: "lantern.tags", Provider: p},
		{Name: "words", File: "true", Cache: true},
	}
	cols := []ui.MetaColumnState{{EntryName: "lantern.tags"}, {EntryName: "words"}}
	h.runForPanel(0, defs, cols)
	pumpWakes(t, screen, h, 0)
	gen := h.runGen[0]

	h.RefreshProviders(0)

	tags, words := &h.model.MetaResults[0][0], &h.model.MetaResults[0][1]
	if tags.Results[path] != "tag1" || pendingCount(h.model.MetaResults[0]) != 0 {
		t.Fatalf("tags = %q, pending = %d; want the old value and no marker until the wake", tags.Results[path], pendingCount(h.model.MetaResults[0]))
	}
	wake := func() WakePayload {
		for {
			if in, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				if d, ok := in.Data().(WakePayload); ok {
					return d
				}
			}
		}
	}()
	h.HandleWake(wake)

	if tags.Results[path] != "tag2" {
		t.Fatalf("tags = %q, want the refreshed value", tags.Results[path])
	}
	if cached, _ := cachedWord(h, path); words.Results[path] != "3" || cached != "3" {
		t.Fatalf("words = %q, cached = %q; want both untouched", words.Results[path], cached)
	}
	if h.runGen[0] != gen || h.loadPending[0] {
		t.Fatal("a provider refresh restarted the run")
	}
}

func TestRefreshProviders_withoutRunIsNoOp(t *testing.T) {
	var calls atomic.Int32
	p := &metacmds.Provider{Name: "lantern.tags", Values: func(context.Context, string, []localfs.Entry) (map[string]string, error) {
		calls.Add(1)
		return nil, nil
	}}
	_, h, _, _ := providerFixture(t, p)

	h.RefreshProviders(0)

	if h.providerCancel[0] != nil || calls.Load() != 0 {
		t.Fatal("a panel with no run started a provider refresh")
	}
}

func TestProviderWake_doesNotRestoreADroppedRow(t *testing.T) {
	_, h, _, harbor, meadow := cachedColumnFixture(t)
	col := &h.model.MetaResults[0][0]
	delete(col.Results, harbor)

	h.HandleWake(WakePayload{EntryName: "words", Gen: h.runGen[0], Values: map[string]string{harbor: "old", meadow: "new"}})

	if _, ok := col.Results[harbor]; ok {
		t.Fatal("a provider snapshot brought a dropped row back")
	}
	if col.Results[meadow] != "new" {
		t.Fatalf("meadow = %q, want new", col.Results[meadow])
	}
}
