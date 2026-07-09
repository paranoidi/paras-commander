package ui

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func dedupTestGroup(hashByte byte, size int64, rels ...string) comparepkg.DedupGroup {
	g := comparepkg.DedupGroup{Size: size}
	g.Hash[0] = hashByte
	for _, rel := range rels {
		g.Files = append(g.Files, comparepkg.DedupFile{Rel: rel, Abs: pathloc.MustParse("/root/" + rel)})
	}
	return g
}

func TestDedupCollapsedSet(t *testing.T) {
	got := DedupCollapsedSet([]string{"d:meadow", "g:01"})
	if len(got) != 2 || !got["d:meadow"] || !got["g:01"] {
		t.Fatalf("DedupCollapsedSet = %v", got)
	}
	if DedupCollapsedSet(nil) != nil {
		t.Fatal("nil ids should return nil map")
	}
}

func TestDedupRowsFromSnapshotDoneOnly(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase:  comparepkg.DedupHashing,
		Groups: []comparepkg.DedupGroup{dedupTestGroup(1, 10, "a")},
	}
	if got, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true}); got != nil {
		t.Fatalf("hashing phase rows = %v, want nil", got)
	}

	snap.Phase = comparepkg.DedupDone
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if len(rows) != 1 {
		t.Fatalf("done rows len = %d, want 1", len(rows))
	}
	d := rows[0].Value
	if d.Kind != DedupRowFile || d.AbsKey != "/root/a" || d.Copies != 1 || !d.ShowSize {
		t.Fatalf("row = %+v, unexpected fields", d)
	}
}

func TestDedupRowsFromSnapshotGroupsModeSortAndIgnoreEmpty(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 300, "zebra/copper", "zebra/willow"), // path-last, largest
			dedupTestGroup(2, 100, "apple/copper", "apple/willow"), // path-first, smallest
			dedupTestGroup(3, 0, "mango/copper", "mango/willow"),   // empty group
		},
	}

	// Default: order by path, empties hidden; header + one child per group.
	rows, ignored := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if ignored != 2 {
		t.Fatalf("ignoredEmpty = %d, want 2", ignored)
	}
	if len(rows) != 4 {
		t.Fatalf("rows len = %d, want 4", len(rows))
	}
	if got := rows[0].Value.Display; got != "apple/copper" {
		t.Fatalf("path-order first = %q, want apple/copper", got)
	}
	if !rows[0].HasChildren || rows[0].Depth != 0 || !rows[0].Expanded {
		t.Fatalf("header row = %+v, want expandable expanded depth 0", rows[0])
	}
	if rows[1].HasChildren || rows[1].Depth != 1 || rows[1].Value.ShowSize {
		t.Fatalf("child row = %+v (%+v), want leaf depth 1 without size", rows[1], rows[1].Value)
	}

	// Most space wasted: largest group first.
	rows, _ = DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, SortByWasted: true})
	if got := rows[0].Value.Display; got != "zebra/copper" {
		t.Fatalf("wasted-order first = %q, want zebra/copper", got)
	}

	// Showing empties keeps all three groups and reports zero ignored.
	rows, ignored = DedupRowsFromSnapshot(snap, DedupViewState{})
	if ignored != 0 || len(rows) != 6 {
		t.Fatalf("show-empty: ignored=%d len=%d, want 0 and 6", ignored, len(rows))
	}
}

func TestDedupRowsFromSnapshotGroupsCollapsePending(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "apple/copper", "apple/willow"),
			dedupTestGroup(2, 200, "pearl/copper", "pearl/willow"),
		},
	}
	view := DedupViewState{IgnoreEmpty: true, GroupsCollapsePending: true}
	for _, id := range DedupExpandableIDs(snap, view) {
		view.Main.SetCollapsed(id, true)
	}
	view.GroupsCollapsePending = false
	rows, _ := DedupRowsFromSnapshot(snap, view)
	if len(rows) != 2 {
		t.Fatalf("collapsed groups rows = %d, want 2 headers", len(rows))
	}
	for _, r := range rows {
		if r.Expanded {
			t.Fatalf("group header %q should be collapsed", r.Value.Display)
		}
	}
}

