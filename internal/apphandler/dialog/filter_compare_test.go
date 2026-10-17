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
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: compare.FilterAll},
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

func TestFilterDialogDirsOnlyDisablesTypes(t *testing.T) {
	t.Parallel()
	model := &ui.Model{}
	h := New(Deps{Host: &identityTestHost{model: model, cfg: config.Default()}, Model: model})
	fd := &model.FilterDialog
	fd.DirsOnly = true
	fd.Types[0] = true
	if h.toggleFilterDialogField(fd, dialog.FilterFocusTypeFirst+1) || fd.Types[1] {
		t.Fatal("type toggle must be a no-op with Directories only")
	}
	if spec := h.filterSpec(); spec.Types != nil || spec.TypeLabel != "" {
		t.Fatalf("spec types = %v %q, want ignored", spec.Types, spec.TypeLabel)
	}
	if !h.toggleFilterDialogField(fd, dialog.FilterFocusFilesOnly) || !fd.FilesOnly || fd.DirsOnly {
		t.Fatal("Files only radio must switch away from Directories only")
	}
	if spec := h.filterSpec(); spec.Types == nil {
		t.Fatal("checked type must apply again once Directories only is off")
	}
}
