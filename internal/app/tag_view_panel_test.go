package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

const (
	tvList   = "fileindex@(beach):///"
	tvStruct = "fileindex(beach):///"
	tvCedarL = "fileindex@(beach):///cedar/lantern"
	tvOrchL  = "fileindex@(beach):///orchard/lantern"
	tvCedarN = "fileindex@(beach):///cedar/notes.txt"
	tvOrchN  = "fileindex@(beach):///orchard/notes.txt"
)

// tagViewApp builds an app over an index of made-up directories (nothing of them is on disk): two
// "lantern" directories and two "notes.txt" files, each pair in different parents, all tagged beach.
func tagViewApp(t *testing.T) (*App, tcell.SimulationScreen) {
	t.Helper()
	a, screen, _, _ := fileIndexApp(t)
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	var batch []db.ScannedFile
	for i, f := range []struct{ dir, name string }{
		{"/orchard", "notes.txt"}, {"/cedar", "notes.txt"},
		{"/orchard/lantern", "clover.txt"}, {"/cedar/lantern", "thistle.txt"},
		{"/orchard/lantern/sub", "moss.txt"},
	} {
		sum := strings.Repeat(string(rune('k'+i)), 32)
		batch = append(batch, db.ScannedFile{Dir: f.dir, Name: f.name, Size: 5, Mtime: time.Now(), MD5: sum, SHA256: sum + sum})
	}
	if _, err := d.CommitScanBatch(batch); err != nil {
		t.Fatal(err)
	}
	if _, err := a.fileIndex.EditTags([]string{
		"fileindex:///orchard/notes.txt", "fileindex:///cedar/notes.txt",
		"fileindex:///orchard/lantern", "fileindex:///cedar/lantern",
	}, []string{"beach"}, true, false); err != nil {
		t.Fatal(err)
	}
	return a, screen
}

// tvGoTo navigates the focused panel to path and waits for the listing.
func tvGoTo(t *testing.T, a *App, screen tcell.SimulationScreen, path string) {
	t.Helper()
	if err := a.navigatePanelToDirectory(a.focusedPanelID(), path, ""); err != nil {
		t.Fatal(err)
	}
	tvWait(t, a, screen, path)
}

// tvWait waits until the focused panel shows path with no listing pending.
func tvWait(t *testing.T, a *App, screen tcell.SimulationScreen, path string) {
	t.Helper()
	want := pathloc.MustParse(path).String()
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.focusedPanel()
		return p.PathString() == want && !p.ListingPending
	})
}

// tvCursor is the path of the focused panel's cursor row.
func tvCursor(t *testing.T, a *App) string {
	t.Helper()
	e, ok := a.focusedPanel().CurrentEntry()
	if !ok {
		t.Fatal("no cursor row")
	}
	return e.Path
}

func tvSelect(t *testing.T, a *App, path string) {
	t.Helper()
	if !a.focusedPanel().SelectVisibleEntry(pathloc.MustParse(path).String()) {
		t.Fatalf("%s not listed in %s", path, a.focusedPanel().PathString())
	}
}

// tvQuiet lets the event loop run for a while, long enough for a reload to land.
func tvQuiet(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	end := time.Now().Add(300*time.Millisecond)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return time.Now().After(end) })
}

func TestTagViewFlatEnterAndUpReturnToTheRow(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	if n := a.focusedPanel().VisibleEntryCount(); n != 4 {
		t.Fatalf("flat list has %d rows, want 4", n)
	}
	for _, row := range []string{tvOrchL, tvCedarL} { // two directories of one base name
		tvSelect(t, a, row)
		a.dispatch(keymap.ActionNavOpen)
		tvWait(t, a, screen, row)
		a.dispatch(keymap.ActionNavParent)
		tvWait(t, a, screen, tvList)
		if got := tvCursor(t, a); got != row {
			t.Fatalf("up from %s left the cursor on %s", row, got)
		}
	}
}