func TestDedupRowsFromSnapshotCollapse(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "apple/copper", "apple/willow", "apple/maple"),
			dedupTestGroup(2, 200, "pearl/copper", "pearl/willow"),
		},
	}
	all, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if len(all) != 5 {
		t.Fatalf("expanded rows = %d, want 5", len(all))
	}

	view := DedupViewState{IgnoreEmpty: true, Main: DedupPane{Collapsed: map[string]bool{all[0].ID: true}}}
	rows, _ := DedupRowsFromSnapshot(snap, view)
	if len(rows) != 3 {
		t.Fatalf("collapsed rows = %d, want 3 (header + second group)", len(rows))
	}
	if rows[0].Expanded || !rows[0].HasChildren {
		t.Fatalf("collapsed header = %+v, want !Expanded && HasChildren", rows[0])
	}
	// Collapsed header keeps showing size ×N.
	if !rows[0].Value.ShowSize || rows[0].Value.Copies != 3 {
		t.Fatalf("collapsed header value = %+v, want ShowSize ×3", rows[0].Value)
	}

	ids := DedupExpandableIDs(snap, view)
	if len(ids) != 2 {
		t.Fatalf("expandable ids = %v, want 2 group ids", ids)
	}
}

func TestDedupRowsFromSnapshotDirsMode(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "backup/old/copper", "copper"),
			dedupTestGroup(2, 200, "backup/willow", "keep/willow"),
		},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})

	// Expected shape (dirs first then files, sorted by name):
	// backup/ → old/ → copper ; willow ; keep/ → willow ; copper
	type want struct {
		display string
		depth   int
		dir     bool
	}
	wants := []want{
		{"backup", 0, true},
		{"old", 1, true},
		{"copper", 2, false},
		{"willow", 1, false},
		{"keep", 0, true},
		{"willow", 1, false},
		{"copper", 0, false},
	}
	if len(rows) != len(wants) {
		t.Fatalf("rows len = %d, want %d (%+v)", len(rows), len(wants), rows)
	}
	for i, w := range wants {
		d := rows[i].Value
		if d.Display != w.display || rows[i].Depth != w.depth || (d.Kind == DedupRowDir) != w.dir {
			t.Fatalf("row %d = %+v (%+v), want %+v", i, rows[i], d, w)
		}
	}
	// Dir rows carry rel path and no group; file leaves show size ×N.
	if rows[1].Value.DirRel != "backup/old" || rows[1].Value.GroupIdx != -1 {
		t.Fatalf("dir row value = %+v, want DirRel backup/old GroupIdx -1", rows[1].Value)
	}
	if !rows[2].Value.ShowSize || rows[2].Value.Copies != 2 {
		t.Fatalf("file leaf value = %+v, want ShowSize ×2", rows[2].Value)
	}

	// Collapsing backup/ hides its whole subtree.
	view := DedupViewState{IgnoreEmpty: true, TreeDirs: true, Main: DedupPane{Collapsed: map[string]bool{"d:backup": true}}}
	rows, _ = DedupRowsFromSnapshot(snap, view)
	if len(rows) != 4 {
		t.Fatalf("collapsed rows len = %d, want 4", len(rows))
	}
}

func TestDedupRowsFromSnapshotTrimmedDisplayRoot(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/x"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "alpha/bravo/charlie/copper.txt", "alpha/bravo/charlie/willow.txt"),
		},
	}
	snap = snap.WithTrimmedDisplayRoot()

	if got := snap.EffectiveDisplayRoot().String(); got != "/scan/x/alpha/bravo/charlie" {
		t.Fatalf("EffectiveDisplayRoot = %q, want /scan/x/alpha/bravo/charlie", got)
	}

	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2 files without empty dir chain", len(rows))
	}
	for _, r := range rows {
		if r.Value.Kind != DedupRowFile || r.Depth != 0 {
			t.Fatalf("row = %+v, want depth-0 file leaf", r)
		}
	}
}

func TestDedupRowsFromSnapshotTrimToBranchParent(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/x"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "alpha/bravo/copper.txt", "alpha/delta/willow.txt"),
		},
	}
	snap = snap.WithTrimmedDisplayRoot()

	if got := snap.EffectiveDisplayRoot().String(); got != "/scan/x/alpha" {
		t.Fatalf("EffectiveDisplayRoot = %q, want /scan/x/alpha", got)
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	var topDirs []string
	for _, r := range rows {
		if r.Value.Kind == DedupRowDir && r.Depth == 0 {
			topDirs = append(topDirs, r.Value.Display)
		}
	}
	if len(topDirs) != 2 || topDirs[0] != "bravo" || topDirs[1] != "delta" {
		t.Fatalf("top-level dirs = %v, want [bravo delta]", topDirs)
	}
}

