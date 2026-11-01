package panel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
	s.scheduleIndexDrift()
	if len(reqs) != 1 || reqs[0].MaxDepth != 3 || len(reqs[0].Subdirs) != 2 {
		t.Fatalf("auto request = %+v", reqs)
	}
	s.RequestIndexDrift("pasture")
	if len(reqs) != 2 || reqs[1].MaxDepth != 0 || len(reqs[1].Subdirs) != 1 {
		t.Fatalf("explicit request = %+v", reqs)
	}
	s.IndexAutoDriftDepth = -1
	s.scheduleIndexDrift()
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
