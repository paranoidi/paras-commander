package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestFilterDialogCompareTarget(t *testing.T) {
	t.Parallel()
	model := &ui.Model{
		ViewMode: ui.ViewCompare,
		CompareSnapshot: compare.Snapshot{Rows: []compare.Row{
			{Kind: compare.KindPrimaryOnly, PrimaryRel: "orchard/lemon.txt", Size: 1},
			{Kind: compare.KindSecondaryOnly, SecondaryRel: "meadow/maple.txt", Size: 1},
			{Kind: compare.KindEqual, PrimaryRel: "meadow/cedar.jpg", SecondaryRel: "meadow/cedar.jpg", Size: 1},
		}},
	}
	h := New(Deps{Host: &identityTestHost{model: model, cfg: config.Default()}, Model: model})

	h.OpenFilterDialog()
	if model.FilterDialog.Target != dialog.FilterTargetCompare || model.FilterDialog.MetaColumnCount != 0 {
		t.Fatalf("dialog = %+v, want Compare target without meta columns", model.FilterDialog)
	}
	model.FilterDialog.Text = "*.txt"
	h.updateFilterDialogPreview()
	if !model.FilterDialog.PreviewShow || model.FilterDialog.PreviewFiles != 2 {
		t.Fatalf("preview = %+v, want 2 files", model.FilterDialog)
	}
	h.executeFilterDialog()
	if model.FilterDialog.Open {
		t.Fatal("dialog should close on OK")
	}
	rows := compare.FilteredRows(model.CompareSnapshot, model.CompareView.Filter, false, model.CompareView.NameFilter)
	if len(rows) != 2 || model.CompareView.NameFilterLabel == "" {
		t.Fatalf("rows = %+v label = %q, want the two .txt rows", rows, model.CompareView.NameFilterLabel)
	}

	h.OpenFilterDialog()
	h.executeFilterDialog()
	if model.CompareView.NameFilter != nil || model.CompareView.NameFilterLabel != "" {
		t.Fatal("empty OK should clear the name filter")
	}
}
