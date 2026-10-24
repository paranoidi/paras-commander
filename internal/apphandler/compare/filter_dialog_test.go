package compare

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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
		t.Fatalf("after Down: Focus = %q, want Equal radio", model.CompareFilterDialog.Focus)
	}

	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	wantPending := comparepkg.FilterAll &^ comparepkg.FilterEqual
	if model.CompareFilterDialog.Filter != wantPending {
		t.Fatalf("after Space: dialog Filter = %v, want All minus Equal", model.CompareFilterDialog.Filter)
	}
	if model.CompareView.Filter != comparepkg.FilterAll {
		t.Fatalf("after Space: view Filter = %v, want All until OK", model.CompareView.Filter)
	}

	for model.CompareFilterDialog.Focus != dlg.OK {
		h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.CompareView.Filter != wantPending {
		t.Fatalf("after OK: view Filter = %v, want All minus Equal", model.CompareView.Filter)
	}
	if model.CompareFilterDialog.Open {
		t.Fatal("dialog should be closed after OK")
	}
}

func TestFilterDialogTogglesMasterAndKinds(t *testing.T) {
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	h.OpenFilterDialog()
	alt := func(r rune) { h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModAlt)) }

	alt('a')
	if got := model.CompareFilterDialog.Filter; got != comparepkg.FilterNone {
		t.Fatalf("master on All: Filter = %v, want None", got)
	}
	alt('l')
	alt('d')
	if got, want := model.CompareFilterDialog.Filter, comparepkg.FilterPrimaryOnly|comparepkg.FilterContentDiff; got != want {
		t.Fatalf("after L, D: Filter = %v, want %v", got, want)
	}
	alt('l')
	if got := model.CompareFilterDialog.Filter; got != comparepkg.FilterContentDiff {
		t.Fatalf("after L again: Filter = %v, want Content diff", got)
	}
	alt('a')
	if got := model.CompareFilterDialog.Filter; got != comparepkg.FilterAll {
		t.Fatalf("master on partial: Filter = %v, want All", got)
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
	h.FilterDialogCancel()
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
	for model.CompareFilterDialog.Focus != dlg.OK {
		h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	h.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
}

func TestSelectDialogSelectsAndUnselectsMatchingFocusedSidePaths(t *testing.T) {
	root1, root2 := pathloc.MustParse("/amber"), pathloc.MustParse("/birch")
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll, FocusColumn: ui.CompareColumnSecondary},
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

func TestSelectDialogOnlyOtherSideSelectsOtherPanel(t *testing.T) {
	root1, root2 := pathloc.MustParse("/amber"), pathloc.MustParse("/birch")
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot: root1, SecondaryRoot: root2,
			Rows: []comparepkg.Row{
				{PrimaryRel: "harbor.txt", Kind: comparepkg.KindPrimaryOnly, HashDone: true},
				{SecondaryRel: "lantern.txt", Kind: comparepkg.KindSecondaryOnly, HashDone: true},
			},
		},
	}
	host := &selectRecordHost{selected: map[int]map[string]bool{}}
	h := New(Deps{Host: host, Model: model})

	h.OpenSelectDialog(true)
	pickKindAndConfirm(h, model, 'n')
	if got := host.selected[ui.SecondaryPanel]; len(got) != 1 || !got["/birch/lantern.txt"] || len(host.selected[ui.PrimaryPanel]) != 0 {
		t.Fatalf("only other side: %v, want lantern.txt in secondary only", host.selected)
	}
}

type toggleRecordHost struct {
	compareHandlerHost
	toggled map[int][]string
}

func (h *toggleRecordHost) TogglePanelSelection(id int, path string) bool {
	h.toggled[id] = append(h.toggled[id], path)
	return false
}

func TestInvertColumnSelectionTogglesFocusedSidePaths(t *testing.T) {
	model := &ui.Model{
		ViewMode:    ui.ViewCompare,
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll, FocusColumn: ui.CompareColumnSecondary},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot: pathloc.MustParse("/amber"), SecondaryRoot: pathloc.MustParse("/birch"),
			Rows: []comparepkg.Row{
				{PrimaryRel: "harbor.txt", SecondaryRel: "harbor.txt", Kind: comparepkg.KindEqual, HashDone: true},
				{PrimaryRel: "quartz.txt", Kind: comparepkg.KindPrimaryOnly, HashDone: true},
				{SecondaryRel: "lantern.txt", Kind: comparepkg.KindSecondaryOnly, HashDone: true},
			},
		},
	}
	host := &toggleRecordHost{toggled: map[int][]string{}}
	New(Deps{Host: host, Model: model}).InvertColumnSelection()
	got := host.toggled[ui.SecondaryPanel]
	if len(got) != 2 || len(host.toggled[ui.PrimaryPanel]) != 0 || got[0] != "/birch/harbor.txt" || got[1] != "/birch/lantern.txt" {
		t.Fatalf("toggled = %v, want two secondary paths", host.toggled)
	}
}
