package ui

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/testutil"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func BenchmarkDrawDedupViewLargeList(b *testing.B) {
	const groups = 45464
	snap := testutil.SyntheticDedupSnapshot(groups)
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if len(list) != groups*2 {
		b.Fatalf("list len = %d, want %d", len(list), groups*2)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)

	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 60, Height: 36},
		Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 36},
	}
	view := DedupViewState{Main: DedupPane{Selected: 1}, Marked: map[string]bool{}}
	copies := DedupCopyRows(snap, list[1], nil, nil)
	styles := theme.Default()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal, nil, ListRowHover{})
	}
}

// dedupNestedBenchSnapshot builds groups three-file groups under a five-deep
// directory tree with word-based names.
func dedupNestedBenchSnapshot(groups int) comparepkg.DedupSnapshot {
	words := []string{"amber", "birch", "cedar", "dune", "ember", "fjord", "grove", "harbor", "iris", "juniper", "kettle", "lantern"}
	snap := comparepkg.DedupSnapshot{Root: pathloc.MustParse("/scan/root"), Phase: comparepkg.DedupDone}
	for g := range groups {
		files := make([]comparepkg.DedupFile, 3)
		for c := range files {
			rel := fmt.Sprintf("%s/%s/%s/%s/%s/%s-%d.bin",
				words[(g+c)%12], words[(g/12+c)%12], words[(g/144+c)%12],
				words[(g/1728+c)%12], words[(g/7+c)%12], words[g%12], g)
			files[c] = comparepkg.DedupFile{Rel: rel, Abs: pathloc.MustParse("/scan/root/" + rel)}
		}
		snap.Groups = append(snap.Groups, comparepkg.DedupGroup{Hash: [32]byte{byte(g), byte(g >> 8), byte(g >> 16)}, Size: 4096, Files: files})
	}
	return snap
}

// BenchmarkDedupRowsFromSnapshotDirsCached is one expand/collapse toggle in dirs
// mode from the all-collapsed start state.
func BenchmarkDedupRowsFromSnapshotDirsCached(b *testing.B) {
	snap := dedupNestedBenchSnapshot(40000)
	view := DedupViewState{TreeDirs: true, IgnoreEmpty: true, Cache: &DedupCache{}}
	ids := DedupExpandableIDs(snap, view)
	view.Main.Collapsed = DedupCollapsedSet(ids)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		view.Main.SetCollapsed(ids[0], i%2 == 0)
		DedupRowsFromSnapshot(snap, view)
	}
}

// BenchmarkDedupCopyRowsTwinDirs is one cursor move onto a duplicate directory
// with thousands of twins: the copies pane keeps every file under any twin.
func BenchmarkDedupCopyRowsTwinDirs(b *testing.B) {
	const dirs = 3000
	words := []string{"amber", "birch", "cedar", "dune", "ember", "fjord", "grove", "harbor", "iris", "juniper", "kettle", "lantern"}
	snap := comparepkg.DedupSnapshot{Root: pathloc.MustParse("/scan/root"), Phase: comparepkg.DedupDone}
	grp := comparepkg.DedupGroup{Hash: [32]byte{1}, Size: 10240}
	tops := make([]string, 0, dirs)
	subs := make([]string, 0, dirs)
	for i := range dirs {
		top := fmt.Sprintf("data/%04d_%s_%s", i, words[i%12], words[(i/12)%12])
		tops = append(tops, top)
		subs = append(subs, top+"/sub")
		for _, rel := range []string{"file1.dat", "file2.dat", "sub/file1.dat", "sub/file2.dat"} {
			rel = top + "/" + rel
			grp.Files = append(grp.Files, comparepkg.DedupFile{Rel: rel, Abs: pathloc.MustParse("/scan/root/" + rel)})
		}
	}
	snap.Groups = []comparepkg.DedupGroup{grp}
	snap.DirGroups = []comparepkg.DedupDirGroup{
		{Kind: comparepkg.DirExact, Size: 20480, Files: 2, Rels: subs, Hidden: make([]bool, dirs)},
		{Kind: comparepkg.DirExact, Size: 40960, Files: 4, Rels: tops, Hidden: make([]bool, dirs)},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{TreeDirs: true})
	sel := -1
	for i, r := range list {
		if r.Value.Kind == DedupRowDir && DedupTwinDirs(snap, r.Value.DirRel, nil) != nil {
			sel = i
			break
		}
	}
	if sel < 0 {
		b.Fatal("no duplicate-dir row")
	}
	cache := &DedupCache{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		DedupCopyRows(snap, list[sel], nil, cache)
	}
}

