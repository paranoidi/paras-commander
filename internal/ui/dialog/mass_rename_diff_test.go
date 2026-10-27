package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestMassRenameDiffIdentical(t *testing.T) {
	removed, replaced, added := MassRenameDiff("alpha_beta.txt", "alpha_beta.txt")
	if len(removed) != 0 || len(replaced) != 0 || len(added) != 0 {
		t.Fatalf("identical: removed=%v replaced=%v added=%v", removed, replaced, added)
	}
}

func TestMassRenameDiffSimpleReplace(t *testing.T) {
	removed, replaced, added := MassRenameDiff("foo_a.txt", "bar_a.txt")
	wantReplaced := []search.Range{{Start: 0, End: 3}}
	wantAdded := []search.Range{{Start: 0, End: 3}}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
	if !rangesEqual(replaced, wantReplaced) {
		t.Fatalf("replaced = %v, want %v", replaced, wantReplaced)
	}
	if !rangesEqual(added, wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
}

func TestMassRenameDiffInsertSuffix(t *testing.T) {
	removed, replaced, added := MassRenameDiff("draft", "draft_final")
	if len(removed) != 0 || len(replaced) != 0 {
		t.Fatalf("removed = %v replaced = %v, want none", removed, replaced)
	}
	wantAdded := []search.Range{{Start: 5, End: 11}}
	if !rangesEqual(added, wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
}

func TestMassRenameDiffDeletePrefix(t *testing.T) {
	removed, replaced, added := MassRenameDiff("old_name", "name")
	wantRemoved := []search.Range{{Start: 0, End: 4}}
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if len(replaced) != 0 || len(added) != 0 {
		t.Fatalf("replaced = %v added = %v, want none", replaced, added)
	}
}

func TestMassRenameDiffDeleteScattered(t *testing.T) {
	old := "45.Years.2015.LIMITED.1080p.BluRay.X264-XXX"
	new := "5.Years.2015.LIMITED.1080p.BluRay.X26-XXX"
	removed, replaced, added := MassRenameDiff(old, new)
	wantRemoved := runeRangesOf(old, '4')
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if len(replaced) != 0 || len(added) != 0 {
		t.Fatalf("replaced = %v added = %v, want none", replaced, added)
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
	removed, replaced, added := MassRenameDiff("before", "after")
	// LCS keeps shared runes (e, r); only unmatched segments are highlighted. "b" sits opposite
	// "aft" so it is replaced; "fo" and the final "e" have nothing in their place.
	wantRemoved := []search.Range{{Start: 2, End: 4}, {Start: 5, End: 6}}
	wantReplaced := []search.Range{{Start: 0, End: 1}}
	wantAdded := []search.Range{{Start: 0, End: 3}}
	if !rangesEqual(replaced, wantReplaced) {
		t.Fatalf("replaced = %v, want %v", replaced, wantReplaced)
	}
	if !rangesEqual(removed, wantRemoved) {
		t.Fatalf("removed = %v, want %v", removed, wantRemoved)
	}
	if !rangesEqual(added, wantAdded) {
		t.Fatalf("added = %v, want %v", added, wantAdded)
	}
}

func TestMassRenameDiffCaseChange(t *testing.T) {
	removed, replaced, added := MassRenameDiff("quiet river", "Quiet River")
	want := []search.Range{{Start: 0, End: 1}, {Start: 6, End: 7}}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
	if !rangesEqual(replaced, want) {
		t.Fatalf("replaced = %v, want %v", replaced, want)
	}
	if !rangesEqual(added, want) {
		t.Fatalf("added = %v, want %v", added, want)
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
