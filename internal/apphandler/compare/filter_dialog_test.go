package compare

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestFilterDialogArrowDoesNotApplyUntilOK(t *testing.T) {
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll},
		Primary:     panelStateAt(pathloc.MustParse("/a")),
		Secondary:   panelStateAt(pathloc.MustParse("/b")),
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	h.OpenFilterDialog()

	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if model.CompareView.Filter != comparepkg.FilterAll {
		t.Fatalf("after Down: view Filter = %v, want All (pending only)", model.CompareView.Filter)
	}
	if model.CompareFilterDialog.Filter != comparepkg.FilterAll {
		t.Fatalf("after Down: dialog Filter = %v, want All (unchanged until Space)", model.CompareFilterDialog.Filter)
	}
	if model.CompareFilterDialog.Focus != dialog.FocusForCompareFilter(comparepkg.FilterEqual) {
		t.Fatalf("after Down: Focus = %d, want Equal radio", model.CompareFilterDialog.Focus)
	}

	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if model.CompareFilterDialog.Filter != comparepkg.FilterEqual {
		t.Fatalf("after Space: dialog Filter = %v, want Equal", model.CompareFilterDialog.Filter)
	}
	if model.CompareView.Filter != comparepkg.FilterAll {
		t.Fatalf("after Space: view Filter = %v, want All until OK", model.CompareView.Filter)
	}

	for model.CompareFilterDialog.Focus != dialog.CompareFilterDialogOKIndex() {
		h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.CompareView.Filter != comparepkg.FilterEqual {
		t.Fatalf("after OK: view Filter = %v, want Equal", model.CompareView.Filter)
	}
	if model.CompareFilterDialog.Open {
		t.Fatal("dialog should be closed after OK")
	}
}

func TestFilterDialogCancelDiscardsPending(t *testing.T) {
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	h.OpenFilterDialog()
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if model.CompareView.Filter != comparepkg.FilterAll {
		t.Fatalf("after Esc: view Filter = %v, want All", model.CompareView.Filter)
	}
	if model.CompareFilterDialog.Open {
		t.Fatal("dialog should be closed after Esc")
	}
}