// benchDrawDedupExpanded is one frame of a fully open tree with a tenth of the files marked.
func benchDrawDedupExpanded(b *testing.B, treeDirs bool) {
	snap := dedupNestedBenchSnapshot(40000)
	view := DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true, Cache: &DedupCache{}, Marked: map[string]bool{}, Kept: map[string]bool{}, Main: DedupPane{Selected: 3}}
	for g := 0; g < len(snap.Groups); g += 10 {
		view.Marked[snap.Groups[g].Files[0].Abs.String()] = true
	}
	view.MarksRev++
	list, _ := DedupRowsFromSnapshot(snap, view)
	copies := DedupCopyRows(snap, list[3], nil, nil)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)
	layout := Layout{Primary: Rect{X: 0, Y: 1, Width: 60, Height: 36}, Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 36}}
	styles := theme.Default()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal, nil, ListRowHover{})
	}
}

func BenchmarkDrawDedupExpandedDirs(b *testing.B)   { benchDrawDedupExpanded(b, true) }
func BenchmarkDrawDedupExpandedGroups(b *testing.B) { benchDrawDedupExpanded(b, false) }

// BenchmarkDedupCopyRowsSmallTwinDirInLargeSnapshot is one cursor move onto a
// content twin dir holding three files, in a 40k-group snapshot with thousands
// of other dir groups.
func BenchmarkDedupCopyRowsSmallTwinDirInLargeSnapshot(b *testing.B) {
	snap := dedupNestedBenchSnapshot(40000)
	var smallFiles []comparepkg.DedupFile
	for _, rel := range []string{"twin_alpha/a.dat", "twin_alpha/b.dat", "twin_alpha/c.dat", "twin_beta/a.dat", "twin_beta/b.dat", "twin_beta/c.dat"} {
		smallFiles = append(smallFiles, comparepkg.DedupFile{Rel: rel, Abs: pathloc.MustParse("/scan/root/" + rel)})
	}
	for i := range 3 {
		snap.Groups = append(snap.Groups, comparepkg.DedupGroup{Hash: [32]byte{9, byte(i)}, Size: 100, Files: []comparepkg.DedupFile{smallFiles[i], smallFiles[i+3]}})
	}
	for i := range 2000 {
		a, c := fmt.Sprintf("filler/%04d_a", i), fmt.Sprintf("filler/%04d_b", i)
		snap.DirGroups = append(snap.DirGroups, comparepkg.DedupDirGroup{Kind: comparepkg.DirContent, Size: 1, Files: 1, Rels: []string{a, c}, Hidden: []bool{false, false}})
	}
	snap.DirGroups = append(snap.DirGroups, comparepkg.DedupDirGroup{Kind: comparepkg.DirContent, Size: 300, Files: 3, Rels: []string{"twin_alpha", "twin_beta"}, Hidden: []bool{false, false}})
	cache := &DedupCache{}
	sel := DedupRow{Value: &DedupRowData{Kind: DedupRowDir, DirRel: "twin_alpha"}}
	if n := len(DedupCopyRows(snap, sel, nil, cache)); n != 4 {
		b.Fatalf("rows = %d, want 4", n)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		DedupCopyRows(snap, sel, nil, cache)
	}
}
