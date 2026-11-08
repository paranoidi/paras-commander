package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// scopedPanels is an app whose two panels browse with fileindex in separate directories.
func scopedPanels(t *testing.T) (*App, string, string) {
	t.Helper()
	a, _, dir, _ := fileIndexApp(t)
	left, right := filepath.Join(dir, "orchard"), filepath.Join(dir, "meadow")
	for _, d := range []string{left, right} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for id, d := range map[int]string{ui.PrimaryPanel: left, ui.SecondaryPanel: right} {
		p := a.panelByID(id)
		p.WithIndex = true
		sched := p.ScheduleAsyncLoad
		p.ScheduleAsyncLoad = nil
		if err := p.Load(d); err != nil {
			t.Fatal(err)
		}
		p.ScheduleAsyncLoad = sched
	}
	return a, left, right
}

func reloads(a *App, f func()) []int {
	before := []uint64{a.panelAsyncLoadGen[ui.PrimaryPanel].Load(), a.panelAsyncLoadGen[ui.SecondaryPanel].Load()}
	f()
	var out []int
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if a.panelAsyncLoadGen[id].Load() != before[id] {
			out = append(out, id)
		}
	}
	return out
}

func TestIndexChangeReloadsOnlyAffectedPanels(t *testing.T) {
	a, left, right := scopedPanels(t)
	for _, c := range []struct {
		name string
		dirs []string
		want []int
	}{
		{"its own dir", []string{left}, []int{ui.PrimaryPanel}},
		{"the other's", []string{right}, []int{ui.SecondaryPanel}},
		{"a dir above it", []string{filepath.Dir(left)}, []int{ui.PrimaryPanel, ui.SecondaryPanel}},
		{"a dir below it", []string{filepath.Join(left, "deeper")}, nil},
		{"unrelated", []string{"/somewhere/else"}, nil},
		{"a sibling sharing a prefix", []string{left + "-not"}, nil},
		{"anything", nil, []int{ui.PrimaryPanel, ui.SecondaryPanel}},
	} {
		got := reloads(a, func() { a.refreshIndexViewsIn(c.dirs) })
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: reloaded %v, want %v", c.name, got, c.want)
		}
	}
}

func inFlight(a *App) []int {
	var out []int
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if a.panelRefreshInFlight[id].Load() {
			out = append(out, id)
		}
	}
	return out
}

func TestScanCommitReListsOnlyThePanelShowingTheDir(t *testing.T) {
	a, left, _ := scopedPanels(t)
	block := make(chan struct{})
	orig := fetchListingForPanelRefresh
	fetchListingForPanelRefresh = func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		<-block
		return orig(ctx, snap)
	}
	t.Cleanup(func() { close(block); fetchListingForPanelRefresh = orig })

	asyncWakes(a).indexScan.push(filepath.Join(left, "deeper")) // not what a panel shows
	a.handleIndexScanWake()
	if got := inFlight(a); len(got) != 0 {
		t.Fatalf("re-listing %v for a dir no panel shows", got)
	}
	asyncWakes(a).indexScan.push(left)
	a.handleIndexScanWake()
	if got := inFlight(a); !slices.Equal(got, []int{ui.PrimaryPanel}) {
		t.Fatalf("re-listing %v, want the panel showing it", got)
	}
}

func TestScanCommitWakeWaitsForTheBrowser(t *testing.T) {
	a, left, _ := scopedPanels(t)
	a.model.ViewMode = ui.ViewCompare
	asyncWakes(a).indexScan.push(left)
	a.handleIndexScanWake()
	if got := inFlight(a); len(got) != 0 {
		t.Fatalf("re-listing %v outside the browser view", got)
	}
	if rest := asyncWakes(a).indexScan.drain(); len(rest) != 0 {
		t.Fatalf("left queued %v: the job's end refreshes anyway", rest)
	}
}

// listedIndexedDir is a listing holding one indexed directory, which a drift pass would check.
func listedIndexedDir(t *testing.T, dir pathloc.Path, name string) []fsbackend.Entry {
	t.Helper()
	loc, err := dir.Join(name)
	if err != nil {
		t.Fatal(err)
	}
	return []fsbackend.Entry{{Name: name, Loc: loc, Type: fsbackend.EntryDirectory, IndexStatus: localfs.IndexStatusIndexed}}
}

func TestProgressListingSkipsDriftPass(t *testing.T) {
	a, left, _ := scopedPanels(t)
	p := a.panelByID(ui.PrimaryPanel)
	asked := 0
	p.ScheduleIndexDrift = func(panel.IndexDriftRequest) bool { asked++; return true }
	if err := os.Mkdir(filepath.Join(left, "shelf"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries := listedIndexedDir(t, p.Path, "shelf")
	a.applyPanelListingRefresh(panelRefreshApplyPayload{PanelID: ui.PrimaryPanel, Path: p.Path, Entries: entries, ListingEpoch: p.ListingEpoch, Progress: true})
	if asked != 0 {
		t.Fatalf("drift passes = %d during a scan's progress", asked)
	}
	if p.SkipIndexDrift {
		t.Fatal("the skip outlived the listing")
	}
}

// Rows follow a running scan: what a commit hashed turns indexed while the rest stays unhashed.
func TestScanCommitShowsPartialResults(t *testing.T) {
	a, screen, _, _ := fileIndexApp(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stamp := map[string]os.FileInfo{}
	var walked []db.DiscoveredFile
	for _, name := range []string{"acorn.txt", "bramble.txt"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		stamp[name] = fi
		walked = append(walked, db.DiscoveredFile{Dir: dir, Name: name, Size: fi.Size(), Mtime: fi.ModTime()})
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CommitDiscovered(walked, []string{dir}); err != nil {
		t.Fatal(err)
	}
	p := a.panelByID(ui.PrimaryPanel)
	p.WithIndex = true
	sched := p.ScheduleAsyncLoad
	p.ScheduleAsyncLoad = nil
	if err := p.Load(dir); err != nil {
		t.Fatal(err)
	}
	p.ScheduleAsyncLoad = sched
	status := func(name string) string {
		for _, e := range p.Entries {
			if e.Name == name {
				return e.IndexStatus
			}
		}
		return ""
	}
	if status("acorn.txt") != localfs.IndexStatusUnhashed || status("bramble.txt") != localfs.IndexStatusUnhashed {
		t.Fatalf("before hashing: %s, %s", status("acorn.txt"), status("bramble.txt"))
	}

	sum := strings.Repeat("c", 32)
	fi := stamp["acorn.txt"]
	if _, err := d.CommitScanBatch([]db.ScannedFile{{Dir: dir, Name: "acorn.txt", Size: fi.Size(), Mtime: fi.ModTime(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	asyncWakes(a).indexScan.push(dir)
	a.handleIndexScanWake()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool { return status("acorn.txt") == localfs.IndexStatusIndexed })
	if got := status("bramble.txt"); got != localfs.IndexStatusUnhashed {
		t.Fatalf("the file not hashed yet shows %s", got)
	}
}
