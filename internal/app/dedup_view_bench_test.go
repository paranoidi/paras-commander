package app

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
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

func benchDedupApp(b *testing.B, groups int) *App {
	b.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(screen.Fini)
	screen.SetSize(120, 40)

	dir := b.TempDir()
	app, err := New(screen, func() (string, error) { return dir, nil })
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(app.stopWorker)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.ViewMode = ui.ViewDedup
	app.model.DedupSnapshot = syntheticDedupSnapshot(groups)
	app.model.DedupView = ui.DedupViewState{Marked: map[string]bool{}, IgnoreEmpty: true, TreeDirs: true}
	app.model.DedupList, _ = ui.DedupRowsFromSnapshot(app.model.DedupSnapshot, app.model.DedupView)
	return app
}

func BenchmarkDedupViewNavDown(b *testing.B) {
	const groups = 45464
	app := benchDedupApp(b, groups)
	if len(app.model.DedupList) != groups*2 {
		b.Fatalf("DedupList = %d, want %d", len(app.model.DedupList), groups*2)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.model.DedupView.Main.Selected = 0
		app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
}
