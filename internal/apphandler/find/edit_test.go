package find

import (
	"testing"

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
