package ui

import (
	"slices"
	"testing"
)

func uniqueDisplays(rows []UniqueRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Value.Display
	}
	return out
}

func TestUniqueRowsTree(t *testing.T) {
	st := &UniqueViewState{Root: "/srv/meadow", Files: []UniqueFile{
		{Path: "/srv/meadow/clover.txt", Size: 1},
		{Path: "/srv/meadow/harbor/lantern.txt", Size: 2, Unverified: true},
		{Path: "/srv/meadow/harbor/deep/fern.txt", Size: 4},
		{Path: "/srv/meadow/orchard/north/east/pebble.txt", Size: 8},
	}}
	rows := UniqueRows(st)
	want := []string{"harbor", "deep", "fern.txt", "lantern.txt", "orchard/north/east", "pebble.txt", "clover.txt"}
	if got := uniqueDisplays(rows); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	harbor := rows[0].Value
	if !harbor.IsDir || harbor.Path != "/srv/meadow/harbor" || harbor.Count != 2 || harbor.Size != 6 || harbor.Unverified != 1 {
		t.Fatalf("harbor = %+v", harbor)
	}
	// A combined row stands for the deepest directory.
	if d := rows[4].Value; d.Path != "/srv/meadow/orchard/north/east" || d.Count != 1 {
		t.Fatalf("combined = %+v", d)
	}
	st.Pane.SetCollapsed("/srv/meadow/harbor", true)
	if got := uniqueDisplays(UniqueRows(st)); !slices.Equal(got, []string{"harbor", "orchard/north/east", "pebble.txt", "clover.txt"}) {
		t.Fatalf("collapsed = %v", got)
	}
}

// A selected directory off Root still gets real paths on its directory rows.
func TestUniqueRowsOffRoot(t *testing.T) {
	st := &UniqueViewState{Root: "/srv/meadow", Files: []UniqueFile{{Path: "/data/willow/reed.txt"}}}
	rows := UniqueRows(st)
	if len(rows) != 2 || rows[0].Value.Path != "/data/willow" || rows[0].Value.Display != "data/willow" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestUniqueHideHiddenAndFilesUnder(t *testing.T) {
	st := &UniqueViewState{Root: "/srv/meadow", Files: []UniqueFile{
		{Path: "/srv/meadow/.nook/fern.txt"},
		{Path: "/srv/meadow/harbor/.reed"},
		{Path: "/srv/meadow/harbor/lantern.txt"},
		{Path: "/srv/meadow/harbor-north/moss.txt"},
	}}
	if n := len(st.FilesUnder("/srv/meadow/harbor")); n != 2 {
		t.Fatalf("FilesUnder = %d (a sibling sharing the prefix is not under it)", n)
	}
	st.HideHidden = true
	if got := st.VisibleFiles(); len(got) != 2 || len(st.Files) != 4 {
		t.Fatalf("visible = %v files = %d", got, len(st.Files))
	}
	if n := len(st.FilesUnder("/srv/meadow/harbor")); n != 1 {
		t.Fatalf("FilesUnder hidden off = %d", n)
	}
}

func TestUniqueMarkAccounting(t *testing.T) {
	st := &UniqueViewState{Root: "/srv/meadow"}
	f := UniqueFile{Path: "/srv/meadow/harbor/deep/fern.txt", Size: 7}
	st.SetMark(f, true)
	st.SetMark(f, true)
	if st.MarkedBytes != 7 || len(st.Marked) != 1 {
		t.Fatalf("bytes = %d marked = %v", st.MarkedBytes, st.Marked)
	}
	under := st.markedUnderDirs()
	if under["/srv/meadow/harbor"] != 1 || under["/srv/meadow/harbor/deep"] != 1 || under["/srv/meadow"] != 0 {
		t.Fatalf("under = %v", under)
	}
	st.SetMark(f, false)
	st.SetMark(f, false)
	if st.MarkedBytes != 0 || len(st.Marked) != 0 {
		t.Fatalf("bytes = %d marked = %v", st.MarkedBytes, st.Marked)
	}
}

func TestUniqueRectsFollowActivePanel(t *testing.T) {
	l := Layout{Primary: Rect{X: 0, Width: 40}, Secondary: Rect{X: 40, Width: 40}}
	if tree, dest := UniqueRects(l, PrimaryPanel); tree != l.Primary || dest != l.Secondary {
		t.Fatalf("primary active: %v %v", tree, dest)
	}
	if tree, dest := UniqueRects(l, SecondaryPanel); tree != l.Secondary || dest != l.Primary {
		t.Fatalf("secondary active: %v %v", tree, dest)
	}
}
