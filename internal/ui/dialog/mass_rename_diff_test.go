package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestMassRenameDiffIdentical(t *testing.T) {
	removed, added := MassRenameDiff("alpha_beta.txt", "alpha_beta.txt")
	if len(removed) != 0 || len(added) != 0 {
		t.Fatalf("identical: removed=%v added=%v", removed, added)
	}
}

func TestMassRenameDiffSimpleReplace(t *testing.T) {
	removed, added := MassRenameDiff("foo_a.txt", "bar_a.txt")
	wantRemoved := []search.Range{{Start: 0, End: 3}}
	wantAdded := []search.Range{{Start: 0, End: 3}}
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if !rangesEqual(added, wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
}

func TestMassRenameDiffInsertSuffix(t *testing.T) {
	removed, added := MassRenameDiff("draft", "draft_final")
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
	wantAdded := []search.Range{{Start: 5, End: 11}}
	if !rangesEqual(added, wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
}

func TestMassRenameDiffDeletePrefix(t *testing.T) {
	removed, added := MassRenameDiff("old_name", "name")
	wantRemoved := []search.Range{{Start: 0, End: 4}}
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if len(added) != 0 {
		t.Fatalf("added = %v, want none", added)
	}
}

func TestMassRenameDiffDeleteScattered(t *testing.T) {
	old := "45.Years.2015.LIMITED.1080p.BluRay.X264-XXX"
	new := "5.Years.2015.LIMITED.1080p.BluRay.X26-XXX"
	removed, added := MassRenameDiff(old, new)
	wantRemoved := runeRangesOf(old, '4')
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if len(added) != 0 {
		t.Fatalf("added = %v, want none", added)
	}
}

func runeRangesOf(s string, r rune) []search.Range {
	rs := []rune(s)
	var out []search.Range
	for i, c := range rs {
		if c != r {
			continue
		}
		if n := len(out); n > 0 && out[n-1].End == i {
			out[n-1].End = i + 1
		} else {
			out = append(out, search.Range{Start: i, End: i + 1})
		}
	}
	return out
}

func TestMassRenameDiffFullReplace(t *testing.T) {
	removed, added := MassRenameDiff("before", "after")
	// LCS keeps shared runes (f, e, r); only unmatched segments are highlighted.
	wantRemoved := []search.Range{{Start: 0, End: 1}, {Start: 2, End: 4}, {Start: 5, End: 6}}
	wantAdded := []search.Range{{Start: 0, End: 3}}
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if !rangesEqual(added, wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
}

func rangesEqual(a, b []search.Range) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
