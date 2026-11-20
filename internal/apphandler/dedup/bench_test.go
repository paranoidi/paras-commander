package dedup

import (
	"fmt"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// benchSnapshot is groups three-file groups under a five-deep word-named tree.
func benchSnapshot(groups int) comparepkg.DedupSnapshot {
	words := []string{"amber", "birch", "cedar", "dune", "ember", "fjord", "grove", "harbor", "iris", "juniper", "kettle", "lantern"}
	root := pathloc.MustParse("/scan/root")
	snap := comparepkg.DedupSnapshot{Root: root, DisplayRoot: root, Phase: comparepkg.DedupDone}
	for g := range groups {
		files := make([]comparepkg.DedupFile, 3)
		for c := range files {
			rel := fmt.Sprintf("%s/%s/%s/%s/%s/%s-%d.bin",
				words[(g+c)%12], words[(g/12+c)%12], words[(g/144+c)%12],
				words[(g/1728+c)%12], words[(g/7+c)%12], words[g%12], g)
			files[c] = comparepkg.DedupFile{Rel: rel, Abs: pathloc.MustParse("/scan/root/" + rel)}
		}
		snap.Groups = append(snap.Groups, comparepkg.DedupGroup{Hash: [32]byte{byte(g), byte(g >> 8), byte(g >> 16)}, Size: int64(4096 + g), Files: files})
	}
	return snap
}

// benchToggle is one expand/collapse of the cursor row through the handler (rows, copies pane,
// cursor restore), from the all-collapsed start state with a tenth of the files marked.
func benchToggle(b *testing.B, treeDirs bool) {
	snap := benchSnapshot(40000)
	view := ui.DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true, Cache: &ui.DedupCache{}, Marked: map[string]bool{}, Kept: map[string]bool{},
		DirsCollapsePending: true, GroupsCollapsePending: true}
	for g := 0; g < len(snap.Groups); g += 10 {
		view.Marked[snap.Groups[g].Files[0].Abs.String()] = true
	}
	model := &ui.Model{ViewMode: ui.ViewDedup, DedupSnapshot: snap, DedupView: view}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.syncDedupList()
	if r, ok := h.selectedRow(); !ok || !r.HasChildren {
		b.Fatal("cursor row not expandable")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		h.toggleExpandableNode()
	}
}

func BenchmarkHandlerToggleDirs(b *testing.B)   { benchToggle(b, true) }
func BenchmarkHandlerToggleGroups(b *testing.B) { benchToggle(b, false) }

// benchExpandAll is the expand-all action from the all-collapsed state.
func benchExpandAll(b *testing.B, treeDirs bool) {
	snap := benchSnapshot(40000)
	model := &ui.Model{ViewMode: ui.ViewDedup, DedupSnapshot: snap, DedupView: ui.DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true, Cache: &ui.DedupCache{}}}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.syncDedupList()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		h.CollapseAll()
		b.StartTimer()
		h.ExpandAll()
	}
}

func BenchmarkHandlerExpandAllDirs(b *testing.B)   { benchExpandAll(b, true) }
func BenchmarkHandlerExpandAllGroups(b *testing.B) { benchExpandAll(b, false) }

// benchCollapseAll is the collapse-all action from the fully expanded state.
func benchCollapseAll(b *testing.B, treeDirs bool) {
	snap := benchSnapshot(40000)
	model := &ui.Model{ViewMode: ui.ViewDedup, DedupSnapshot: snap, DedupView: ui.DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true, Cache: &ui.DedupCache{}}}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.syncDedupList()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		h.ExpandAll()
		b.StartTimer()
		h.CollapseAll()
	}
}

func BenchmarkHandlerCollapseAllDirs(b *testing.B)   { benchCollapseAll(b, true) }
func BenchmarkHandlerCollapseAllGroups(b *testing.B) { benchCollapseAll(b, false) }

// benchToggleExpanded is one expand/collapse of the cursor row inside a fully open tree.
func benchToggleExpanded(b *testing.B, treeDirs bool) {
	snap := benchSnapshot(40000)
	model := &ui.Model{ViewMode: ui.ViewDedup, DedupSnapshot: snap, DedupView: ui.DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true, Cache: &ui.DedupCache{}}}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.syncDedupList()
	model.DedupView.Main.Collapsed = nil
	h.resyncPreservingCursor()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		h.toggleExpandableNode()
	}
}

func BenchmarkHandlerToggleExpandedDirs(b *testing.B)   { benchToggleExpanded(b, true) }
func BenchmarkHandlerToggleExpandedGroups(b *testing.B) { benchToggleExpanded(b, false) }
