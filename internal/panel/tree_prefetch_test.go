package panel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func prefetchTestChildren(t *testing.T, s *State, dir string) TreePrefetchResult {
	t.Helper()
	loc, err := pathloc.Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	be, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(loc, 0))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fsbackend.ToPanelEntries(be)
	if err != nil {
		t.Fatal(err)
	}
	return TreePrefetchResult{Entries: entries}
}

func prefetchTestTree(t *testing.T) (root, meadow, harbor string) {
	t.Helper()
	root = t.TempDir()
	meadow = filepath.Join(root, "meadow")
	harbor = filepath.Join(meadow, "harbor")
	if err := os.MkdirAll(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(harbor, "willow.txt"), filepath.Join(meadow, "lantern.txt"), filepath.Join(root, "beacon.txt")} {
		if err := os.WriteFile(f, []byte("xy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, meadow, harbor
}

func TestApplyListingPrefetchedRestoresNestedExpansionsWithoutAsyncLoads(t *testing.T) {
	root, meadow, harbor := prefetchTestTree(t)
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s.SetListLayout(ListLayoutTree, 10)
	s.TreeExpanded = map[string]bool{meadow: true, harbor: true}
	s.ScheduleTreeChildLoad = func(TreeChildLoadRequest) bool {
		t.Error("ScheduleTreeChildLoad called despite prefetched children")
		return true
	}
	if got := s.TreePrefetchIDs(s.Path); len(got) != 2 {
		t.Fatalf("TreePrefetchIDs = %v, want meadow and harbor", got)
	}
	be, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(s.Path, 0))
	if err != nil {
		t.Fatal(err)
	}
	prefetch := map[string]TreePrefetchResult{
		meadow: prefetchTestChildren(t, &s, meadow),
		harbor: prefetchTestChildren(t, &s, harbor),
	}
	if err := s.ApplyListingPrefetched(s.Path, be, "", 10, NoIndexCursorFallback, false, nil, prefetch); err != nil {
		t.Fatal(err)
	}
	// beacon, meadow, harbor, willow, lantern
	if got := s.VisibleEntryCount(); got != 5 {
		t.Fatalf("VisibleEntryCount = %d, want 5", got)
	}
	if _, ok := s.ListingEntryAt(filepath.Join(harbor, "willow.txt")); !ok {
		t.Fatal("tree child not indexed")
	}
	if s.treePrefetch != nil {
		t.Fatal("transient prefetch not cleared")
	}
}

func TestApplyListingPrefetchedErrorCollapsesDirectory(t *testing.T) {
	root, meadow, harbor := prefetchTestTree(t)
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s.SetListLayout(ListLayoutTree, 10)
	s.TreeExpanded = map[string]bool{meadow: true, harbor: true}
	be, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(s.Path, 0))
	if err != nil {
		t.Fatal(err)
	}
	prefetch := map[string]TreePrefetchResult{
		meadow: prefetchTestChildren(t, &s, meadow),
		harbor: {Err: errors.New("denied")},
	}
	if err := s.ApplyListingPrefetched(s.Path, be, "", 10, NoIndexCursorFallback, false, nil, prefetch); err != nil {
		t.Fatal(err)
	}
	if s.TreeExpanded[harbor] {
		t.Fatal("failed prefetch dir must be dropped from TreeExpanded")
	}
	// beacon, meadow, harbor (collapsed), lantern
	if got := s.VisibleEntryCount(); got != 4 {
		t.Fatalf("VisibleEntryCount = %d, want 4", got)
	}
}

func TestApplyPeriodicRefreshPrefetchedRestoresNestedExpansionsWithoutAsyncLoads(t *testing.T) {
	root, meadow, harbor := prefetchTestTree(t)
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s.SetListLayout(ListLayoutTree, 10)
	s.TreeExpanded = map[string]bool{meadow: true, harbor: true}
	s.ScheduleTreeChildLoad = func(TreeChildLoadRequest) bool {
		t.Error("ScheduleTreeChildLoad called despite prefetched children")
		return true
	}
	if err := os.WriteFile(filepath.Join(root, "orchard.txt"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	be, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(s.Path, 0))
	if err != nil {
		t.Fatal(err)
	}
	prefetch := map[string]TreePrefetchResult{
		meadow: prefetchTestChildren(t, &s, meadow),
		harbor: prefetchTestChildren(t, &s, harbor),
	}
	applied, err := s.ApplyPeriodicRefresh(s.Path, be, 10, nil, prefetch, false)
	if err != nil || !applied {
		t.Fatalf("ApplyPeriodicRefresh = %v, %v", applied, err)
	}
	// beacon, orchard, meadow, harbor, willow, lantern
	if got := s.VisibleEntryCount(); got != 6 {
		t.Fatalf("VisibleEntryCount = %d, want 6", got)
	}
}

func TestTreeRefreshPlanTiersAndRoundRobin(t *testing.T) {
	root := t.TempDir()
	names := []string{"anchor", "bramble", "cobalt", "dune", "ember"}
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n, "inner"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s.SetListLayout(ListLayoutTree, 3)
	s.TreeExpanded = map[string]bool{}
	for _, n := range names {
		s.TreeExpanded[filepath.Join(root, n)] = true
	}
	if err := s.Refresh(3); err != nil {
		t.Fatal(err)
	}
	// Rows: anchor, anchor/inner, bramble, bramble/inner, ... Put the cursor on anchor/inner
	// (caret parent = anchor) with a 3-row viewport showing anchor, anchor/inner, bramble.
	s.Cursor, s.ScrollOffset = 1, 0
	abs := func(n string) string { return filepath.Join(root, n) }
	plan := s.TreeRefreshPlan(3, 1)
	// bramble is the parent of "bramble/inner" only when that row is visible; row 2 is bramble
	// itself, whose parent is root (not in the set), so bramble is off-screen.
	want := []TreeRefreshReq{{abs("anchor"), TreeRefreshCaret}, {abs("bramble"), TreeRefreshOffscreen}}
	if !slices.Equal(plan, want) {
		t.Fatalf("plan = %v, want %v", plan, want)
	}
	plan = s.TreeRefreshPlan(3, 1)
	if len(plan) != 2 || plan[1].ID != abs("cobalt") || plan[1].Tier != TreeRefreshOffscreen {
		t.Fatalf("second plan = %v, want cobalt next", plan)
	}
	// Widen the viewport: bramble's child row becomes visible, promoting it to the visible tier.
	s.ScrollOffset = 0
	plan = s.TreeRefreshPlan(4, 1)
	if plan[0].Tier != TreeRefreshCaret || plan[1] != (TreeRefreshReq{abs("bramble"), TreeRefreshVisible}) {
		t.Fatalf("wide plan = %v, want caret anchor then visible bramble", plan)
	}
}