func TestDedupRowIndexByID(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase:  comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{dedupTestGroup(1, 100, "apple/copper", "apple/willow")},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if got := DedupRowIndexByID(rows, "/root/apple/willow"); got != 1 {
		t.Fatalf("index by abs = %d, want 1", got)
	}
	if got := DedupRowIndexByID(rows, rows[0].ID); got != 0 {
		t.Fatalf("index by group id = %d, want 0", got)
	}
	if got := DedupRowIndexByID(rows, "missing"); got != -1 {
		t.Fatalf("index missing = %d, want -1", got)
	}
	if got := DedupRowIndexByID(rows, ""); got != -1 {
		t.Fatalf("index empty = %d, want -1", got)
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

func TestDedupPaneEnsureSelectionVisibleMaxScroll(t *testing.T) {
	p := DedupPane{Selected: 99, ListScroll: 0}
	p.EnsureSelectionVisible(100, 10)
	if p.ListScroll != 90 {
		t.Fatalf("ListScroll = %d, want 90", p.ListScroll)
	}
}

func TestDedupCopyRows(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt", "orchard/deep/lantern.txt"),
			dedupTestGroup(2, 200, "pebble.txt", "meadow/pebble.txt"),
		},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})

	// Selection on the first group's header: copies pane shows the OTHER two
	// copies as a directory tree, excluding the selected file itself.
	copies := DedupCopyRows(snap, rows[0], nil)
	var files []string
	for _, r := range copies {
		if r.Value.Kind == DedupRowFile {
			files = append(files, r.Value.File.Rel)
		}
	}
	if len(files) != 2 || files[0] != "meadow/lantern.txt" || files[1] != "orchard/deep/lantern.txt" {
		t.Fatalf("copy files = %v, want the two other lantern copies", files)
	}
	for _, r := range copies {
		if r.Value.AbsKey == "/root/lantern.txt" {
			t.Fatal("copies pane must exclude the selected file itself")
		}
	}

	// Dir rows are not files: no copies pane content.
	dirRows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	var dirRow DedupRow
	for _, r := range dirRows {
		if r.Value.Kind == DedupRowDir {
			dirRow = r
			break
		}
	}
	if got := DedupCopyRows(snap, dirRow, nil); got != nil {
		t.Fatalf("copies for dir row = %v, want nil", got)
	}
}

func TestDedupDirRowDetails(t *testing.T) {
	// meadow holds one copy of each group; both groups also live outside, so
	// everything under meadow is reclaimable: 2 dups, 100+200 wasted bytes.
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt"),
			dedupTestGroup(2, 200, "pebble.txt", "meadow/pebble.txt"),
		},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	var meadow DedupRowData
	for _, r := range rows {
		if r.Value.Kind == DedupRowDir && r.Value.DirRel == "meadow" {
			meadow = r.Value
		}
	}
	if meadow.DupCount != 2 || meadow.WastedBytes != 300 {
		t.Fatalf("meadow details = %d dups %d wasted, want 2 and 300", meadow.DupCount, meadow.WastedBytes)
	}

	// A group living entirely under one dir keeps one survivor: 2 dups but only
	// one copy's bytes are reclaimable.
	snap = comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(3, 500, "vault/copper.txt", "vault/willow.txt"),
		},
	}
	rows, _ = DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	if rows[0].Value.Kind != DedupRowDir || rows[0].Value.DirRel != "vault" {
		t.Fatalf("first row = %+v, want vault dir", rows[0].Value)
	}
	if rows[0].Value.DupCount != 2 || rows[0].Value.WastedBytes != 500 {
		t.Fatalf("vault details = %d dups %d wasted, want 2 and 500 (keep one survivor)",
			rows[0].Value.DupCount, rows[0].Value.WastedBytes)
	}
}

func TestDedupCollapseNewIDs(t *testing.T) {
	pane := DedupPane{}
	DedupCollapseNewIDs(&pane, []string{"d:a", "d:b"}, []string{"d:a", "d:b", "d:c"})
	if !pane.Collapsed["d:c"] {
		t.Fatal("new id should be collapsed")
	}
	if pane.Collapsed["d:a"] || pane.Collapsed["d:b"] {
		t.Fatal("previously absent ids should not be forced collapsed")
	}

	pane2 := DedupPane{Collapsed: map[string]bool{"d:a": true}}
	DedupCollapseNewIDs(&pane2, []string{"d:a"}, []string{"d:a", "d:b"})
	if !pane2.Collapsed["d:a"] {
		t.Fatal("still-collapsed id should stay collapsed")
	}
	if !pane2.Collapsed["d:b"] {
		t.Fatal("new id should be collapsed")
	}
}

