package panel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestFetchListingRunsIndexOverlayOnlyWhenWithIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "harbor.txt"), []byte("anchor"), 0o644); err != nil {
		t.Fatal(err)
	}
	loc, _ := pathloc.File(dir)
	ghost, _ := pathloc.FileIndex(filepath.Join(dir, ".quiet.txt"))
	calls := 0
	s := State{IndexOverlay: func(_ context.Context, _ pathloc.Path, in []fsbackend.Entry) ([]fsbackend.Entry, error) {
		calls++
		for i := range in {
			in[i].IndexStatus = localfs.IndexStatusIndexed
		}
		return append(in, fsbackend.Entry{Name: ".quiet.txt", Loc: ghost, IndexStatus: localfs.IndexStatusMissing}), nil
	}}

	if _, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(loc, 0)); err != nil || calls != 0 {
		t.Fatalf("overlay ran for a panel without WithIndex: calls=%d err=%v", calls, err)
	}
	s.WithIndex = true
	entries, _, _, _, err := FetchListing(context.Background(), s.ListingRefreshSnapshot(loc, 0))
	if err != nil || calls != 1 {
		t.Fatalf("overlay calls=%d err=%v, want one call", calls, err)
	}
	if len(entries) != 1 || entries[0].IndexStatus != localfs.IndexStatusIndexed {
		t.Fatalf("entries = %+v, want the status set and the hidden ghost filtered out", entries)
	}
}

func TestApplyIndexDriftMarksOnlyIndexedDirs(t *testing.T) {
	s := State{Entries: []localfs.Entry{
		{Name: "orchard", Path: "/vol/orchard", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "pasture", Path: "/vol/pasture", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusNew},
		{Name: "marsh", Path: "/vol/marsh", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusTagged},
		{Name: "reed.txt", Path: "/vol/reed.txt", Type: localfs.EntryFile, IndexStatus: localfs.IndexStatusIndexed},
	}}
	s.ApplyIndexDrift(map[string]string{"orchard": localfs.IndexStatusDiffers, "pasture": localfs.IndexStatusDiffers, "marsh": localfs.IndexStatusDiffers, "reed.txt": localfs.IndexStatusDiffers})
	var got []string
	for _, e := range s.Entries {
		got = append(got, e.IndexStatus)
	}
	want := []string{localfs.IndexStatusDiffers, localfs.IndexStatusNew, localfs.IndexStatusDiffers, localfs.IndexStatusIndexed}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("statuses = %v, want %v", got, want)
		}
	}
}

func TestRequestIndexDriftWantsNamedIndexedDirsOnly(t *testing.T) {
	root, _ := pathloc.File("/vol")
	var req IndexDriftRequest
	s := State{Path: root, WithIndex: true, ScheduleIndexDrift: func(r IndexDriftRequest) bool { req = r; return true }, Entries: []localfs.Entry{
		{Name: "orchard", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "pasture", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusNew},
		{Name: "reed.txt", Type: localfs.EntryFile, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "thicket", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusTagged},
	}}
	s.RequestIndexDrift("orchard", "pasture", "reed.txt")
	if len(req.Subdirs) != 1 || req.Subdirs[0] != "orchard" {
		t.Fatalf("drift request = %+v, want only orchard", req)
	}
}

