package panel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	applied, err := s.ApplyPeriodicRefresh(s.Path, be, 10, nil, prefetch)
	if err != nil || !applied {
		t.Fatalf("ApplyPeriodicRefresh = %v, %v", applied, err)
	}
	// beacon, orchard, meadow, harbor, willow, lantern
	if got := s.VisibleEntryCount(); got != 6 {
		t.Fatalf("VisibleEntryCount = %d, want 6", got)
	}
}