func TestDedupRowsFromSnapshotShowEmptyRespectsCollapsed(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "apple/copper", "apple/willow"),
			dedupTestGroup(2, 0, "emptydir/copper", "emptydir/willow"),
		},
	}
	view := DedupViewState{IgnoreEmpty: true, TreeDirs: true, Main: DedupPane{Collapsed: map[string]bool{"d:apple": true}}}
	prevIDs := DedupExpandableIDs(snap, view)

	view.IgnoreEmpty = false
	DedupCollapseNewIDs(&view.Main, prevIDs, DedupExpandableIDs(snap, view))
	rows, _ := DedupRowsFromSnapshot(snap, view)

	var emptyDir *DedupRow
	for i := range rows {
		if rows[i].Value.Kind == DedupRowDir && rows[i].Value.DirRel == "emptydir" {
			emptyDir = &rows[i]
			break
		}
	}
	if emptyDir == nil {
		t.Fatal("emptydir should appear when showing empties")
	}
	if emptyDir.Expanded {
		t.Fatal("emptydir should stay collapsed (directory tree default)")
	}
	if !view.Main.Collapsed["d:apple"] {
		t.Fatal("previously collapsed dir should stay collapsed")
	}
}

func TestDedupActiveGroups(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "apple/copper", "apple/willow"),
			dedupTestGroup(2, 0, "mango/copper", "mango/willow"),
		},
	}
	if got := len(DedupActiveGroups(snap, true)); got != 1 {
		t.Fatalf("active groups (ignore empty) = %d, want 1", got)
	}
	if got := len(DedupActiveGroups(snap, false)); got != 2 {
		t.Fatalf("active groups (show empty) = %d, want 2", got)
	}
}

func TestDedupGroupFullyMarked(t *testing.T) {
	g := dedupTestGroup(1, 10, "g1/a", "g1/b")
	if DedupGroupFullyMarked(g, nil) {
		t.Fatal("nil marked map: want false")
	}
	if DedupGroupFullyMarked(g, map[string]bool{"/root/g1/a": true}) {
		t.Fatal("partial group: want false")
	}
	if !DedupGroupFullyMarked(g, map[string]bool{"/root/g1/a": true, "/root/g1/b": true}) {
		t.Fatal("full group: want true")
	}
	if DedupGroupFullyMarked(comparepkg.DedupGroup{}, map[string]bool{}) {
		t.Fatal("empty group: want false")
	}
}

func TestDedupMarkedDirSet(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "meadow/copper/anchor", "harbor/anchor"),
		},
	}
	if got := dedupMarkedDirSet(snap, nil); got != nil {
		t.Fatalf("no marks = %v, want nil", got)
	}
	got := dedupMarkedDirSet(snap, map[string]bool{"/root/meadow/copper/anchor": true})
	if len(got) != 2 || !got["meadow"] || !got["meadow/copper"] {
		t.Fatalf("marked dirs = %v, want exactly meadow + meadow/copper", got)
	}
}

func TestDedupDangerMarkedDirSet(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "meadow/copper/anchor", "harbor/anchor"),
		},
	}
	if got := dedupDangerMarkedDirSet(snap, nil); got != nil {
		t.Fatalf("no marks = %v, want nil", got)
	}
	partial := map[string]bool{"/root/meadow/copper/anchor": true}
	if got := dedupDangerMarkedDirSet(snap, partial); len(got) != 0 {
		t.Fatalf("partial mark danger dirs = %v, want none", got)
	}
	full := map[string]bool{
		"/root/meadow/copper/anchor": true,
		"/root/harbor/anchor":        true,
	}
	got := dedupDangerMarkedDirSet(snap, full)
	if len(got) != 3 || !got["meadow"] || !got["meadow/copper"] || !got["harbor"] {
		t.Fatalf("fully marked danger dirs = %v, want meadow + meadow/copper + harbor", got)
	}
}

