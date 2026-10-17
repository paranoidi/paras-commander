package panel

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func rangeTestState(names ...string) *State {
	s := &State{}
	for _, n := range names {
		s.Entries = append(s.Entries, localfs.Entry{Name: n, Path: filepath.Join("/tmp", n)})
	}
	return s
}

func TestMarkRange(t *testing.T) {
	names := []string{"amber", "birch", "cedar", "dune", "ember", "frost"}
	for _, tc := range []struct {
		name           string
		anchor, cursor int
	}{
		{"forward", 1, 4}, {"reverse", 4, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rangeTestState(names...)
			s.Cursor = tc.anchor
			if ok, added, _ := s.MarkRange(10); !ok || added != 0 || s.RangeAnchor == "" {
				t.Fatalf("first press: ok=%v added=%d anchor=%q", ok, added, s.RangeAnchor)
			}
			if s.Cursor != tc.anchor+1 {
				t.Fatalf("first press cursor=%d, want %d", s.Cursor, tc.anchor+1)
			}
			s.Cursor = tc.cursor
			if _, added, _ := s.MarkRange(10); added != 4 {
				t.Fatalf("added=%d, want 4", added)
			}
			for i, n := range names {
				if got, want := s.IsSelectedPath(filepath.Join("/tmp", n)), i >= 1 && i <= 4; got != want {
					t.Fatalf("%s selected=%v want %v", n, got, want)
				}
			}
			if s.RangeAnchor != "" {
				t.Fatal("anchor not cleared")
			}
		})
	}
}

func TestMarkRangeStaleAnchorBecomesFresh(t *testing.T) {
	s := rangeTestState("amber", "birch", "cedar")
	s.RangeAnchor = "/tmp/gone"
	s.Cursor = 2
	if _, added, _ := s.MarkRange(10); added != 0 || s.RangeAnchor != "/tmp/cedar" {
		t.Fatalf("added=%d anchor=%q", added, s.RangeAnchor)
	}
	if s.IsSelectedPath("/tmp/cedar") {
		t.Fatal("fresh anchor must not select")
	}
}
