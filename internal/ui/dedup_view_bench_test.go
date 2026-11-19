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
	copies := DedupCopyRows(snap, list[1], nil)
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