func TestDedupCopyFilesUnderDir(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt", "meadow/beacon.txt", "orchard/lantern.txt"),
		},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	var mainSel DedupRow
	for _, r := range rows {
		if r.Value.Kind == DedupRowFile && r.Value.AbsKey == "/root/lantern.txt" {
			mainSel = r
			break
		}
	}
	if mainSel.Value.AbsKey == "" {
		t.Fatal("mainSel lantern.txt row not found")
	}

	meadow := DedupCopyFilesUnderDir(snap, mainSel, "meadow")
	if len(meadow) != 2 {
		t.Fatalf("meadow files = %d, want 2", len(meadow))
	}
	meadowRels := map[string]bool{meadow[0].Rel: true, meadow[1].Rel: true}
	if !meadowRels["meadow/beacon.txt"] || !meadowRels["meadow/lantern.txt"] {
		t.Fatalf("meadow files = %v, want beacon + lantern under meadow", meadowRels)
	}

	orchard := DedupCopyFilesUnderDir(snap, mainSel, "orchard")
	if len(orchard) != 1 || orchard[0].Rel != "orchard/lantern.txt" {
		t.Fatalf("orchard files = %v, want orchard/lantern.txt", orchard)
	}

	if got := DedupCopyFilesUnderDir(snap, DedupRow{Value: DedupRowData{Kind: DedupRowDir}}, "meadow"); got != nil {
		t.Fatalf("dir mainSel = %v, want nil", got)
	}

	all := DedupCopyPaneFiles(snap, mainSel)
	if len(all) != 3 {
		t.Fatalf("DedupCopyPaneFiles = %d, want 3 other copies", len(all))
	}
}

func TestDedupGroupFilesUnderDir(t *testing.T) {
	g := dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt", "meadow/beacon.txt", "orchard/lantern.txt")
	meadow := DedupGroupFilesUnderDir(g, "meadow")
	if len(meadow) != 2 {
		t.Fatalf("meadow files = %d, want 2", len(meadow))
	}
	if got := DedupGroupFilesUnderDir(g, "orchard"); len(got) != 1 || got[0].Rel != "orchard/lantern.txt" {
		t.Fatalf("orchard files = %v, want orchard/lantern.txt", got)
	}
	if got := DedupGroupFilesUnderDir(g, "missing"); len(got) != 0 {
		t.Fatalf("missing dir = %v, want empty", got)
	}
}

func TestDedupSnapshotFilesUnderDir(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt"),
			dedupTestGroup(2, 50, "meadow/beacon.txt", "orchard/beacon.txt"),
		},
	}
	byGroup := DedupSnapshotFilesUnderDir(snap, "meadow")
	if len(byGroup) != 2 {
		t.Fatalf("groups under meadow = %d, want 2", len(byGroup))
	}
	if len(byGroup[0]) != 1 || byGroup[0][0].Rel != "meadow/lantern.txt" {
		t.Fatalf("group 0 meadow files = %v", byGroup[0])
	}
	if len(byGroup[1]) != 1 || byGroup[1][0].Rel != "meadow/beacon.txt" {
		t.Fatalf("group 1 meadow files = %v", byGroup[1])
	}
	if got := DedupSnapshotFilesUnderDir(snap, "missing"); got != nil {
		t.Fatalf("missing dir groups = %v, want nil", got)
	}
}

func TestDedupKeptDirSet(t *testing.T) {
	kept := map[string]bool{
		"/root/meadow/copper/anchor": true,
		"/root/harbor/anchor":        true,
	}
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "meadow/copper/anchor", "harbor/anchor"),
		},
	}
	got := dedupKeptDirSet(snap, kept)
	if len(got) != 3 || !got["meadow"] || !got["meadow/copper"] || !got["harbor"] {
		t.Fatalf("kept dirs = %v, want meadow + meadow/copper + harbor", got)
	}
	if dedupKeptDirSet(snap, nil) != nil {
		t.Fatal("nil kept should return nil dir set")
	}
}

func TestDedupCopyDirFullyMarked(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt", "meadow/beacon.txt", "orchard/lantern.txt"),
		},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	var mainSel DedupRow
	for _, r := range rows {
		if r.Value.AbsKey == "/root/lantern.txt" {
			mainSel = r
			break
		}
	}
	marked := map[string]bool{
		"/root/meadow/lantern.txt": true,
		"/root/meadow/beacon.txt":  true,
	}
	if !DedupCopyDirFullyMarked(snap, mainSel, "meadow", marked) {
		t.Fatal("meadow with all copies marked: want true")
	}
	if DedupCopyDirFullyMarked(snap, mainSel, "orchard", marked) {
		t.Fatal("orchard with unmarked copy: want false")
	}
	if DedupCopyDirFullyMarked(snap, mainSel, "meadow", nil) {
		t.Fatal("nil marks: want false")
	}
}

