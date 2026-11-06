package meta

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// pumpWakes applies posted meta wakes like Run's interrupt switch until the panel's columns have
// nothing pending. It returns how many WakePayload and ExecFailedPayload events it applied.
func pumpWakes(t *testing.T, screen tcell.Screen, h *Handler, panelID int) (wakes, fails int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pending := 0
		for _, c := range h.model.MetaResults[panelID] {
			pending += c.PendingCount
		}
		if pending == 0 {
			return wakes, fails
		}
		ev := screen.PollEvent()
		if in, ok := ev.(*tcell.EventInterrupt); ok {
			switch d := in.Data().(type) {
			case WakePayload:
				wakes++
				h.HandleWake(d)
			case ExecFailedPayload:
				fails++
				h.HandleExecFailed(d)
			}
		}
	}
	t.Fatal("provider results never resolved")
	return wakes, fails
}

func providerFixture(t *testing.T, p *metacmds.Provider) (tcell.Screen, *Handler, *panel.State, []localfs.Entry) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "harbor.txt", Path: filepath.Join(dir, "harbor.txt"), Type: localfs.EntryFile},
		{Name: "meadow.txt", Path: filepath.Join(dir, "meadow.txt"), Type: localfs.EntryFile},
		{Name: "thistle", Path: filepath.Join(dir, "thistle"), Type: localfs.EntryDirectory},
	}
	pan := testPanel(t, dir, append([]localfs.Entry(nil), entries...))
	fh := &fakeHost{panels: [2]*panel.State{pan}}
	h := New(Deps{Host: fh, Screen: screen, Model: &ui.Model{}, Config: config.Default(), Providers: []*metacmds.Provider{p}})
	return screen, h, pan, entries
}

func TestProviderColumnCalledOncePerListing(t *testing.T) {
	var calls atomic.Int32
	p := &metacmds.Provider{Name: "lantern.tags", Column: "Tags", Values: func(_ context.Context, dir string, entries []localfs.Entry) (map[string]string, error) {
		calls.Add(1)
		out := map[string]string{}
		for _, e := range entries {
			if e.Name != "meadow.txt" {
				out[e.Path] = strings.ToUpper(e.Name[:1]) + "-" + filepath.Base(dir)[:1]
			}
		}
		return out, nil
	}}
	screen, h, pan, entries := providerFixture(t, p)

	mf := metacmds.WithProviders(nil, h.providers)
	defs := metacmds.SortEntriesForDisplay([]string{"lantern.tags"}, mf)
	if len(defs) != 1 || defs[0].Provider != p || defs[0].Column != "Tags" {
		t.Fatalf("provider entry = %+v", defs)
	}
	h.runForPanel(0, defs, []ui.MetaColumnState{{EntryName: "lantern.tags", ColumnTitle: "Tags"}})
	if col := h.model.MetaResults[0][0]; col.PendingCount != len(entries) {
		t.Fatalf("PendingCount = %d, want every entry pending", col.PendingCount)
	}
	pumpWakes(t, screen, h, 0)

	if n := calls.Load(); n != 1 {
		t.Fatalf("provider called %d times, want once per listing", n)
	}
	col := h.model.MetaResults[0][0]
	if want := "H-" + filepath.Base(pan.PathString())[:1]; col.Results[entries[0].Path] != want {
		t.Fatalf("harbor = %q, want %q (provider got the panel dir)", col.Results[entries[0].Path], want)
	}
	if got := col.Results[entries[1].Path]; got != "" {
		t.Fatalf("meadow (left out by the provider) = %q, want empty", got)
	}
}

func TestProviderColumnSortsAndFilters(t *testing.T) {
	p := &metacmds.Provider{Name: "lantern.tags", Values: func(_ context.Context, _ string, entries []localfs.Entry) (map[string]string, error) {
		vals := map[string]string{"harbor.txt": "zinnia", "meadow.txt": "aster", "thistle": "marigold"}
		out := map[string]string{}
		for _, e := range entries {
			out[e.Path] = vals[e.Name]
		}
		return out, nil
	}}
	screen, h, pan, _ := providerFixture(t, p)
	defs := metacmds.SortEntriesForDisplay([]string{"lantern.tags"}, metacmds.WithProviders(nil, h.providers))
	h.runForPanel(0, defs, []ui.MetaColumnState{{EntryName: "lantern.tags"}})
	pumpWakes(t, screen, h, 0)

	metaValue := func(column string) (map[string]string, string, bool) {
		c, ok := ui.MetaColumnByName(h.model.MetaResults[0], column)
		if !ok {
			return nil, "", false
		}
		return c.Results, c.Pending, true
	}
	sorted := append([]localfs.Entry(nil), pan.Entries...)
	panel.SortEntries(sorted, panel.SortState{Mode: panel.SortMeta, MetaColumn: "lantern.tags"}, nil, false, metaValue)
	var order []string
	for _, e := range sorted {
		order = append(order, e.Name)
	}
	if got := strings.Join(order, ","); got != "meadow.txt,thistle,harbor.txt" {
		t.Fatalf("sorted by provider column = %s", got)
	}

	f, err := panel.PatternFilter(panel.FilterSpec{Pattern: "*gold*"}, func() panel.GroupSelectMeta {
		return panel.GroupSelectMeta{Cols: []map[string]string{h.model.MetaResults[0][0].Results}, OnlyMeta: true}
	})
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, e := range pan.Entries {
		if f.Match(e, pan) {
			kept = append(kept, e.Name)
		}
	}
	if strings.Join(kept, ",") != "thistle" {
		t.Fatalf("filter on provider values kept %v, want [thistle]", kept)
	}
}

