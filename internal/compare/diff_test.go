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

func TestClassifyUniqueSizePrimaryOnlyIsHashDone(t *testing.T) {
	// 0-byte sole-side file: never hashed, must not stay pending.
	primary := []FileRecord{{Rel: "empty.txt", Size: 0}}
	secondary := []FileRecord{{Rel: "other.txt", Size: 10}}
	rows := Classify(primary, secondary, nil, nil, nil, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Kind == KindPrimaryOnly && r.PrimaryRel == "empty.txt" {
			if !r.HashDone {
				t.Fatal("unique-size primary-only empty file must be HashDone (no pending glyph)")
			}
			return
		}
	}
	t.Fatal("missing primary-only empty.txt row")
}

func TestClassifyRelocatedCandidatesStayPendingUntilHashed(t *testing.T) {
	primary := []FileRecord{{Rel: "old/a.txt", Size: 5}}
	secondary := []FileRecord{{Rel: "new/b.txt", Size: 5}}
	rows := Classify(primary, secondary, nil, nil, nil, nil)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.HashDone {
			t.Fatalf("same-size unpaired row %+v should be pending until hashed", r)
		}
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

func TestFilteredRowsIgnoreEmpty(t *testing.T) {
	snap := Snapshot{
		Rows: []Row{
			{Kind: KindPrimaryOnly, PrimaryRel: "empty.txt", Size: 0, HashDone: true},
			{Kind: KindPrimaryOnly, PrimaryRel: "data.bin", Size: 100, HashDone: true},
			{Kind: KindEqual, PrimaryRel: "both-empty", SecondaryRel: "both-empty", Size: 0, HashDone: true},
		},
	}
	got := FilteredRows(snap, FilterAll, true)
	if len(got) != 1 || got[0].PrimaryRel != "data.bin" {
		t.Fatalf("ignoreEmpty rows = %+v, want only data.bin", got)
	}
	got = FilteredRows(snap, FilterAll, false)
	if len(got) != 3 {
		t.Fatalf("show empty rows = %d, want 3", len(got))
	}
}

func TestEndLabelEmptyHidden(t *testing.T) {
	snap := Snapshot{Rows: []Row{
		{Kind: KindPrimaryOnly, PrimaryRel: "e", Size: 0},
		{Kind: KindSecondaryOnly, SecondaryRel: "f", Size: 0},
	}}
	if got := EndLabel(FilterAll, true, snap); got != "All · 2 empty hidden" {
		t.Fatalf("EndLabel = %q, want All · 2 empty hidden", got)
	}
	if got := EndLabel(FilterAll, false, snap); got != "All" {
		t.Fatalf("EndLabel show empty = %q, want All", got)
	}
	if got := EndLabel(FilterAll, true, Snapshot{}); got != "All" {
		t.Fatalf("EndLabel no empties = %q, want All", got)
	}
}

func TestMarkHashingSetsActiveOnly(t *testing.T) {
	rows := []Row{
		{Kind: KindEqual, PrimaryRel: "shared.txt", SecondaryRel: "shared.txt"},
		{Kind: KindPrimaryOnly, PrimaryRel: "solo.txt"},
		{Kind: KindEqual, PrimaryRel: "done.txt", SecondaryRel: "done.txt", HashDone: true},
	}
	hashingP := map[string]struct{}{"shared.txt": {}}
	got := MarkHashing(append([]Row(nil), rows...), hashingP, nil)
	if !got[0].Hashing {
		t.Fatal("shared.txt should be Hashing while primary side is in flight")
	}
	if got[1].Hashing {
		t.Fatal("solo.txt queued pending must not be Hashing")
	}
	if got[2].Hashing {
		t.Fatal("HashDone row must not be Hashing")
	}

	got = MarkHashing(got, nil, nil)
	if got[0].Hashing {
		t.Fatal("Hashing must clear when no workers are active")
	}
}
