package panel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func matchNamesFixture(t *testing.T, names ...string) (State, string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, s.Path.String()
}

func mapPtr(m map[string]struct{}) uintptr { return reflect.ValueOf(m).Pointer() }

func TestNamesDirectlyInCached(t *testing.T) {
	t.Parallel()
	s, dir := matchNamesFixture(t, "lantern", "meadow", "violin")
	m := s.NamesDirectlyIn(dir)
	if len(m) != 3 {
		t.Fatalf("names = %v", m)
	}
	if mapPtr(s.NamesDirectlyIn(dir)) != mapPtr(m) {
		t.Fatal("second call rebuilt the map")
	}
	cp := s
	if mapPtr(cp.NamesDirectlyIn(dir)) != mapPtr(m) {
		t.Fatal("State copy did not share the cache")
	}
	s.Move(1, 10)
	if mapPtr(s.NamesDirectlyIn(dir)) != mapPtr(m) {
		t.Fatal("cursor move invalidated the cache")
	}
	other := s.NamesDirectlyIn(filepath.Join(dir, "elsewhere"))
	if len(other) != 0 || mapPtr(other) == mapPtr(m) {
		t.Fatalf("different dir returned %v", other)
	}
}

func TestNamesDirectlyInTracksListingChanges(t *testing.T) {
	t.Parallel()
	s, dir := matchNamesFixture(t, "lantern", "meadow")
	before := s.NamesDirectlyIn(dir)

	added := filepath.Join(dir, "harbor")
	if !s.InsertEntries([]localfs.Entry{{Name: "harbor", Path: added, Type: localfs.EntryFile}}, 10) {
		t.Fatal("InsertEntries did nothing")
	}
	got := s.NamesDirectlyIn(dir)
	if _, ok := got["harbor"]; !ok || mapPtr(got) == mapPtr(before) {
		t.Fatalf("insert not visible: %v", got)
	}

	if !s.RenameEntry(added, "willow", 10) {
		t.Fatal("RenameEntry did nothing")
	}
	got = s.NamesDirectlyIn(dir)
	if _, ok := got["willow"]; !ok {
		t.Fatalf("rename not visible: %v", got)
	}
	if _, ok := got["harbor"]; ok {
		t.Fatalf("old name still present: %v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "cedar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(10); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.NamesDirectlyIn(dir)["cedar"]; !ok {
		t.Fatal("reload not visible")
	}
}

func TestNamesDirectlyInLiteralRebuilds(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "orchard")
	s := State{Path: pathloc.MustParse(dir)}
	s.Entries = append(s.Entries, localfs.Entry{Name: "anchor", Path: filepath.Join(dir, "anchor")})
	if got := s.NamesDirectlyIn(dir); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	s.Entries = append(s.Entries, localfs.Entry{Name: "ember", Path: filepath.Join(dir, "ember")})
	if got := s.NamesDirectlyIn(dir); len(got) != 2 {
		t.Fatalf("direct Entries edit not seen: %v", got)
	}
}

// Tree rows and filters feed the name set, so expand, collapse, filter and layout changes must not
// serve the previous answer.
func TestNamesDirectlyInTracksTreeAndFilter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, f := range []string{"meadow/anchor", "meadow/violin", "pebble"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	meadow := filepath.Join(s.Path.String(), "meadow")

	s.SetListLayout(ListLayoutTree, 10)
	if got := s.NamesDirectlyIn(meadow); len(got) != 0 {
		t.Fatalf("collapsed: %v", got)
	}
	if err := s.ToggleTreeExpand(10); err != nil { // cursor starts on meadow
		t.Fatal(err)
	}
	if got := s.NamesDirectlyIn(meadow); len(got) != 2 {
		t.Fatalf("expanded: %v", got)
	}
	s.SetEntryFilter(&EntryFilter{ID: "no-anchor", Match: func(e localfs.Entry, _ *State) bool { return e.Name != "anchor" }})
	got := s.NamesDirectlyIn(meadow)
	if _, hidden := got["anchor"]; hidden || len(got) != 1 {
		t.Fatalf("filtered: %v", got)
	}
	s.SetEntryFilter(nil)
	if got := s.NamesDirectlyIn(meadow); len(got) != 2 {
		t.Fatalf("filter cleared: %v", got)
	}
	if err := s.ToggleTreeExpand(10); err != nil {
		t.Fatal(err)
	}
	if got := s.NamesDirectlyIn(meadow); len(got) != 0 {
		t.Fatalf("collapsed again: %v", got)
	}

	top := s.Path.String()
	treeNames := s.NamesDirectlyIn(top)
	s.SetEntryFilter(&EntryFilter{ID: "no-pebble", Match: func(e localfs.Entry, _ *State) bool { return e.Name != "pebble" }})
	if _, hidden := s.NamesDirectlyIn(top)["pebble"]; hidden {
		t.Fatalf("tree filter must hide the name (before: %v)", treeNames)
	}
	s.SetListLayout(ListLayoutFlat, 10)
	if _, ok := s.NamesDirectlyIn(top)["pebble"]; !ok {
		t.Fatal("flat layout reads Entries unfiltered, so the name must return")
	}
}