func TestProviderErrorLeavesCellsEmptyAndReportsOnce(t *testing.T) {
	p := &metacmds.Provider{Name: "lantern.tags", Values: func(context.Context, string, []localfs.Entry) (map[string]string, error) {
		return nil, errors.New("index unavailable")
	}}
	screen, h, _, entries := providerFixture(t, p)
	defs := metacmds.SortEntriesForDisplay([]string{"lantern.tags"}, metacmds.WithProviders(nil, h.providers))
	h.runForPanel(0, defs, []ui.MetaColumnState{{EntryName: "lantern.tags"}})
	pumpWakes(t, screen, h, 0)
	for _, e := range entries {
		if v := h.model.MetaResults[0][0].Results[e.Path]; v != "" {
			t.Fatalf("%s = %q, want empty after a provider error", e.Name, v)
		}
	}
}

func TestWithProvidersKeepsProvidersWithoutMetaFile(t *testing.T) {
	p := &metacmds.Provider{Name: "lantern.tags"}
	mf := metacmds.WithProviders(&metacmds.MetaFile{Entries: []metacmds.MetaEntry{{Name: "words"}, {Name: "lantern.tags", File: "echo"}}}, []*metacmds.Provider{p})
	if len(mf.Entries) != 2 || mf.Entries[0].Name != "words" || mf.Entries[1].Provider != p {
		t.Fatalf("entries = %+v, want meta.toml entry then the provider replacing its namesake", mf.Entries)
	}
	if got := entriesFromFile(metacmds.WithProviders(nil, []*metacmds.Provider{p})); len(got) != 1 || got[0].Name != "lantern.tags" {
		t.Fatalf("picker entries without meta.toml = %+v", got)
	}
}

// treeFixture returns a handler over a tree-layout panel rooted at a directory holding the
// sub-directories orchard and meadow, each with files. Nothing is expanded yet.
func treeFixture(t *testing.T, defs ...*metacmds.Provider) (tcell.Screen, *Handler, *panel.State, string) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	root := t.TempDir()
	for _, f := range []string{"orchard/willow.txt", "orchard/birch.txt", "meadow/clover.txt"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pan := &panel.State{}
	if err := pan.Load(root); err != nil {
		t.Fatal(err)
	}
	if !pan.SetListLayout(panel.ListLayoutTree, 20) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	fh := &fakeHost{panels: [2]*panel.State{pan}}
	h := New(Deps{Host: fh, Screen: screen, Model: &ui.Model{}, Config: config.Default(), Providers: defs})
	return screen, h, pan, root
}

// dirRecorder is a provider that records the directories it is asked about and answers
// "<dir base>:<entry name>" for every entry, failing for directories named in fail.
type dirRecorder struct {
	mu   sync.Mutex
	dirs []string
	fail map[string]bool
}

func (r *dirRecorder) provider() *metacmds.Provider {
	return &metacmds.Provider{Name: "lantern.tags", Column: "Tags", Values: func(_ context.Context, dir string, entries []localfs.Entry) (map[string]string, error) {
		r.mu.Lock()
		r.dirs = append(r.dirs, dir)
		r.mu.Unlock()
		if r.fail[filepath.Base(dir)] {
			return nil, errors.New("index unavailable")
		}
		out := map[string]string{}
		for _, e := range entries {
			out[e.Path] = filepath.Base(dir) + ":" + e.Name
		}
		return out, nil
	}}
}

func (r *dirRecorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.dirs...)
	slices.Sort(out)
	return out
}

func providerDefs(h *Handler) []metacmds.MetaEntry {
	return metacmds.SortEntriesForDisplay([]string{"lantern.tags"}, metacmds.WithProviders(nil, h.providers))
}

