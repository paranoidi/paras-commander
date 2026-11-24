package panel

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// tagListRow is a row of the flat tag view "beach" for the made-up host path.
func tagListRow(host string, dir bool) fsbackend.Entry {
	q := pathloc.TagQuery{Flat: true, Tags: []string{"beach"}}
	loc, err := pathloc.TagView(q, host)
	if err != nil {
		panic(err)
	}
	e := fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryFile}
	if dir {
		e.Type = fsbackend.EntryDirectory
	}
	return e
}

func tagList() pathloc.Path {
	loc, err := pathloc.TagView(pathloc.TagQuery{Flat: true, Tags: []string{"beach"}}, "/")
	if err != nil {
		panic(err)
	}
	return loc
}

func tagListState(t *testing.T, hosts ...string) *State {
	t.Helper()
	s := &State{Path: tagList(), Sort: defaultSortState()}
	var rows []fsbackend.Entry
	for _, h := range hosts {
		rows = append(rows, tagListRow(h, false))
	}
	if _, err := s.ApplyListingPrefetched(tagList(), rows, "", 10, noIndexCursorFallback, false, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	return s
}

func cursorPath(t *testing.T, s *State) string {
	t.Helper()
	e, ok := s.CurrentEntry()
	if !ok {
		t.Fatal("no cursor row")
	}
	return e.Path
}

func TestSelectNameIsThePathOnlyInAFlatTagList(t *testing.T) {
	t.Parallel()
	row := tagListRow("/orchard/notes.txt", false).Loc
	if got := SelectName(tagList(), row.String(), row.Base()); got != row.String() {
		t.Fatalf("in the list: %q", got)
	}
	plain := pathloc.MustParse("fileindex:///orchard")
	if got := SelectName(plain, "fileindex:///orchard/notes.txt", "notes.txt"); got != "notes.txt" {
		t.Fatalf("in a plain directory: %q", got)
	}
}

func TestTagListReloadKeepsTheRightRowOfEqualNames(t *testing.T) {
	t.Parallel()
	s := tagListState(t, "/cedar/notes.txt", "/orchard/notes.txt", "/orchard/ropes.txt")
	want := tagListRow("/orchard/notes.txt", false).Loc.String()
	if !s.SelectVisibleEntry(want) {
		t.Fatal("row not selectable by path")
	}
	fresh := []fsbackend.Entry{
		tagListRow("/cedar/notes.txt", false), tagListRow("/cedar/sails.txt", false),
		tagListRow("/orchard/notes.txt", false), tagListRow("/orchard/ropes.txt", false),
	}
	if _, err := s.ApplyPeriodicRefresh(tagList(), fresh, 10, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if got := cursorPath(t, s); got != want {
		t.Fatalf("periodic refresh left the cursor on %s, want %s", got, want)
	}
	// The cursor row goes: the row below it takes the cursor, not a same-named one above.
	if _, err := s.ApplyPeriodicRefresh(tagList(), []fsbackend.Entry{
		tagListRow("/cedar/notes.txt", false), tagListRow("/orchard/ropes.txt", false),
	}, 10, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if got, want := cursorPath(t, s), tagListRow("/orchard/ropes.txt", false).Loc.String(); got != want {
		t.Fatalf("after the row went the cursor is on %s, want %s", got, want)
	}
}

func TestTagListMarksNoRowByAnotherDirectorysName(t *testing.T) {
	t.Parallel()
	s := tagListState(t, "/orchard/notes.txt")
	fresh := []fsbackend.Entry{tagListRow("/cedar/notes.txt", false), tagListRow("/orchard/notes.txt", false)}
	if _, err := s.ApplyPeriodicRefresh(tagList(), fresh, 10, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	for _, e := range s.Entries {
		if got := s.NewFileMarkTier(e); got != panellist.NewFileMarkNone {
			t.Fatalf("%s is marked new (%v)", e.Path, got)
		}
	}
	// Rename marks are kept by the row's own directory, so equal names stay apart.
	orchard := s.Entries[1]
	s.AddRenameMarks(pathloc.MustParse(orchard.Path).Parent(), []string{orchard.Name})
	if !s.IsRenameMarked(orchard) || s.IsRenameMarked(s.Entries[0]) {
		t.Fatal("a rename mark reached the same-named row of another directory")
	}
}

func TestTagListEntryDirIsTheHostDirectory(t *testing.T) {
	t.Parallel()
	s := tagListState(t, "/orchard/notes.txt", "/rootfile.txt")
	got := map[string]string{}
	for _, e := range s.Entries {
		got[e.Name] = s.EntryDir(e).String()
	}
	if got["notes.txt"] != "fileindex@(beach):///orchard" || got["rootfile.txt"] != "fileindex@(beach):///" {
		t.Fatalf("EntryDir = %v", got)
	}
	if s.CursorDir().IsZero() {
		t.Fatal("no cursor dir")
	}
}

func TestSelectVisibleEntryByPathKeepsTheRowBoundary(t *testing.T) {
	t.Parallel()
	row := tagListRow("/orchard/lantern", true).Loc.String()
	inside := row + "//moss"                                              // a child of the row
	alsoTagged := tagListRow("/orchard/lantern/moss", false).Loc.String() // a row of its own
	if filepath.Clean(inside) != filepath.Clean(alsoTagged) {
		t.Fatal("test premise: the two paths clean alike")
	}
	s := &State{Path: tagList(), Entries: []localfs.Entry{
		{Name: "moss", Path: inside, Type: localfs.EntryFile},
		{Name: "moss", Path: alsoTagged, Type: localfs.EntryFile},
	}}
	if !s.SelectVisibleEntry(alsoTagged) || s.Cursor != 1 {
		t.Fatalf("cursor = %d, want the row of its own", s.Cursor)
	}
	if !s.SelectVisibleEntry(inside) || s.Cursor != 0 {
		t.Fatalf("cursor = %d, want the child", s.Cursor)
	}
}

func TestTagListRowsDoNotShowTheSelectionStrip(t *testing.T) {
	t.Parallel()
	s := tagListState(t, "/cedar/notes.txt", "/orchard/notes.txt")
	for _, e := range s.Entries {
		s.AddSelection(e.Path)
	}
	if n := len(s.SelectedStripPaths()); n != 0 {
		t.Fatalf("strip lists %d paths for selections made in the list itself", n)
	}
}

func TestTagListTreeChainsEndAtTheList(t *testing.T) {
	t.Parallel()
	row := tagListRow("/orchard/lantern", true).Loc.String()
	sub := row + "//sub"
	deeper := sub + "/moss"
	list := tagList()
	got := treePrefetchIDsFor(list, map[string]bool{row: true, sub: true, deeper: true})
	if !slices.Equal(got, []string{row, sub, deeper}) && !slices.Equal(got, []string{deeper, row, sub}) {
		t.Fatalf("expanded chain = %v, want the row and everything below it", got)
	}
	if got := treePrefetchIDsFor(list, map[string]bool{sub: true}); len(got) != 0 {
		t.Fatalf("%v expanded under a collapsed row", got)
	}
	// A path outside the list's view ends the walk instead of looping.
	other := pathloc.MustParse("fileindex@(harvest):///orchard/lantern//sub").String()
	if got := treePrefetchIDsFor(list, map[string]bool{other: true}); len(got) != 0 {
		t.Fatalf("%v from another view", got)
	}
}

func TestFirstMissingChildNameInAFlatList(t *testing.T) {
	t.Parallel()
	list := tagList()
	row := tagListRow("/orchard/lantern", true).Loc
	inner := pathloc.MustParse(row.String() + "//sub/moss")
	if got := firstMissingChildName(list, inner); got != row.String() {
		t.Fatalf("firstMissingChildName = %q, want the row path %q", got, row)
	}
}
