package dedup

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// keepDirFixture scans two groups: widget.txt in amber, birch and cedar; gadget.txt in amber and
// cedar. Directory rows are amber, birch and cedar (tree dirs view).
func keepDirFixture(t *testing.T) (*Handler, *ui.Model) {
	t.Helper()
	root := pathloc.MustParse("/scan")
	snap := comparepkg.DedupSnapshot{
		Root: root, DisplayRoot: root, Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			{Size: 100, Files: []comparepkg.DedupFile{dedupFile("amber/widget.txt"), dedupFile("birch/widget.txt"), dedupFile("cedar/widget.txt")}},
			{Size: 50, Files: []comparepkg.DedupFile{dedupFile("amber/gadget.txt"), dedupFile("cedar/gadget.txt")}},
		},
	}
	return dedupHandlerWithView(t, snap, ui.DedupViewState{TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{}})
}

func selectDirRow(t *testing.T, h *Handler, model *ui.Model, rel string) {
	t.Helper()
	for i, r := range model.DedupList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == rel {
			model.DedupView.Main.Selected = i
			return
		}
	}
	t.Fatalf("no dir row %q", rel)
}

func keepFile(h *Handler, _ *ui.Model, rel string) {
	h.setKept("/scan/"+rel, true)
}

func TestKeepDirWithoutPriorKeepAppliesAtOnce(t *testing.T) {
	h, model := keepDirFixture(t)
	selectDirRow(t, h, model, "amber")
	h.KeepSelection()
	if model.DedupKeepConfirm.Open {
		t.Fatal("dialog opened without a keep elsewhere")
	}
	v := model.DedupView
	if !v.Kept["/scan/amber/widget.txt"] || !v.Kept["/scan/amber/gadget.txt"] {
		t.Fatalf("amber not kept: %v", v.Kept)
	}
	if !v.Marked["/scan/birch/widget.txt"] || !v.Marked["/scan/cedar/gadget.txt"] {
		t.Fatalf("other copies not marked: %v", v.Marked)
	}
}

func TestKeepDirAsksWhenKeepElsewhere(t *testing.T) {
	h, model := keepDirFixture(t)
	keepFile(h, model, "birch/widget.txt") // group 0 kept elsewhere; group 1 untouched
	selectDirRow(t, h, model, "amber")
	before := model.DedupView.Main.Selected
	h.KeepSelection()
	dlg := model.DedupKeepConfirm
	if !dlg.Open || dlg.Groups != 1 || len(dlg.Kept) != 1 || dlg.Kept[0] != "birch/widget.txt" || dlg.Dir != "amber" {
		t.Fatalf("dialog = %+v", dlg)
	}
	if model.DedupView.Main.Selected != before || model.DedupView.Kept["/scan/amber/widget.txt"] || model.DedupView.Kept["/scan/amber/gadget.txt"] {
		t.Fatal("state changed before the answer")
	}
	h.CancelDirKeep()
	if model.DedupView.Kept["/scan/amber/gadget.txt"] || !model.DedupView.Kept["/scan/birch/widget.txt"] || model.DedupView.Main.Selected != before {
		t.Fatalf("cancel changed state: %v", model.DedupView.Kept)
	}
	h.ResolveDirKeep(true) // nothing pending any more
	if model.DedupView.Kept["/scan/amber/gadget.txt"] {
		t.Fatal("resolve without a pending keep applied something")
	}
}

func TestKeepDirKeepThisReplacesOldKeep(t *testing.T) {
	h, model := keepDirFixture(t)
	keepFile(h, model, "birch/widget.txt")
	selectDirRow(t, h, model, "amber")
	before := model.DedupView.Main.Selected
	h.KeepSelection()
	h.ResolveDirKeep(true)
	v := model.DedupView
	if v.Kept["/scan/birch/widget.txt"] || !v.Marked["/scan/birch/widget.txt"] {
		t.Fatalf("old keep not replaced: kept=%v marked=%v", v.Kept, v.Marked)
	}
	if !v.Kept["/scan/amber/widget.txt"] || !v.Kept["/scan/amber/gadget.txt"] || !v.Marked["/scan/cedar/gadget.txt"] {
		t.Fatalf("directory not kept: kept=%v marked=%v", v.Kept, v.Marked)
	}
	if v.Main.Selected == before {
		t.Fatal("cursor did not step past the folder")
	}
}

func TestKeepDirKeepPreviousLeavesConflictingGroup(t *testing.T) {
	h, model := keepDirFixture(t)
	keepFile(h, model, "birch/widget.txt")
	h.setMark("/scan/amber/widget.txt", 100, true)
	h.setMark("/scan/cedar/widget.txt", 100, true)
	selectDirRow(t, h, model, "amber")
	h.KeepSelection()
	h.ResolveDirKeep(false)
	v := model.DedupView
	if !v.Kept["/scan/birch/widget.txt"] || v.Kept["/scan/amber/widget.txt"] || !v.Marked["/scan/amber/widget.txt"] {
		t.Fatalf("conflicting group touched: kept=%v marked=%v", v.Kept, v.Marked)
	}
	if !v.Kept["/scan/amber/gadget.txt"] || !v.Marked["/scan/cedar/gadget.txt"] {
		t.Fatalf("non-conflicting group not kept: kept=%v marked=%v", v.Kept, v.Marked)
	}
}

func TestKeepDirToggleOffIsNoConflict(t *testing.T) {
	h, model := keepDirFixture(t)
	selectDirRow(t, h, model, "amber")
	h.KeepSelection()
	selectDirRow(t, h, model, "amber")
	h.KeepSelection()
	if model.DedupKeepConfirm.Open || len(model.DedupView.Kept) != 0 {
		t.Fatalf("toggle off: open=%v kept=%v", model.DedupKeepConfirm.Open, model.DedupView.Kept)
	}
}

func TestKeepCopiesDirAsks(t *testing.T) {
	h, model := keepDirFixture(t)
	keepFile(h, model, "birch/widget.txt")
	model.DedupView.Main.Selected = ui.DedupRowIndexByID(model.DedupList, "/scan/amber/widget.txt")
	h.syncCopies()
	for i, r := range model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "cedar" {
			model.DedupView.Copies.Selected = i
		}
	}
	model.DedupView.FocusCopies = true
	h.KeepSelection()
	if !model.DedupKeepConfirm.Open || model.DedupKeepConfirm.Dir != "cedar" {
		t.Fatalf("dialog = %+v", model.DedupKeepConfirm)
	}
	h.ResolveDirKeep(false)
	if !model.DedupView.Kept["/scan/birch/widget.txt"] || model.DedupView.Kept["/scan/cedar/widget.txt"] {
		t.Fatalf("keep previous changed the group: %v", model.DedupView.Kept)
	}
	h.KeepSelection()
	h.ResolveDirKeep(true)
	if model.DedupView.Kept["/scan/birch/widget.txt"] || !model.DedupView.Kept["/scan/cedar/widget.txt"] {
		t.Fatalf("keep this did not replace: %v", model.DedupView.Kept)
	}
}