func TestRunForPanel_providerCoversExpandedTreeChildren(t *testing.T) {
	rec := &dirRecorder{}
	screen, h, pan, root := treeFixture(t, rec.provider())
	if err := pan.ExpandAllTreeShallow(20); err != nil {
		t.Fatal(err)
	}

	h.runForPanel(0, providerDefs(h), []ui.MetaColumnState{{EntryName: "lantern.tags"}})
	wakes, fails := pumpWakes(t, screen, h, 0)

	want := []string{root, filepath.Join(root, "meadow"), filepath.Join(root, "orchard")}
	if got := rec.seen(); !slices.Equal(got, want) {
		t.Fatalf("provider dirs = %v, want %v (root plus each tree child's own directory)", got, want)
	}
	if wakes != 1 || fails != 0 {
		t.Fatalf("wakes = %d, fails = %d; want one batched wake across all directories", wakes, fails)
	}
	res := h.model.MetaResults[0][0].Results
	for path, v := range map[string]string{
		filepath.Join(root, "orchard"):               filepath.Base(root) + ":orchard",
		filepath.Join(root, "orchard", "willow.txt"): "orchard:willow.txt",
		filepath.Join(root, "orchard", "birch.txt"):  "orchard:birch.txt",
		filepath.Join(root, "meadow", "clover.txt"):  "meadow:clover.txt",
	} {
		if res[path] != v {
			t.Fatalf("Results[%s] = %q, want %q", path, res[path], v)
		}
	}
}

func TestRunForPanel_providerErrorInOneTreeDirectoryKeepsOthers(t *testing.T) {
	rec := &dirRecorder{fail: map[string]bool{"orchard": true}}
	screen, h, pan, root := treeFixture(t, rec.provider())
	if err := pan.ExpandAllTreeShallow(20); err != nil {
		t.Fatal(err)
	}
	h.runForPanel(0, providerDefs(h), []ui.MetaColumnState{{EntryName: "lantern.tags"}})
	wakes, fails := pumpWakes(t, screen, h, 0)
	if wakes != 1 || fails != 1 {
		t.Fatalf("wakes = %d, fails = %d; want one wake and one failure report", wakes, fails)
	}
	res := h.model.MetaResults[0][0].Results
	if v := res[filepath.Join(root, "orchard", "willow.txt")]; v != "" {
		t.Fatalf("failed directory cell = %q, want empty", v)
	}
	if v := res[filepath.Join(root, "meadow", "clover.txt")]; v != "meadow:clover.txt" {
		t.Fatalf("other directory cell = %q, want its value", v)
	}
}

func TestReconcileForPanel_dispatchesNewTreeChildrenIntoRun(t *testing.T) {
	rec := &dirRecorder{}
	screen, h, pan, root := treeFixture(t, rec.provider())
	h.runForPanel(0, providerDefs(h), []ui.MetaColumnState{{EntryName: "lantern.tags"}})
	pumpWakes(t, screen, h, 0)
	h.ReconcileForPanel(0) // nothing new: no dispatch
	if got := rec.seen(); len(got) != 1 {
		t.Fatalf("provider dirs = %v, want only the root run", got)
	}
	gen := h.runGen[0]

	if err := pan.ExpandAllTreeShallow(20); err != nil {
		t.Fatal(err)
	}
	h.ReconcileForPanel(0)
	pumpWakes(t, screen, h, 0)

	if h.runGen[0] != gen || h.loadPending[0] {
		t.Fatal("reconcile restarted the run instead of dispatching into it")
	}
	want := []string{root, filepath.Join(root, "meadow"), filepath.Join(root, "orchard")}
	if got := rec.seen(); !slices.Equal(got, want) {
		t.Fatalf("provider dirs = %v, want %v (root once, then only the new children)", got, want)
	}
	res := h.model.MetaResults[0][0].Results
	if v := res[filepath.Join(root, "orchard", "birch.txt")]; v != "orchard:birch.txt" {
		t.Fatalf("new child = %q, want its value", v)
	}

	// Already-covered rows are left alone by later events.
	res[filepath.Join(root, "orchard", "birch.txt")] = "kept"
	h.ReconcileForPanel(0)
	if got := rec.seen(); len(got) != 3 || res[filepath.Join(root, "orchard", "birch.txt")] != "kept" {
		t.Fatalf("existing rows re-dispatched: dirs = %v", got)
	}
}

func TestRunForPanel_shellColumnRunsInTreeChildDirectory(t *testing.T) {
	screen, h, pan, root := treeFixture(t)
	if err := pan.ExpandAllTreeShallow(20); err != nil {
		t.Fatal(err)
	}
	defs := []metacmds.MetaEntry{{Name: "where", File: "pwd -P", Dirs: "pwd -P", Workers: 1}}
	h.runForPanel(0, defs, []ui.MetaColumnState{{EntryName: "where"}})
	pumpWakes(t, screen, h, 0)

	res := h.model.MetaResults[0][0].Results
	real := func(p string) string {
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for path, wantDir := range map[string]string{
		filepath.Join(root, "orchard"):               root,
		filepath.Join(root, "orchard", "willow.txt"): filepath.Join(root, "orchard"),
		filepath.Join(root, "meadow", "clover.txt"):  filepath.Join(root, "meadow"),
	} {
		if got := res[path]; got != real(wantDir) {
			t.Fatalf("Results[%s] = %q, want cwd %q", path, got, real(wantDir))
		}
	}
}
