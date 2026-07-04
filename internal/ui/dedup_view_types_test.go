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
	if got := DedupEntriesFromSnapshot(snap); got != nil {
		t.Fatalf("hashing phase list = %v, want nil", got)
	}

	snap.Phase = comparepkg.DedupDone
	list := DedupEntriesFromSnapshot(snap)
	if len(list) != 1 {
		t.Fatalf("done list len = %d, want 1", len(list))
	}
	if !list[0].GroupFirst || list[0].AbsKey != "/root/a" || list[0].Copies != 1 {
		t.Fatalf("entry = %+v, unexpected fields", list[0])
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
