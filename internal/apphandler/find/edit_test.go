package find

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func editTestModel(selected int) *ui.Model {
	return &ui.Model{FindDialog: dialog.FindDialogState{
		Open:     true,
		RootPath: "/meadow",
		Entries: []dialog.FindEntry{
			{RelLine: "harbor/lantern.txt"},
			{RelLine: "harbor/orchard", IsDir: true},
		},
		Ranked:   []int{0, 1},
		Selected: selected,
	}}
}

func TestFindEditOpensHighlightedFileAndKeepsDialog(t *testing.T) {
	t.Parallel()
	host := &fakeFindHost{}
	model := editTestModel(0)
	h := newTestFindHandler(host, model)
	if !h.RunAction(keymap.ActionFindEdit) {
		t.Fatal("find.edit not handled")
	}
	if host.editedPath != "/meadow/harbor/lantern.txt" {
		t.Fatalf("edited %q, want /meadow/harbor/lantern.txt", host.editedPath)
	}
	if !model.FindDialog.Open {
		t.Fatal("find.edit closed the dialog")
	}
}

func TestFindEditRefusesDirectory(t *testing.T) {
	t.Parallel()
	host := &fakeFindHost{}
	h := newTestFindHandler(host, editTestModel(1))
	h.RunAction(keymap.ActionFindEdit)
	if host.editedPath != "" {
		t.Fatalf("edited directory %q", host.editedPath)
	}
	if host.transientMsg != "Edit: not a file" {
		t.Fatalf("message %q, want Edit: not a file", host.transientMsg)
	}
}

func TestFindEditSwallowedInJumpMode(t *testing.T) {
	t.Parallel()
	host := &fakeFindHost{}
	model := editTestModel(0)
	model.FindDialog.JumpMode = true
	h := newTestFindHandler(host, model)
	if !h.RunAction(keymap.ActionFindEdit) {
		t.Fatal("find.edit not swallowed in jump mode")
	}
	if host.editedPath != "" {
		t.Fatalf("jump mode edited %q", host.editedPath)
	}
}

// A rank result that lands while Find is hidden behind the viewer it opened is applied, and the
// reopened dialog shows it; a closed Find drops it.
func TestFindRankAppliesWhileHiddenForView(t *testing.T) {
	t.Parallel()
	host := &fakeFindHost{}
	model := editTestModel(0)
	h := newTestFindHandler(host, model)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	h.screen = screen
	h.RunAction(keymap.ActionFindView)
	if model.FindDialog.Open || !h.HiddenForView() {
		t.Fatal("find.view did not hide Find")
	}
	h.pendingRank = &rankResult{gen: h.rankGen, ranked: []int{0}}
	if !h.ApplyPendingRank() {
		t.Fatal("rank dropped while hidden for view")
	}
	h.ReopenAfterPreviewClose()
	if !model.FindDialog.Open || len(model.FindDialog.Ranked) != 1 {
		t.Fatalf("reopened open=%v ranked=%v", model.FindDialog.Open, model.FindDialog.Ranked)
	}

	h.CloseDialog()
	h.pendingRank = &rankResult{gen: h.rankGen, ranked: []int{0}}
	if h.ApplyPendingRank() {
		t.Fatal("rank applied to a closed Find")
	}
}
