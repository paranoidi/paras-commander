package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/testutil"
)

func touchTreeFile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, p)
}

// treeLoadHarness records ScheduleTreeChildLoad requests and plays the app: CancelTreeChildLoads
// discards everything in flight, settle lands what is pending.
type treeLoadHarness struct {
	t          *testing.T
	s          *State
	pending    []string
	dispatched []string
}

func newTreeLoadHarness(t *testing.T, s *State) *treeLoadHarness {
	h := &treeLoadHarness{t: t, s: s}
	s.ScheduleTreeChildLoad = func(r TreeChildLoadRequest) bool {
		h.pending = append(h.pending, r.DirID)
		h.dispatched = append(h.dispatched, r.DirID)
		return true
	}
	s.CancelTreeChildLoads = func() { h.pending = nil }
	return h
}

// land applies the oldest pending load from disk.
func (h *treeLoadHarness) land() {
	h.t.Helper()
	id := h.pending[0]
	h.pending = h.pending[1:]
	entries, err := h.s.loadTreeChildren(id)
	h.s.ApplyTreeChildLoad(id, entries, err, 10)
}

func (h *treeLoadHarness) settle() {
	h.t.Helper()
	for n := 0; len(h.pending) > 0; n++ {
		if n > 100 {
			h.t.Fatal("loads never settle")
		}
		h.land()
	}
}

func (h *treeLoadHarness) refresh(prefetch map[string]TreePrefetchResult) {
	h.t.Helper()
	s := h.s
	be, _, _, _, err := s.fetchBackendEntries(s.Path)
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := s.ApplyPeriodicRefresh(s.Path, be, 10, nil, prefetch, true); err != nil {
		h.t.Fatal(err)
	}
}

func orchardTree(t *testing.T) (*State, string) {
	root := t.TempDir()
	orchard := filepath.Join(root, "orchard")
	touchTreeFile(t, filepath.Join(orchard, "ember.txt"))
	touchTreeFile(t, filepath.Join(root, "lantern.txt"))
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s.SetListLayout(ListLayoutTree, 10)
	return &s, orchard
}

func TestSameDirReloadRedispatchesFirstExpandInFlight(t *testing.T) {
	s, orchard := orchardTree(t)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 1 {
		t.Fatalf("dispatches = %v, want one", h.dispatched)
	}
	touchTreeFile(t, filepath.Join(s.Path.String(), "willow.txt"))
	h.refresh(nil)
	if len(h.dispatched) != 2 || h.dispatched[1] != orchard || len(h.pending) != 1 {
		t.Fatalf("dispatched = %v pending = %v, want orchard re-dispatched", h.dispatched, h.pending)
	}
	if n := findTreeNode(s.TreeRoots, orchard); n == nil || !n.Value.Loading {
		t.Fatal("orchard must be loading again after the reload")
	}
	h.settle()
	if !s.TreeExpanded[orchard] {
		t.Fatal("orchard must be expanded once the re-dispatched load lands")
	}
	if n := findTreeNode(s.TreeRoots, orchard); n == nil || n.Children == nil {
		t.Fatal("orchard children missing after the re-dispatched load")
	}
}

func TestSameDirReloadServesLoadingFolderFromPrefetch(t *testing.T) {
	s, orchard := orchardTree(t)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	entries, err := s.loadTreeChildren(orchard)
	if err != nil {
		t.Fatal(err)
	}
	h.refresh(map[string]TreePrefetchResult{orchard: {Entries: entries}})
	if len(h.dispatched) != 1 {
		t.Fatalf("dispatched = %v, want no re-dispatch when prefetch serves the folder", h.dispatched)
	}
	if !s.TreeExpanded[orchard] {
		t.Fatal("orchard must be expanded from prefetch")
	}
}

