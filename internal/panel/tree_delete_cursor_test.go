package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// setupTreeWithTwoExpandedDirs builds root/{alpha,beta}/{notes,zeta} in tree mode with both
// expanded and the cursor on cursorDir/notes.
func setupTreeWithTwoExpandedDirs(t *testing.T, cursorDir string) (s *State, root string) {
	t.Helper()
	root = t.TempDir()
	for _, d := range []string{"alpha/notes", "alpha/zeta", "beta/notes", "beta/zeta"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s = &st
	s.SetListLayout(ListLayoutTree, 20)
	for _, name := range []string{"beta", "alpha"} {
		s.selectVisibleEntryByPath(filepath.Join(root, name))
		if err := s.ExpandTreeCursorRow(20); err != nil {
			t.Fatal(err)
		}
	}
	s.selectVisibleEntryByPath(filepath.Join(root, cursorDir, "notes"))
	s.syncTreeCursorIDToCursor()
	return s, root
}

func treeCursorPath(t *testing.T, s *State) string {
	t.Helper()
	e, ok := s.CurrentEntry()
	if !ok {
		t.Fatal("no current entry")
	}
	return e.Path
}

// Deleting an expanded child must hand the cursor to its next neighbour, not to a same-named row
// under another directory.
func TestTreeDeleteCursorMovesToNextNeighbour(t *testing.T) {
	s, root := setupTreeWithTwoExpandedDirs(t, "alpha")
	if err := os.Remove(filepath.Join(root, "alpha", "notes")); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(20); err != nil {
		t.Fatal(err)
	}
	if got, want := treeCursorPath(t, s), filepath.Join(root, "alpha", "zeta"); got != want {
		t.Fatalf("cursor = %q, want %q", got, want)
	}
}

// With async child loads the reload first shows only top-level rows; once the expanded
// children land (in any order), the cursor must settle on the deleted row's neighbour.
func TestTreeDeleteCursorSettlesAfterAsyncChildLoads(t *testing.T) {
	s, root := setupTreeWithTwoExpandedDirs(t, "beta")
	var reqs []TreeChildLoadRequest
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool {
		reqs = append(reqs, req)
		return true
	}
	if err := os.Remove(filepath.Join(root, "beta", "notes")); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(20); err != nil {
		t.Fatal(err)
	}
	if len(reqs) == 0 {
		t.Fatal("expected async child loads for expanded dirs")
	}
	// Land the loads last-requested first so rows above the cursor shift after the reload.
	for i := len(reqs) - 1; i >= 0; i-- {
		req := reqs[i]
		des, err := os.ReadDir(req.DirID)
		if err != nil {
			t.Fatal(err)
		}
		var entries []localfs.Entry
		for _, de := range des {
			entries = append(entries, localfs.Entry{Name: de.Name(), Path: filepath.Join(req.DirID, de.Name()), Type: localfs.EntryDirectory})
		}
		s.ApplyTreeChildLoad(req.DirID, entries, nil, 20)
	}
	if got, want := treeCursorPath(t, s), filepath.Join(root, "beta", "zeta"); got != want {
		t.Fatalf("cursor = %q, want %q", got, want)
	}
}

// Re-sorting (reversed, so beta/notes comes first) must keep the cursor on its exact row, not
// the first same-named row in the tree.
func TestTreeSortChangeKeepsCursorByPath(t *testing.T) {
	s, root := setupTreeWithTwoExpandedDirs(t, "alpha")
	s.SetSortMode(s.Sort.Mode, !s.Sort.Reverse, s.Sort.DirectoriesFirst, 20)
	if got, want := treeCursorPath(t, s), filepath.Join(root, "alpha", "notes"); got != want {
		t.Fatalf("cursor = %q, want %q", got, want)
	}
}

// A bare name in tree mode means only this panel's own child, never a nested same-named row.
func TestTreeSelectVisibleEntryMatchesOnlyOwnChild(t *testing.T) {
	s, _ := setupTreeWithTwoExpandedDirs(t, "beta")
	if s.SelectVisibleEntry("notes") {
		t.Fatal(`SelectVisibleEntry("notes") matched a nested row`)
	}
}