func TestTagViewFlatGoingUpFromInsideARow(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvOrchL+"//sub")
	a.dispatch(keymap.ActionNavParent)
	tvWait(t, a, screen, tvOrchL)
	if got := tvCursor(t, a); got != tvOrchL+"//sub" {
		t.Fatalf("cursor = %s, want the directory just left", got)
	}
	a.dispatch(keymap.ActionNavParent)
	tvWait(t, a, screen, tvList)
	if got := tvCursor(t, a); got != tvOrchL {
		t.Fatalf("cursor = %s, want the row just left", got)
	}
	if err := a.navigatePanelToAncestor(a.focusedPanelID(), 1); err != nil {
		t.Fatal(err)
	}
	if got := a.focusedPanel().PathString(); got != tvList {
		t.Fatalf("at the list, an ancestor is still %s, got %s", tvList, got)
	}
}

func TestTagViewFlatRefreshKeepsTheRightOfEqualNames(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	p := a.focusedPanel()
	rows := a.panelViewportRows(a.focusedPanelID())
	for _, row := range []string{tvOrchN, tvCedarN, tvOrchL} {
		tvSelect(t, a, row)
		if err := p.Refresh(rows); err != nil {
			t.Fatal(err)
		}
		tvWait(t, a, screen, tvList)
		if got := tvCursor(t, a); got != row {
			t.Fatalf("refresh moved the cursor from %s to %s", row, got)
		}
		if err := p.SetShowHidden(!p.ShowHidden, rows); err != nil {
			t.Fatal(err)
		}
		tvWait(t, a, screen, tvList)
		if got := tvCursor(t, a); got != row {
			t.Fatalf("show-hidden moved the cursor from %s to %s", row, got)
		}
	}
}

func TestTagViewFlatHistoryRestoresTheRow(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	tvSelect(t, a, tvOrchN)
	tvGoTo(t, a, screen, tvCedarL)
	tvGoTo(t, a, screen, tvStruct)
	a.dispatch(keymap.ActionNavBackward)
	tvWait(t, a, screen, tvCedarL)
	a.dispatch(keymap.ActionNavBackward)
	tvWait(t, a, screen, tvList)
	if got := tvCursor(t, a); got != tvOrchN {
		t.Fatalf("history back put the cursor on %s, want %s", got, tvOrchN)
	}
	a.dispatch(keymap.ActionNavForward)
	tvWait(t, a, screen, tvCedarL)
	a.dispatch(keymap.ActionNavForward)
	tvWait(t, a, screen, tvStruct)
}

func TestTagViewStructureDownAndUp(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvStruct)
	tvSelect(t, a, "fileindex(beach):///orchard")
	a.dispatch(keymap.ActionNavOpen)
	tvWait(t, a, screen, "fileindex(beach):///orchard")
	a.dispatch(keymap.ActionNavParent)
	tvWait(t, a, screen, tvStruct)
	if got := tvCursor(t, a); got != "fileindex(beach):///orchard" {
		t.Fatalf("cursor = %s, want orchard", got)
	}
}

func TestTagViewUntaggingTheLastMatchLeavesAnEmptyList(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	if _, err := a.fileIndex.EditTags([]string{
		"fileindex:///orchard/notes.txt", "fileindex:///cedar/notes.txt",
		"fileindex:///orchard/lantern", "fileindex:///cedar/lantern",
	}, []string{"beach"}, false, false); err != nil {
		t.Fatal(err)
	}
	a.refreshIndexViews()
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.focusedPanel()
		return !p.ListingPending && p.VisibleEntryCount() == 0
	})
	if a.model.Message != "" && a.model.MessageUrgency >= ui.MessageUrgencyWarn {
		t.Fatalf("untagging showed %q", a.model.Message)
	}

	// Inside a row the listing is the plain index, so it survives the row's untagging.
	tvGoTo(t, a, screen, tvOrchL)
	a.refreshIndexViews()
	tvWait(t, a, screen, tvOrchL)
	if n := a.focusedPanel().VisibleEntryCount(); n != 2 {
		t.Fatalf("inside the row: %d entries, want 2", n)
	}
}

