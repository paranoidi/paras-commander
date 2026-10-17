package compare

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	if model.CompareFilterDialog.Focus != model.CompareFilterDialog.FocusForFilter(comparepkg.FilterEqual) {
		t.Fatalf("after Down: Focus = %d, want Equal radio", model.CompareFilterDialog.Focus)
	}

	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if model.CompareFilterDialog.Filter != comparepkg.FilterEqual {
		t.Fatalf("after Space: dialog Filter = %v, want Equal", model.CompareFilterDialog.Filter)
	}
	if model.CompareView.Filter != comparepkg.FilterAll {
		t.Fatalf("after Space: view Filter = %v, want All until OK", model.CompareView.Filter)
	}

	for model.CompareFilterDialog.Focus != model.CompareFilterDialog.OKIndex() {
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

type selectRecordHost struct {
	compareHandlerHost
	selected map[int]map[string]bool
}

func (h *selectRecordHost) SetPanelSelection(id int, path string, selected bool) bool {
	if h.selected[id] == nil {
		h.selected[id] = map[string]bool{}
	}
	if h.selected[id][path] == selected {
		return false
	}
	if selected {
		h.selected[id][path] = true
	} else {
		delete(h.selected[id], path)
	}
	return true
}

// pickKindAndConfirm chooses the radio with Alt+mnemonic, then presses OK.
func pickKindAndConfirm(h *Handler, model *ui.Model, mnemonic rune) {
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, mnemonic, tcell.ModAlt))
	for model.CompareFilterDialog.Focus != model.CompareFilterDialog.OKIndex() {
		h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
}

func TestSelectDialogSelectsAndUnselectsMatchingFocusedSidePaths(t *testing.T) {
	root1, root2 := pathloc.MustParse("/amber"), pathloc.MustParse("/birch")
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{FocusColumn: ui.CompareColumnSecondary},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot: root1, SecondaryRoot: root2,
			Rows: []comparepkg.Row{
				{PrimaryRel: "harbor.txt", SecondaryRel: "harbor.txt", Kind: comparepkg.KindEqual, HashDone: true},
				{PrimaryRel: "lantern.txt", SecondaryRel: "meadow.txt", Kind: comparepkg.KindRelocated, HashDone: true},
				{PrimaryRel: "quartz.txt", SecondaryRel: "quartz.txt", Kind: comparepkg.KindContentDiff, HashDone: true},
			},
		},
	}
	host := &selectRecordHost{selected: map[int]map[string]bool{}}
	h := New(Deps{Host: host, Model: model})

	h.OpenSelectDialog(true)
	pickKindAndConfirm(h, model, 'a')
	if got := len(host.selected[ui.SecondaryPanel]); got != 3 || len(host.selected[ui.PrimaryPanel]) != 0 {
		t.Fatalf("after select all: %v, want 3 secondary paths only", host.selected)
	}
	if model.CompareFilterDialog.Open {
		t.Fatal("dialog should be closed after OK")
	}

	h.OpenSelectDialog(false)
	pickKindAndConfirm(h, model, 'm')
	got := host.selected[ui.SecondaryPanel]
	if len(got) != 2 || got["/birch/quartz.txt"] {
		t.Fatalf("after unselect modified: %v, want quartz.txt removed only", got)
	}
}
