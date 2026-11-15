package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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
	if model.FilterDialog.Text != "*.txt" || model.FilterDialog.PrefillPending {
		t.Fatalf("reopen text %q, want the active compare filter", model.FilterDialog.Text)
	}
	model.FilterDialog.Text = ""
	h.executeFilterDialog()
	if model.CompareView.NameFilter != nil || model.CompareView.NameFilterLabel != "" {
		t.Fatal("empty OK should clear the name filter")
	}
}

func TestFilterDialogScopeDirsDisablesTypes(t *testing.T) {
	t.Parallel()
	model := &ui.Model{}
	h := New(Deps{Host: &identityTestHost{model: model, cfg: config.Default()}, Model: model})
	fd := &model.FilterDialog
	fd.Scope = panel.ScopeDirs
	fd.Types[0] = true
	if fd.Apply(dlg.Event{ID: dialog.PatternTypeID(1)}) || fd.Types[1] {
		t.Fatal("type toggle must be a no-op with the Dirs scope")
	}
	if spec := h.filterSpec(); spec.Types != nil || spec.TypeLabel != "" {
		t.Fatalf("spec types = %v %q, want ignored", spec.Types, spec.TypeLabel)
	}
	if !fd.Apply(dlg.Event{ID: dialog.PatternScopeFiles}) || fd.Scope != panel.ScopeFiles {
		t.Fatal("Files radio must switch the scope away from Dirs")
	}
	if spec := h.filterSpec(); spec.Types == nil {
		t.Fatal("checked type must apply again once the scope is not Dirs")
	}
}

func TestFilterDialogReopenRestoresActiveFilterAndClear(t *testing.T) {
	t.Parallel()
	model := &ui.Model{}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 40)
	h := New(Deps{Host: &identityTestHost{model: model, cfg: config.Default()}, Model: model, Screen: screen})

	h.OpenFilterDialog()
	fd := &model.FilterDialog
	fd.Text, fd.PatternMode, fd.Invert, fd.Scope = "lantern", panel.GroupPatternRegex, true, panel.ScopeDirs
	fd.Types[dialog.FilterTypeAudio] = true
	h.executeFilterDialog()
	if model.Primary.ActiveEntryFilter == nil {
		t.Fatal("filter should be active")
	}

	h.OpenFilterDialog()
	if fd.Text != "lantern" || fd.TextCursor != len("lantern") || fd.PrefillPending || fd.PatternMode != panel.GroupPatternRegex ||
		!fd.Invert || fd.Scope != panel.ScopeDirs || !fd.Types[dialog.FilterTypeAudio] || fd.Focus != dialog.PatternInput {
		t.Fatalf("reopened state = %+v, want the applied settings", *fd)
	}

	if !fd.FilterActive {
		t.Fatal("Clear should be shown while the filter is active")
	}
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if fd.Open || model.Primary.ActiveEntryFilter != nil {
		t.Fatalf("Clear: open %v filter %v", fd.Open, model.Primary.ActiveEntryFilter)
	}
	h.OpenFilterDialog()
	if fd.FilterActive {
		t.Fatal("Clear should be hidden with no filter")
	}
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if !fd.Open {
		t.Fatal("Alt+F must not clear while Clear is hidden")
	}
	if fd.Text != "" || fd.Invert || fd.PatternMode != panel.GroupPatternSimple {
		t.Fatalf("after Clear the dialog should open fresh: %+v", *fd)
	}

	// A filter installed elsewhere (e.g. a git filter) replaces the remembered one.
	fd.Text = "harbor"
	h.executeFilterDialog()
	model.Primary.SetEntryFilter(&panel.EntryFilter{ID: "git", Match: func(localfs.Entry, *panel.State) bool { return true }})
	h.OpenFilterDialog()
	if fd.Text != "" {
		t.Fatalf("replaced filter must not be restored, text %q", fd.Text)
	}
	ApplyPatternDialogEvent(h.FilterPatternKeys(), dlg.Event{ID: dialog.PatternClearButton})
	if model.Primary.ActiveEntryFilter != nil {
		t.Fatal("Clear should remove any entry filter")
	}
}