func TestNavigationDoesNotCarryLoadingFolder(t *testing.T) {
	s, orchard := orchardTree(t)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	touchTreeFile(t, filepath.Join(other, "meadow", "harbor.txt"))
	otherLoc, err := pathloc.File(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyListing(otherLoc, nil, "", 10, 0, false); err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 1 || len(h.pending) != 0 {
		t.Fatalf("dispatched = %v pending = %v, want no re-dispatch after navigation", h.dispatched, h.pending)
	}
	if s.TreeExpanded[orchard] {
		t.Fatal("loading folder must not be remembered across navigation")
	}
}

func TestCollapseDuringRestoredLoadStaysNoOp(t *testing.T) {
	s, orchard := orchardTree(t)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	h.refresh(nil)
	if !s.TreeExpanded[orchard] {
		t.Fatal("test setup: restored load should leave TreeExpanded set")
	}
	s.Cursor = 0
	if err := s.CollapseTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleTreeExpand(10); err != nil {
		t.Fatal(err)
	}
	if !s.TreeExpanded[orchard] {
		t.Fatal("collapse must not clear the flag while the load is in flight")
	}
	h.settle()
	// The late result expands the folder; a press afterwards collapses it for good.
	if !s.TreeExpanded[orchard] {
		t.Fatal("orchard must be expanded after the load lands")
	}
	if err := s.CollapseTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	if s.TreeExpanded[orchard] {
		t.Fatal("collapse after the load landed must collapse")
	}
}

func TestExpandAllFullyCompletesAfterMidCascadeReload(t *testing.T) {
	root := t.TempDir()
	cedar := filepath.Join(root, "alder", "birch", "cedar")
	reed := filepath.Join(root, "willow", "reed")
	touchTreeFile(t, filepath.Join(cedar, "ember.txt"))
	touchTreeFile(t, filepath.Join(reed, "lantern.txt"))
	st, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s := &st
	s.SetListLayout(ListLayoutTree, 10)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandAllTreeFully(10); err != nil {
		t.Fatal(err)
	}
	if len(h.pending) != 2 {
		t.Fatalf("pending = %v, want the two depth-0 dirs", h.pending)
	}
	h.land() // alder lands, willow still in flight
	touchTreeFile(t, filepath.Join(root, "orchard.txt"))
	h.refresh(nil)
	h.settle()
	for _, id := range []string{filepath.Join(root, "alder"), filepath.Join(root, "alder", "birch"), cedar, filepath.Join(root, "willow"), reed} {
		if !s.TreeExpanded[id] {
			t.Errorf("%s not expanded after cascade", id)
		}
	}
	if s.treeExpandQuiet != 0 || s.treeExpandAllAuto {
		t.Fatalf("quiet = %d auto = %v, want 0 and disarmed", s.treeExpandQuiet, s.treeExpandAllAuto)
	}
}

func TestExpandAllFullyDrivenWhenReloadResolvesLevelFromPrefetch(t *testing.T) {
	root := t.TempDir()
	cedar := filepath.Join(root, "alder", "birch", "cedar")
	touchTreeFile(t, filepath.Join(cedar, "ember.txt"))
	st, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s := &st
	s.SetListLayout(ListLayoutTree, 10)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandAllTreeFully(10); err != nil {
		t.Fatal(err)
	}
	alder := filepath.Join(root, "alder")
	entries, err := s.loadTreeChildren(alder)
	if err != nil {
		t.Fatal(err)
	}
	h.refresh(map[string]TreePrefetchResult{alder: {Entries: entries}})
	if len(h.pending) != 1 || h.pending[0] != filepath.Join(alder, "birch") {
		t.Fatalf("pending = %v, want the cascade to have moved on to birch", h.pending)
	}
	h.settle()
	if !s.TreeExpanded[cedar] || s.treeExpandQuiet != 0 || s.treeExpandAllAuto {
		t.Fatalf("cedar=%v quiet=%d auto=%v, want fully expanded and disarmed", s.TreeExpanded[cedar], s.treeExpandQuiet, s.treeExpandAllAuto)
	}
}
