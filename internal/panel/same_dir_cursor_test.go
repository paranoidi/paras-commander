package panel

import (
	"os"
	"path/filepath"
	"testing"
)

// A same-directory load with no name to select (NavigateTo to the current directory, as nav.home
// does when already home) keeps the cursor on its own entry, and only moves on when it is gone.
func TestNavigateToSameDirKeepsCursorEntry(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"cherry", "damson", "elder", "fig"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	newState := func(t *testing.T, cursorName string) *State {
		t.Helper()
		s, err := New(root)
		if err != nil {
			t.Fatal(err)
		}
		if cursorName != "" && !s.SelectVisibleEntry(cursorName) {
			t.Fatalf("%s not listed", cursorName)
		}
		return &s
	}
	removeUntilCleanup := func(t *testing.T, name string) {
		t.Helper()
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644) })
	}
	cursorName := func(t *testing.T, s *State) string {
		t.Helper()
		e, ok := s.CurrentEntry()
		if !ok {
			t.Fatal("no cursor entry")
		}
		return e.Name
	}

	t.Run("first row", func(t *testing.T) {
		s := newState(t, "")
		s.Cursor = 0
		want := cursorName(t, s)
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if s.Cursor != 0 || cursorName(t, s) != want {
			t.Fatalf("cursor = %d on %q, want 0 on %q", s.Cursor, cursorName(t, s), want)
		}
	})
	t.Run("middle row", func(t *testing.T) {
		s := newState(t, "damson")
		row := s.Cursor
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if s.Cursor != row || cursorName(t, s) != "damson" {
			t.Fatalf("cursor = %d on %q, want %d on damson", s.Cursor, cursorName(t, s), row)
		}
	})
	t.Run("last row", func(t *testing.T) {
		s := newState(t, "fig")
		row := s.Cursor
		if row != s.VisibleEntryCount()-1 {
			t.Fatalf("fig is row %d of %d, want the last", row, s.VisibleEntryCount())
		}
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if s.Cursor != row || cursorName(t, s) != "fig" {
			t.Fatalf("cursor = %d on %q, want %d on fig", s.Cursor, cursorName(t, s), row)
		}
	})
	t.Run("cursor file deleted moves to the next neighbour", func(t *testing.T) {
		s := newState(t, "damson")
		if err := os.Remove(filepath.Join(root, "damson")); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.WriteFile(filepath.Join(root, "damson"), []byte("x"), 0o644) })
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if got := cursorName(t, s); got != "elder" {
			t.Fatalf("cursor on %q, want elder", got)
		}
	})
	t.Run("refresh after the last file is deleted lands on the new last row", func(t *testing.T) {
		s := newState(t, "fig")
		removeUntilCleanup(t, "fig")
		if err := s.Refresh(10); err != nil {
			t.Fatal(err)
		}
		if got := cursorName(t, s); got != "elder" {
			t.Fatalf("cursor on %q, want elder (the row that is now last)", got)
		}
	})
	t.Run("last file deleted lands on the new last row", func(t *testing.T) {
		s := newState(t, "fig")
		removeUntilCleanup(t, "fig")
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if got := cursorName(t, s); got != "elder" {
			t.Fatalf("cursor on %q, want elder (the row that is now last)", got)
		}
	})
	t.Run("last two files deleted land on the new last row", func(t *testing.T) {
		s := newState(t, "fig")
		removeUntilCleanup(t, "fig")
		removeUntilCleanup(t, "elder")
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if got := cursorName(t, s); got != "damson" {
			t.Fatalf("cursor on %q, want damson (the row that is now last)", got)
		}
	})
	t.Run("last file deleted with a filter active keeps a valid cursor", func(t *testing.T) {
		s := newState(t, "")
		s.OpenFilter(10)
		s.AppendFilterRune('e', 10)
		if !s.SelectVisibleEntry("elder") {
			t.Fatal("elder not visible")
		}
		removeUntilCleanup(t, "elder")
		if err := s.NavigateTo(root, "", 10); err != nil {
			t.Fatal(err)
		}
		if n := s.VisibleEntryCount(); s.Cursor < 0 || s.Cursor >= n {
			t.Fatalf("cursor %d out of range of %d visible rows", s.Cursor, n)
		}
	})
}

// A same-directory load of a directory whose files are all gone leaves a valid empty state.
func TestNavigateToSameDirAllFilesDeleted(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"quince", "rowan"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SelectVisibleEntry("rowan") {
		t.Fatal("rowan not listed")
	}
	for _, name := range []string{"quince", "rowan"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.NavigateTo(root, "", 10); err != nil {
		t.Fatal(err)
	}
	n := s.VisibleEntryCount()
	if s.Cursor < 0 || (n == 0 && s.Cursor != 0) || (n > 0 && s.Cursor >= n) {
		t.Fatalf("cursor %d invalid for %d visible rows", s.Cursor, n)
	}
	if n > 0 {
		if _, ok := s.CurrentEntry(); !ok {
			t.Fatal("cursor row has no entry")
		}
	}
}
