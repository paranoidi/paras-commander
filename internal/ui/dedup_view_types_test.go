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

func TestDedupRedundantUnder(t *testing.T) {
	// g1: one copy under backup/, one outside → the backup copy is redundant.
	// g2: both copies under backup/ → keep the first, drop the second.
	// g3: entirely outside backup/ → untouched.
	groups := []comparepkg.DedupGroup{
		dedupTestGroup(1, 10, "backup/copper", "keep/copper"),
		dedupTestGroup(2, 10, "backup/inner/willow", "backup/other/willow"),
		dedupTestGroup(3, 10, "keep/maple", "elsewhere/maple"),
	}

	got := DedupRedundantUnder(groups, "/root/backup")
	want := map[string]bool{"/root/backup/copper": true, "/root/backup/other/willow": true}
	if len(got) != len(want) {
		t.Fatalf("marked %v, want keys %v", got, want)
	}
	for _, k := range got {
		if !want[k] {
			t.Fatalf("unexpected mark %q (all: %v)", k, got)
		}
	}
}

func TestDedupDuplicatesUnder(t *testing.T) {
	// g1: one copy under backup/, one outside → the backup copy is stored elsewhere.
	// g2: both copies under backup/ → no external survivor, leave untouched.
	groups := []comparepkg.DedupGroup{
		dedupTestGroup(1, 10, "backup/copper", "keep/copper"),
		dedupTestGroup(2, 10, "backup/inner/willow", "backup/other/willow"),
	}

	got := DedupDuplicatesUnder(groups, "/root/backup")
	if len(got) != 1 || got[0] != "/root/backup/copper" {
		t.Fatalf("marked %v, want only /root/backup/copper", got)
	}
	// A sibling directory prefix ("/root/back") must not match "/root/backup".
	if got := DedupDuplicatesUnder(groups, "/root/back"); len(got) != 0 {
		t.Fatalf("prefix-collision marks = %v, want none", got)
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
