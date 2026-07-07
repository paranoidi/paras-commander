package ui

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func syntheticDedupSnapshot(groups int) comparepkg.DedupSnapshot {
	root := pathloc.MustParse("/scan/root")
	snap := comparepkg.DedupSnapshot{
		Root:  root,
		Phase: comparepkg.DedupDone,
	}
	for g := range groups {
		relA := fmt.Sprintf("group-%d/a.bin", g)
		relB := fmt.Sprintf("group-%d/b.bin", g)
		snap.Groups = append(snap.Groups, comparepkg.DedupGroup{
			Size: 4096,
			Files: []comparepkg.DedupFile{
				{Rel: relA, Abs: pathloc.MustParse("/scan/root/" + relA)},
				{Rel: relB, Abs: pathloc.MustParse("/scan/root/" + relB)},
			},
		})
	}
	return snap
}

func BenchmarkDrawDedupViewLargeList(b *testing.B) {
	const groups = 45464
	snap := syntheticDedupSnapshot(groups)
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
		drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal)
	}
}
