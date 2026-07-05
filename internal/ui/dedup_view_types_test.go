package ui

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestDedupEntriesFromSnapshotDoneOnly(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupHashing,
		Groups: []comparepkg.DedupGroup{{
			Size: 10,
			Files: []comparepkg.DedupFile{
				{Rel: "a", Abs: pathloc.MustParse("/root/a")},
			},
		}},
	}
	if got, _ := DedupEntriesFromSnapshot(snap, false, true); got != nil {
		t.Fatalf("hashing phase list = %v, want nil", got)
	}

	snap.Phase = comparepkg.DedupDone
	list, _ := DedupEntriesFromSnapshot(snap, false, true)
	if len(list) != 1 {
		t.Fatalf("done list len = %d, want 1", len(list))
	}
	if !list[0].GroupFirst || list[0].AbsKey != "/root/a" || list[0].Copies != 1 {
		t.Fatalf("entry = %+v, unexpected fields", list[0])
	}
}

func TestDedupEntriesFromSnapshotSortAndIgnoreEmpty(t *testing.T) {
	mk := func(rel string, size int64) comparepkg.DedupGroup {
		return comparepkg.DedupGroup{
			Size: size,
			Files: []comparepkg.DedupFile{
				{Rel: rel + "/copper", Abs: pathloc.MustParse("/root/" + rel + "/copper")},
				{Rel: rel + "/willow", Abs: pathloc.MustParse("/root/" + rel + "/willow")},
			},
		}
	}
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			mk("zebra", 300), // path-last, largest
			mk("apple", 100), // path-first, smallest
			mk("mango", 0),   // empty group
		},
	}

	firstRel := func(list []DedupEntry) string { return list[0].File.Rel }

	// Default: order by path, empties hidden.
	list, ignored := DedupEntriesFromSnapshot(snap, false, true)
	if ignored != 2 {
		t.Fatalf("ignoredEmpty = %d, want 2", ignored)
	}
	if len(list) != 4 { // two non-empty groups × 2 files
		t.Fatalf("list len = %d, want 4", len(list))
	}
	if got := firstRel(list); got != "apple/copper" {
		t.Fatalf("path-order first = %q, want apple/copper", got)
	}

	// Most space wasted: largest group first.
	list, _ = DedupEntriesFromSnapshot(snap, true, true)
	if got := firstRel(list); got != "zebra/copper" {
		t.Fatalf("wasted-order first = %q, want zebra/copper", got)
	}

	// Showing empties keeps all three groups and reports zero ignored.
	list, ignored = DedupEntriesFromSnapshot(snap, false, false)
	if ignored != 0 || len(list) != 6 {
		t.Fatalf("show-empty: ignored=%d len=%d, want 0 and 6", ignored, len(list))
	}
}

func TestDedupViewStateMarkedSummary(t *testing.T) {
	st := DedupViewState{MarkedCount: 0}
	if got := st.MarkedSummary(); got != "" {
		t.Fatalf("empty summary = %q, want \"\"", got)
	}
	st.MarkedCount = 2
	st.MarkedReclaimBytes = 2048
	got := st.MarkedSummary()
	if got != "2 marked · 2.0K" {
		t.Fatalf("summary = %q, want %q", got, "2 marked · 2.0K")
	}
}

func TestDedupViewStateEnsureSelectionVisibleMaxScroll(t *testing.T) {
	st := DedupViewState{Selected: 99, ListScroll: 0}
	st.EnsureSelectionVisible(100, 10)
	if st.ListScroll != 90 {
		t.Fatalf("ListScroll = %d, want 90", st.ListScroll)
	}
}

func TestDedupRedundantUnder(t *testing.T) {
	mk := func(rel string) DedupEntry {
		e := DedupEntry{File: comparepkg.DedupFile{Rel: rel, Abs: pathloc.MustParse("/root/" + rel)}, Size: 10}
		e.AbsKey = e.File.Abs.String()
		return e
	}
	// g1: one copy under backup/, one outside → the backup copy is redundant.
	// g2: both copies under backup/ → keep the first, drop the second.
	// g3: entirely outside backup/ → untouched.
	list := []DedupEntry{
		func() DedupEntry { e := mk("backup/copper"); e.GroupFirst = true; return e }(),
		mk("keep/copper"),
		func() DedupEntry { e := mk("backup/inner/willow"); e.GroupFirst = true; return e }(),
		mk("backup/other/willow"),
		func() DedupEntry { e := mk("keep/maple"); e.GroupFirst = true; return e }(),
		mk("elsewhere/maple"),
	}

	// Selected row lives under /root/backup/... → its directory is /root/backup.
	// (Selected = 2, whose parent is /root/backup/inner; prefix match covers the subtree.)
	got := DedupRedundantUnder(list, 0) // selected /root/backup/copper → dir /root/backup
	want := map[string]bool{"/root/backup/copper": true, "/root/backup/other/willow": true}
	if len(got) != len(want) {
		t.Fatalf("marked %v, want keys %v", got, want)
	}
	for _, k := range got {
		if !want[k] {
			t.Fatalf("unexpected mark %q (all: %v)", k, got)
		}
	}
	// The survivor of g2 (first under-dir copy) must NOT be marked.
	for _, k := range got {
		if k == "/root/backup/inner/willow" {
			t.Fatal("survivor of fully-inside group was marked")
		}
	}
}

func TestDedupGroupFullyMarked(t *testing.T) {
	list := []DedupEntry{
		{AbsKey: "/g1/a", GroupFirst: true, Copies: 2},
		{AbsKey: "/g1/b", Copies: 2},
		{AbsKey: "/g2/a", GroupFirst: true, Copies: 3},
		{AbsKey: "/g2/b", Copies: 3},
		{AbsKey: "/g2/c", Copies: 3},
	}

	if DedupGroupFullyMarked(list, nil, 0) {
		t.Fatal("nil marked map: want false")
	}
	if DedupGroupFullyMarked(list, map[string]bool{"/g1/a": true}, 0) {
		t.Fatal("partial group 1: want false")
	}
	if !DedupGroupFullyMarked(list, map[string]bool{"/g1/a": true, "/g1/b": true}, 1) {
		t.Fatal("full group 1 at second row: want true")
	}
	if DedupGroupFullyMarked(list, map[string]bool{"/g2/a": true, "/g2/b": true}, 2) {
		t.Fatal("partial group 2: want false")
	}
	marked := map[string]bool{"/g2/a": true, "/g2/b": true, "/g2/c": true}
	if !DedupGroupFullyMarked(list, marked, 3) {
		t.Fatal("full group 2 at last row: want true")
	}
}