func TestTagViewTreeLayoutInAFlatList(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	p := a.focusedPanel()
	rows := a.panelViewportRows(a.focusedPanelID())
	if !p.SetListLayout(panel.ListLayoutTree, rows) {
		t.Fatal("tree layout refused")
	}
	tvSelect(t, a, tvOrchL)
	if err := p.ExpandTreeCursorRow(rows); err != nil {
		t.Fatal(err)
	}
	child := tvOrchL + "//sub"
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return treeShows(p, child) })
	if !treeShows(p, tvOrchL+"//clover.txt") {
		t.Fatal("the expanded row's files are not listed")
	}

	tvSelect(t, a, child)
	if got := tvCursor(t, a); got != child {
		t.Fatalf("cursor = %s, want the nested row", got)
	}
	if err := p.Refresh(rows); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return !p.ListingPending && treeShows(p, child)
	})
	if got := tvCursor(t, a); got != child {
		t.Fatalf("refresh moved the cursor to %s, want %s", got, child)
	}

	tvSelect(t, a, tvOrchL)
	if err := p.ToggleTreeExpand(rows); err != nil {
		t.Fatal(err)
	}
	if treeShows(p, child) {
		t.Fatal("collapsing left the children listed")
	}
	// The row's directory is its own: a row of the same base name elsewhere stays collapsed.
	if e, ok := p.CurrentEntry(); !ok || p.EntryDir(e).String() != "fileindex@(beach):///orchard" {
		t.Fatalf("EntryDir of the row = %v", p.EntryDir(e))
	}
}

func TestTagViewMetaTagsOfEqualNamesInDifferentDirectories(t *testing.T) {
	a, screen := tagViewApp(t)
	if _, err := a.fileIndex.EditTags([]string{"fileindex:///orchard/notes.txt"}, []string{"harvest"}, true, false); err != nil {
		t.Fatal(err)
	}
	tvGoTo(t, a, screen, tvList)
	p := a.focusedPanel()
	prov := a.fileIndex.Provider(fileindex.TagsColumn)
	if prov == nil {
		t.Fatal("no tags provider")
	}
	byDir := map[string][]localfs.Entry{}
	for _, e := range p.Entries {
		if e.Name == "notes.txt" {
			dir := p.EntryDir(e).String()
			byDir[dir] = append(byDir[dir], e)
		}
	}
	if len(byDir) != 2 {
		t.Fatalf("notes.txt rows group into %d directories, want 2", len(byDir))
	}
	for dir, es := range byDir {
		vals, err := prov.Values(t.Context(), dir, es)
		if err != nil {
			t.Fatal(err)
		}
		got := vals[es[0].Path]
		want := "beach"
		if strings.Contains(es[0].Path, "orchard") {
			want = "beach, harvest"
		}
		if got != want {
			t.Fatalf("tags of %s = %q, want %q", es[0].Path, got, want)
		}
	}
}

func TestTagViewScanCommitDoesNotReloadButAChangeDoes(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	p := a.focusedPanel()
	if !showsTagListing(p) || !a.indexChangeAffects(p, []string{"/somewhere/else"}) || !a.indexChangeReachesSubtrees(p, []string{"/somewhere/else"}) {
		t.Fatal("any index change must reach a tag list")
	}
	epoch := p.ListingEpoch
	a.queueIndexScanWake([]string{"/orchard", "/cedar", "/"})
	a.handleIndexScanWake()
	tvQuiet(t, a, screen)
	if p.ListingEpoch != epoch {
		t.Fatal("a scan commit reloaded the tag list")
	}
	a.refreshIndexViewsIn([]string{"/elsewhere"})
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return p.ListingEpoch != epoch && !p.ListingPending })

	tvGoTo(t, a, screen, tvStruct)
	p = a.focusedPanel()
	epoch = p.ListingEpoch
	a.queueIndexScanWake([]string{"/"})
	a.handleIndexScanWake()
	tvQuiet(t, a, screen)
	if p.ListingEpoch != epoch {
		t.Fatal("a scan commit reloaded the structure view")
	}
}