func TestDedupCopyPaneFullyMarkedDirSet(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt", "meadow/beacon.txt", "orchard/deep/lantern.txt"),
		},
	}
	rows, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	var mainSel DedupRow
	for _, r := range rows {
		if r.Value.AbsKey == "/root/lantern.txt" {
			mainSel = r
			break
		}
	}
	partial := map[string]bool{"/root/meadow/lantern.txt": true}
	got := dedupCopyPaneFullyMarkedDirSet(snap, mainSel, partial)
	if len(got) != 0 {
		t.Fatalf("partial marks = %v, want none fully marked", got)
	}
	fullMeadow := map[string]bool{
		"/root/meadow/lantern.txt": true,
		"/root/meadow/beacon.txt":  true,
	}
	got = dedupCopyPaneFullyMarkedDirSet(snap, mainSel, fullMeadow)
	if len(got) != 1 || !got["meadow"] {
		t.Fatalf("meadow fully marked = %v, want only meadow", got)
	}
	fullAll := map[string]bool{
		"/root/meadow/lantern.txt":       true,
		"/root/meadow/beacon.txt":        true,
		"/root/orchard/deep/lantern.txt": true,
	}
	got = dedupCopyPaneFullyMarkedDirSet(snap, mainSel, fullAll)
	if len(got) != 3 || !got["meadow"] || !got["orchard"] || !got["orchard/deep"] {
		t.Fatalf("all copies marked = %v, want meadow + orchard + orchard/deep", got)
	}
}

func TestDedupSnapshotFullyMarkedDirSet(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 100, "lantern.txt", "meadow/lantern.txt", "orchard/lantern.txt"),
			dedupTestGroup(2, 200, "meadow/beacon.txt", "orchard/beacon.txt"),
		},
	}
	partial := map[string]bool{"/root/meadow/lantern.txt": true}
	got := dedupSnapshotFullyMarkedDirSet(snap, partial)
	if len(got) != 0 {
		t.Fatalf("partial marks = %v, want none fully marked", got)
	}
	fullMeadow := map[string]bool{
		"/root/meadow/lantern.txt": true,
		"/root/meadow/beacon.txt":  true,
	}
	got = dedupSnapshotFullyMarkedDirSet(snap, fullMeadow)
	if len(got) != 1 || !got["meadow"] {
		t.Fatalf("meadow fully marked = %v, want only meadow", got)
	}
	fullAll := map[string]bool{
		"/root/lantern.txt":         true,
		"/root/meadow/lantern.txt":  true,
		"/root/meadow/beacon.txt":   true,
		"/root/orchard/lantern.txt": true,
		"/root/orchard/beacon.txt":  true,
	}
	got = dedupSnapshotFullyMarkedDirSet(snap, fullAll)
	if len(got) != 2 || !got["meadow"] || !got["orchard"] {
		t.Fatalf("all files marked = %v, want meadow + orchard", got)
	}
}

func TestDedupNextDirRowIndex(t *testing.T) {
	rows := []DedupRow{
		{Value: DedupRowData{Kind: DedupRowDir, DirRel: "meadow"}, Depth: 0},
		{Value: DedupRowData{Kind: DedupRowFile, Display: "beacon"}, Depth: 1},
		{Value: DedupRowData{Kind: DedupRowFile, Display: "lantern"}, Depth: 1},
		{Value: DedupRowData{Kind: DedupRowDir, DirRel: "orchard"}, Depth: 0},
		{Value: DedupRowData{Kind: DedupRowFile, Display: "lantern"}, Depth: 1},
	}
	if got := DedupSubtreeEndIndex(rows, 0); got != 3 {
		t.Fatalf("subtree end at meadow = %d, want 3", got)
	}
	if got := DedupNextDirRowIndex(rows, 0); got != 3 {
		t.Fatalf("next dir after meadow = %d, want 3 (orchard)", got)
	}
	if got := DedupNextDirRowIndex(rows, 3); got != 5 {
		t.Fatalf("next dir after orchard = %d, want 5 (past end)", got)
	}
}
