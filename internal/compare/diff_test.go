package compare

import "testing"

func TestClassifyEqualSamePath(t *testing.T) {
	primary := []FileRecord{{Rel: "a.txt", Size: 10}}
	secondary := []FileRecord{{Rel: "a.txt", Size: 10}}
	h := [32]byte{1}
	rows := Classify(primary, secondary, map[string][32]byte{"a.txt": h}, map[string][32]byte{"a.txt": h}, nil, nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Kind != KindEqual {
		t.Fatalf("kind = %v, want Equal", rows[0].Kind)
	}
}

func TestClassifyContentDiffBySize(t *testing.T) {
	primary := []FileRecord{{Rel: "a.txt", Size: 10}}
	secondary := []FileRecord{{Rel: "a.txt", Size: 20}}
	rows := Classify(primary, secondary, nil, nil, nil, nil)
	if len(rows) != 1 || rows[0].Kind != KindContentDiff {
		t.Fatalf("rows = %+v, want content diff", rows)
	}
}

func TestClassifyRelocated(t *testing.T) {
	h := [32]byte{42}
	primary := []FileRecord{{Rel: "old/name.txt", Size: 5}}
	secondary := []FileRecord{{Rel: "new/name.txt", Size: 5}}
	rows := Classify(primary, secondary,
		map[string][32]byte{"old/name.txt": h},
		map[string][32]byte{"new/name.txt": h},
		nil, nil,
	)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Kind != KindRelocated {
		t.Fatalf("kind = %v, want Relocated", rows[0].Kind)
	}
	if rows[0].PrimaryRel != "old/name.txt" || rows[0].SecondaryRel != "new/name.txt" {
		t.Fatalf("paths = %+v", rows[0])
	}
}

func TestClassifyPrimaryOnlySecondaryOnly(t *testing.T) {
	hP := [32]byte{7}
	hS := [32]byte{8}
	primary := []FileRecord{{Rel: "only-p.txt", Size: 1}}
	secondary := []FileRecord{{Rel: "only-s.txt", Size: 2}}
	rows := Classify(primary, secondary,
		map[string][32]byte{"only-p.txt": hP},
		map[string][32]byte{"only-s.txt": hS},
		nil, nil,
	)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	kinds := map[Kind]int{}
	for _, r := range rows {
		kinds[r.Kind]++
	}
	if kinds[KindPrimaryOnly] != 1 || kinds[KindSecondaryOnly] != 1 {
		t.Fatalf("kinds = %v", kinds)
	}
}

func TestFilterMatches(t *testing.T) {
	if !FilterMatches(FilterRelocated, KindRelocated) {
		t.Fatal("relocated filter should match relocated kind")
	}
	if FilterMatches(FilterEqual, KindPrimaryOnly) {
		t.Fatal("equal filter should not match primary only")
	}
}