func TestApplyIndexDriftUncheckedLifecycle(t *testing.T) {
	s := State{Entries: []localfs.Entry{
		{Name: "orchard", Path: "/vol/orchard", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "marsh", Path: "/vol/marsh", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusTagged},
		{Name: "pasture", Path: "/vol/pasture", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
	}}
	s.ApplyIndexDrift(map[string]string{"orchard": localfs.IndexStatusUnchecked, "marsh": localfs.IndexStatusUnchecked, "pasture": localfs.IndexStatusUnchecked})
	if s.Entries[0].IndexStatus != localfs.IndexStatusUnchecked || s.Entries[1].IndexStatus != localfs.IndexStatusTagged {
		t.Fatalf("unchecked must replace indexed but not tagged: %+v", s.Entries)
	}
	s.ApplyIndexDrift(map[string]string{"orchard": localfs.IndexStatusIndexed, "pasture": localfs.IndexStatusDiffers})
	if s.Entries[0].IndexStatus != localfs.IndexStatusIndexed || s.Entries[2].IndexStatus != localfs.IndexStatusDiffers {
		t.Fatalf("explicit result must clear unchecked or mark differs: %+v", s.Entries)
	}
}

func TestIndexDriftAutoPassUsesDepthAndCanBeOff(t *testing.T) {
	root, _ := pathloc.File("/vol")
	var reqs []IndexDriftRequest
	s := State{Path: root, WithIndex: true, IndexAutoDriftDepth: 3, ScheduleIndexDrift: func(r IndexDriftRequest) bool { reqs = append(reqs, r); return true }, Entries: []localfs.Entry{
		{Name: "orchard", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "pasture", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusUnchecked},
	}}
	s.scheduleIndexDrift(nil)
	if len(reqs) != 1 || reqs[0].MaxDepth != 3 || len(reqs[0].Subdirs) != 2 {
		t.Fatalf("auto request = %+v", reqs)
	}
	s.RequestIndexDrift("pasture")
	if len(reqs) != 2 || reqs[1].MaxDepth != 0 || len(reqs[1].Subdirs) != 1 {
		t.Fatalf("explicit request = %+v", reqs)
	}
	s.IndexAutoDriftDepth = -1
	s.scheduleIndexDrift(nil)
	if len(reqs) != 2 {
		t.Fatalf("a negative depth must turn the automatic pass off: %+v", reqs)
	}
}

func TestGhostSources(t *testing.T) {
	local := State{Path: pathloc.MustParse("/vol")}
	if got := local.GhostSources("/vol/real.txt", "fileindex:///vol/bramble.txt"); len(got) != 1 || got[0] != "bramble.txt" {
		t.Fatalf("ghosts = %v", got)
	}
	idx := State{Path: pathloc.MustParse("fileindex:///vol")}
	if got := idx.GhostSources("fileindex:///vol/bramble.txt"); got != nil {
		t.Fatalf("a fileindex:// panel has no ghost rows, got %v", got)
	}
	if msg := GhostRefusal([]string{"a", "b"}); msg != "Only in the index: a, b" {
		t.Fatalf("refusal = %q", msg)
	}
}

func TestApplyListingClearsWithIndexOffLocalPaths(t *testing.T) {
	s := State{WithIndex: true}
	loc, _ := pathloc.FileIndex("/vol")
	_ = s.ApplyListing(loc, nil, "", 10, -1, false)
	if s.WithIndex {
		t.Fatal("WithIndex must clear when the listing is not local")
	}
}

func TestIndexDriftPendingFollowsThePass(t *testing.T) {
	root, _ := pathloc.File("/vol")
	var reqs []IndexDriftRequest
	started := true
	s := State{Path: root, WithIndex: true, ScheduleIndexDrift: func(r IndexDriftRequest) bool { reqs = append(reqs, r); return started }, Entries: []localfs.Entry{
		{Name: "orchard", Path: "/vol/orchard", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "marsh", Path: "/vol/marsh", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusTagged},
		{Name: "pasture", Path: "/vol/pasture", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusNew},
		{Name: "reed.txt", Path: "/vol/reed.txt", Type: localfs.EntryFile, IndexStatus: localfs.IndexStatusIndexed},
	}}
	pending := func(i int) bool { return s.IndexDriftPending(s.Entries[i]) }

	s.scheduleIndexDrift(nil)
	if !pending(0) || !pending(1) || pending(2) || pending(3) {
		t.Fatalf("pending after the automatic pass = %v", s.indexDriftPending)
	}
	first := reqs[0].Seq

	// A newer pass over orchard keeps it waiting after the first pass ends.
	s.RequestIndexDrift("orchard")
	second := reqs[1].Seq
	if !s.FinishIndexDrift(first) {
		t.Fatal("finishing the first pass stopped no row waiting")
	}
	if !pending(0) || pending(1) {
		t.Fatalf("pending after the first pass ended = %v", s.indexDriftPending)
	}

	// A result of differs is an answer, even while a pass still covers the directory.
	s.ApplyIndexDrift(map[string]string{"orchard": localfs.IndexStatusDiffers})
	if pending(0) {
		t.Fatal("a directory that differs still shows as waiting")
	}
	if !s.FinishIndexDrift(second) || s.FinishIndexDrift(second) {
		t.Fatal("finishing the second pass must end the wait exactly once")
	}

	// A pass that did not start leaves nothing waiting.
	started = false
	s.Entries[0].IndexStatus = localfs.IndexStatusIndexed
	s.scheduleIndexDrift(nil)
	if pending(0) || pending(1) {
		t.Fatalf("pending after a refused pass = %v", s.indexDriftPending)
	}
}

func TestFinishIndexDriftDirsEndsOnlyTheNamedRows(t *testing.T) {
	root, _ := pathloc.File("/vol")
	var reqs []IndexDriftRequest
	s := State{Path: root, WithIndex: true, ScheduleIndexDrift: func(r IndexDriftRequest) bool { reqs = append(reqs, r); return true }, Entries: []localfs.Entry{
		{Name: "orchard", Path: "/vol/orchard", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "marsh", Path: "/vol/marsh", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
		{Name: "quarry", Path: "/vol/quarry", Type: localfs.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed},
	}}
	pending := func(i int) bool { return s.IndexDriftPending(s.Entries[i]) }

	s.scheduleIndexDrift(nil)
	first := reqs[0].Seq
	s.RequestIndexDrift("marsh") // the newer pass owns marsh

	s.FinishIndexDriftDirs(map[string]uint64{"orchard": first, "marsh": first})
	if pending(0) || !pending(1) || !pending(2) {
		t.Fatalf("pending after the first pass reported orchard and marsh = %v", s.indexDriftPending)
	}
	s.FinishIndexDriftDirs(map[string]uint64{"marsh": reqs[1].Seq})
	if pending(1) || !pending(2) {
		t.Fatalf("pending after the newer pass reported marsh = %v", s.indexDriftPending)
	}
}

func TestSameDirReloadKeepsDriftOfUnchangedDirs(t *testing.T) {
	root, _ := pathloc.File("/vol")
	var reqs []IndexDriftRequest
	s := State{Path: root, WithIndex: true, ScheduleIndexDrift: func(r IndexDriftRequest) bool { reqs = append(reqs, r); return true }}
	when := time.Unix(1_700_000_000, 0)
	dir := func(name string, mod time.Time) fsbackend.Entry {
		loc, _ := pathloc.File("/vol/" + name)
		return fsbackend.Entry{Name: name, Loc: loc, Type: fsbackend.EntryDirectory, ModifiedAt: mod, IndexStatus: localfs.IndexStatusIndexed}
	}
	file := func(name string) fsbackend.Entry {
		loc, _ := pathloc.File("/vol/" + name)
		return fsbackend.Entry{Name: name, Loc: loc, Type: fsbackend.EntryFile, ModifiedAt: when, IndexStatus: localfs.IndexStatusIndexed}
	}
	listing := []fsbackend.Entry{dir("marsh", when), dir("orchard", when), dir("pasture", when), file("lantern.txt")}
	if err := s.ApplyListing(root, listing, "", 10, -1, false); err != nil || len(reqs) != 1 || len(reqs[0].Subdirs) != 3 {
		t.Fatalf("first listing drift = %+v err=%v, want both dirs", reqs, err)
	}
	s.ApplyIndexDrift(map[string]string{"marsh": localfs.IndexStatusIndexed, "orchard": localfs.IndexStatusDiffers, "pasture": localfs.IndexStatusIndexed})
	s.FinishIndexDrift(reqs[0].Seq)

	// lantern.txt deleted, pasture touched: only pasture walks again; orchard keeps differs.
	reload := []fsbackend.Entry{dir("marsh", when), dir("orchard", when), dir("pasture", when.Add(time.Second))}
	if err := s.ApplyListing(root, reload, "", 10, -1, false); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 2 || len(reqs[1].Subdirs) != 1 || reqs[1].Subdirs[0] != "pasture" {
		t.Fatalf("reload drift = %+v, want only pasture", reqs)
	}
	if e := s.Entries[1]; e.Name != "orchard" || e.IndexStatus != localfs.IndexStatusDiffers || s.IndexDriftPending(e) {
		t.Fatalf("orchard = %+v, want its differs kept and not pending", e)
	}

	s.ForgetIndexDrift()
	if err := s.ApplyListing(root, reload, "", 10, -1, false); err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 3 || len(reqs[2].Subdirs) != 3 {
		t.Fatalf("after ForgetIndexDrift drift = %+v, want both dirs", reqs)
	}
}
