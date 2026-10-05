package panel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panellist"
)

// TestTreeNestedNewFileMarkedAfterReload checks that a file created inside an expanded nested
// directory gets the new-file mark on a same-directory reload, while pre-existing siblings do not.
func TestTreeNestedNewFileMarkedAfterReload(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alpha, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alpha, "meadow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(10); err != nil {
		t.Fatal(err)
	}
	tiers := map[string]panellist.NewFileMarkTier{}
	for _, e := range s.treeByPath {
		tiers[e.Name] = s.NewFileMarkTier(e)
	}
	if got := tiers["meadow.txt"]; got != panellist.NewFileMarkLatest {
		t.Fatalf("meadow.txt tier = %v, want latest (tiers=%v)", got, tiers)
	}
	if got, ok := tiers["beacon.txt"]; !ok || got != panellist.NewFileMarkNone {
		t.Fatalf("beacon.txt tier = %v ok=%v, want none", got, ok)
	}
}

// TestApplyPeriodicRefreshForceShowsDeepNewFile checks that a file created two levels deep appears
// (and is marked) when the root listing is unchanged but force is set.
func TestApplyPeriodicRefreshForceShowsDeepNewFile(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	brook := filepath.Join(alpha, "brook")
	if err := os.MkdirAll(brook, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brook, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	if err := s.ExpandTreeCursorRow(10); err != nil { // alpha
		t.Fatal(err)
	}
	s.Cursor = 1 // brook, directly under alpha
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brook, "meadow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	be, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(s.Path, 0))
	if err != nil {
		t.Fatal(err)
	}
	prefetch := map[string]TreePrefetchResult{
		alpha: prefetchTestChildren(t, &s, alpha),
		brook: prefetchTestChildren(t, &s, brook),
	}
	if _, err := s.ApplyPeriodicRefresh(s.Path, be, 10, nil, prefetch, true); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range s.treeByPath {
		if e.Name == "meadow.txt" {
			found = true
			if got := s.NewFileMarkTier(e); got != panellist.NewFileMarkLatest {
				t.Fatalf("meadow.txt tier = %v, want latest", got)
			}
		}
	}
	if !found {
		t.Fatal("meadow.txt missing from tree after forced refresh")
	}
}
