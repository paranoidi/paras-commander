package dialog

import (
	"testing"

	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func dedupFilterGroup(size int64, rels ...string) compare.DedupGroup {
	g := compare.DedupGroup{Size: size}
	for _, r := range rels {
		g.Files = append(g.Files, compare.DedupFile{Rel: r, Abs: pathloc.MustParse("/scan/" + r)})
	}
	return g
}

func TestFilterDialogDedupTarget(t *testing.T) {
	t.Parallel()
	model := &ui.Model{
		ViewMode: ui.ViewDedup,
		DedupSnapshot: compare.DedupSnapshot{Phase: compare.DedupDone, Groups: []compare.DedupGroup{
			dedupFilterGroup(10, "orchard/lemon.txt", "meadow/lemon.jpg"),
			dedupFilterGroup(10, "orchard/maple.txt", "meadow/maple.txt"),
			dedupFilterGroup(10, "orchard/cedar.png", "meadow/cedar.png"),
		}},
	}
	host := &identityTestHost{model: model, cfg: config.Default()}
	dd := dedupctrl.New(dedupctrl.Deps{Host: nil, Model: model})
	h := New(Deps{Host: host, Model: model, Dedup: dd})

	h.OpenFilterDialog()
	if model.FilterDialog.Target != dialog.FilterTargetDedup || model.FilterDialog.MetaColumnCount != 0 {
		t.Fatalf("dialog = %+v, want dedup target without meta columns", model.FilterDialog)
	}
	model.FilterDialog.Text = "*.jpg"
	h.updateFilterDialogPreview()
	if !model.FilterDialog.PreviewShow || model.FilterDialog.PreviewFiles != 2 {
		t.Fatalf("preview = %+v, want the 2 files of the one group with a .jpg copy", model.FilterDialog)
	}
	model.FilterDialog.Invert = true
	h.updateFilterDialogPreview()
	if model.FilterDialog.PreviewFiles != 4 {
		t.Fatalf("inverted preview files = %d, want 4", model.FilterDialog.PreviewFiles)
	}
	model.FilterDialog.Invert = false
	h.executeFilterDialog()
	if model.FilterDialog.Open || len(ui.DedupActiveGroups(model.DedupSnapshot, model.DedupView)) != 1 || model.DedupView.NameFilterLabel == "" {
		t.Fatalf("after OK: open=%v view=%+v, want one visible group and a label", model.FilterDialog.Open, model.DedupView)
	}

	h.OpenFilterDialog()
	h.executeFilterDialog()
	if model.DedupView.GroupFilter != nil || model.DedupView.NameFilterLabel != "" {
		t.Fatal("empty OK should clear the group filter")
	}
}
