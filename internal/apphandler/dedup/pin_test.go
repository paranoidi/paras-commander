package dedup

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSelectedPinTargetFileRow(t *testing.T) {
	root := pathloc.MustParse("/scan")
	fMain := dedupFile("alpha/widget.txt")
	fCopy := dedupFile("beta/widget.txt")
	snap := dedupDoneSnapshot(root, fMain, fCopy)
	view := ui.DedupViewState{
		TreeDirs: true,
		Marked:   map[string]bool{},
		Kept:     map[string]bool{},
	}
	h, model := dedupHandlerWithView(t, snap, view)

	mainIdx := ui.DedupRowIndexByID(model.DedupList, fMain.Abs.String())
	if mainIdx < 0 {
		t.Fatalf("main file row %q not found", fMain.Abs)
	}
	model.DedupView.Main.Selected = mainIdx

	path, isDir, ok := h.SelectedPinTarget()
	if !ok {
		t.Fatal("SelectedPinTarget: ok = false, want true for a file row")
	}
	if isDir {
		t.Fatal("SelectedPinTarget: isDir = true, want false for a file row")
	}
	if want := fMain.Abs.String(); path != want {
		t.Fatalf("SelectedPinTarget path = %q, want %q", path, want)
	}
}

func TestSelectedPinTargetDirRow(t *testing.T) {
	root := pathloc.MustParse("/scan")
	snap := dedupDoneSnapshot(root, dedupFile("alpha/widget.txt"), dedupFile("beta/widget.txt"))
	view := ui.DedupViewState{
		TreeDirs: true,
		Marked:   map[string]bool{},
		Kept:     map[string]bool{},
	}
	h, model := dedupHandlerWithView(t, snap, view)

	var dirIdx = -1
	for i, row := range model.DedupList {
		if row.Value.Kind == ui.DedupRowDir && row.Value.DirRel == "alpha" {
			dirIdx = i
			break
		}
	}
	if dirIdx < 0 {
		t.Fatal("main list missing alpha directory row")
	}
	model.DedupView.Main.Selected = dirIdx

	path, isDir, ok := h.SelectedPinTarget()
	if !ok {
		t.Fatal("SelectedPinTarget: ok = false, want true for a directory row")
	}
	if !isDir {
		t.Fatal("SelectedPinTarget: isDir = false, want true for a directory row")
	}
	if want := "/scan/alpha"; path != want {
		t.Fatalf("SelectedPinTarget path = %q, want %q", path, want)
	}
}

func TestSelectedPinTargetNoSelection(t *testing.T) {
	root := pathloc.MustParse("/scan")
	snap := comparepkg.DedupSnapshot{Root: root, DisplayRoot: root, Phase: comparepkg.DedupDone}
	view := ui.DedupViewState{Marked: map[string]bool{}, Kept: map[string]bool{}}
	h, _ := dedupHandlerWithView(t, snap, view)

	if _, _, ok := h.SelectedPinTarget(); ok {
		t.Fatal("SelectedPinTarget: ok = true, want false with an empty list")
	}
}
