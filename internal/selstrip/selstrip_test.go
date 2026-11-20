package selstrip

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestClampKeepsCursorVisible(t *testing.T) {
	var s State
	s.Move(7, 10, 3)
	if s.Cursor != 7 || s.Scroll != 5 {
		t.Fatalf("after move: cursor %d scroll %d, want 7 / 5", s.Cursor, s.Scroll)
	}
	s.Move(99, 10, 3)
	if s.Cursor != 9 || s.Scroll != 7 {
		t.Fatalf("after overshoot: cursor %d scroll %d, want 9 / 7", s.Cursor, s.Scroll)
	}
	s.Top(10, 3)
	if s.Cursor != 0 || s.Scroll != 0 {
		t.Fatalf("after top: cursor %d scroll %d", s.Cursor, s.Scroll)
	}
	s.Bottom(10, 3)
	if s.Cursor != 9 || s.Scroll != 7 {
		t.Fatalf("after bottom: cursor %d scroll %d", s.Cursor, s.Scroll)
	}
	s.Move(1, 0, 3)
	if s.Cursor != 0 || s.Scroll != 0 {
		t.Fatalf("empty list must reset: cursor %d scroll %d", s.Cursor, s.Scroll)
	}
}

func TestRememberRestore(t *testing.T) {
	paths := Rows([]string{"/tmp/meadow/bravo.txt", "/tmp/meadow/crystal.txt"}, "/tmp", false, nil)
	s := State{Cursor: 1}
	s.Remember(paths)
	s.Cursor = 0
	s.Restore(paths)
	if s.Cursor != 1 {
		t.Fatalf("restored cursor %d, want 1", s.Cursor)
	}
	s.CursorPath = "/tmp/gone.txt"
	s.Restore(paths)
	if s.Cursor != 1 {
		t.Fatalf("missing path moved cursor to %d", s.Cursor)
	}
}

func TestMapBasenameRangesToDisplay(t *testing.T) {
	display := "meadow/crystal.txt"
	abs := "/tmp/meadow/crystal.txt"
	mapped := MapBasenameRangesToDisplay(display, abs, []search.Range{{Start: 0, End: 3}})
	if len(mapped) != 1 {
		t.Fatalf("mapped = %v, want one range", mapped)
	}
	if mapped[0].Start != len([]rune("meadow/")) || mapped[0].End != len([]rune("meadow/"))+3 {
		t.Fatalf("mapped[0] = %+v, want offset onto basename", mapped[0])
	}
}

func TestRowsTreeFoldsChain(t *testing.T) {
	rows := Rows([]string{"/tmp/meadow/foo/bar/asdf/crystal.txt"}, "/tmp/meadow", true, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want folder + file", rows)
	}
	if !rows[0].Group || rows[0].Display != "foo/bar/asdf" || rows[0].Path != "/tmp/meadow/foo/bar/asdf" || rows[0].Depth != 0 {
		t.Fatalf("group row = %+v", rows[0])
	}
	if rows[1].Group || rows[1].Display != "crystal.txt" || rows[1].Depth != 1 || !rows[1].LastChild {
		t.Fatalf("file row = %+v", rows[1])
	}
	if flat := Rows([]string{"/tmp/meadow/foo/bar/asdf/crystal.txt"}, "/tmp/meadow", false, nil); len(flat) != 1 || flat[0].Group {
		t.Fatalf("flat rows = %+v", flat)
	}
}

func TestCollapseExpand(t *testing.T) {
	paths := []string{"/tmp/meadow/lantern/bravo.txt", "/tmp/meadow/lantern/crystal.txt", "/tmp/meadow/willow.txt"}
	var s State
	rows := Rows(paths, "/tmp/meadow", true, s.Collapsed)
	if len(rows) != 4 || !rows[0].Group || !rows[0].Expanded {
		t.Fatalf("rows = %+v, want open folder lantern + 2 files + willow", rows)
	}
	s.Cursor = 2 // crystal.txt: Collapse jumps to the parent folder
	if s.Collapse(rows) || s.Cursor != 0 {
		t.Fatalf("collapse on file: cursor = %d, want 0 and no row change", s.Cursor)
	}
	if !s.Collapse(rows) {
		t.Fatal("collapse on open folder reported no change")
	}
	rows = Rows(paths, "/tmp/meadow", true, s.Collapsed)
	if len(rows) != 2 || rows[0].Expanded {
		t.Fatalf("collapsed rows = %+v, want closed lantern + willow", rows)
	}
	if !s.Expand(rows) || len(Rows(paths, "/tmp/meadow", true, s.Collapsed)) != 4 {
		t.Fatal("expand did not reopen lantern")
	}
}

func TestBulkFold(t *testing.T) {
	paths := []string{
		"/tmp/meadow/lantern/ember/bravo.txt", "/tmp/meadow/lantern/ember/crystal.txt",
		"/tmp/meadow/lantern/delta.txt", "/tmp/meadow/willow.txt",
	}
	base := "/tmp/meadow"
	var s State
	rows := func() []Row { return Rows(paths, base, true, s.Collapsed) }
	names := func() []string {
		var out []string
		for _, r := range rows() {
			out = append(out, r.Display)
		}
		return out
	}
	// lantern, ember, bravo, crystal, delta, willow
	s.Cursor = 3 // crystal.txt
	s.CollapseLevel(rows())
	s.Restore(rows())
	if got := strings.Join(names(), ","); got != "lantern,ember,delta.txt,willow.txt" || s.Cursor != 1 {
		t.Fatalf("collapse level: %s cursor %d, want ember folded and cursor on it", got, s.Cursor)
	}
	s.CollapseAll(NewTree(paths, base).Nodes(), rows())
	s.Restore(rows())
	if got := strings.Join(names(), ","); got != "lantern,willow.txt" || s.Cursor != 0 {
		t.Fatalf("collapse all: %s cursor %d", got, s.Cursor)
	}
	s.ExpandLevel(rows())
	if got := strings.Join(names(), ","); got != "lantern,ember,delta.txt,willow.txt" {
		t.Fatalf("expand level: %s", got)
	}
	s.ExpandAll(rows())
	s.Restore(rows())
	if len(rows()) != 6 || s.Cursor != 0 {
		t.Fatalf("expand all: %d rows cursor %d", len(rows()), s.Cursor)
	}
	flat := Rows(paths, base, false, nil)
	s.CollapseAll(NewTree(paths, base).Nodes(), flat)
	if len(s.Collapsed) != 0 {
		t.Fatal("bulk fold must ignore a flat list")
	}
}
