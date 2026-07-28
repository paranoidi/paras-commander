package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestShortcutIDsMinWidthTwo(t *testing.T) {
	ids := ShortcutIDs(3)
	if len(ids) != 3 {
		t.Fatalf("len = %d, want 3", len(ids))
	}
	want := []string{"aa", "ab", "ac"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestShortcutIDsUniqueAndWiderWhenNeeded(t *testing.T) {
	n := 26*26 + 1
	ids := ShortcutIDs(n)
	if len(ids) != n {
		t.Fatalf("len = %d, want %d", len(ids), n)
	}
	if len(ids[0]) != 3 {
		t.Fatalf("width = %d, want 3", len(ids[0]))
	}
	seen := make(map[string]struct{}, n)
	for _, id := range ids {
		if len(id) != 3 {
			t.Fatalf("id %q width = %d, want 3", id, len(id))
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestPreferExactShortcutID(t *testing.T) {
	ids := []string{"aa", "ab", "ac"}
	ranked := []int{2, 0, 1}
	ranges := make([][]search.Range, 3)
	got, gotRanges := PreferExactShortcutID(ids, "ab", ranked, ranges, true)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("ranked = %v, want [1]", got)
	}
	if len(gotRanges[1]) != 1 || gotRanges[1][0].End != 2 {
		t.Fatalf("ranges[1] = %v, want End=2", gotRanges[1])
	}
	got, _ = PreferExactShortcutID(ids, "AB", ranked, ranges, false)
	if len(got) != 3 {
		t.Fatalf("case-sensitive miss: ranked = %v, want unchanged", got)
	}
	got, _ = PreferExactShortcutID(ids, "path", ranked, ranges, true)
	if len(got) != 3 {
		t.Fatalf("non-id query: ranked = %v, want unchanged", got)
	}
}
