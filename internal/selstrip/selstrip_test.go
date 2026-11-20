package selstrip

import (
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
	paths := []string{"/tmp/meadow/bravo.txt", "/tmp/meadow/crystal.txt"}
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
