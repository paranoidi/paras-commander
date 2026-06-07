package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestPanelStashToggleIndependentPanels(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	for _, p := range []string{aPath, bPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	left := app.panelByID(ui.LeftPanel)
	right := app.panelByID(ui.RightPanel)
	left.AddSelection(aPath)
	app.togglePanelSelectionStash()
	if left.StashPathCount() != 1 || left.SelectedPathCount() != 0 {
		t.Fatalf("left after stash: stash=%d live=%d", left.StashPathCount(), left.SelectedPathCount())
	}

	app.model.ActivePanel = ui.RightPanel
	right.AddSelection(bPath)
	app.togglePanelSelectionStash()
	if right.StashPathCount() != 1 {
		t.Fatalf("right stash = %d, want 1", right.StashPathCount())
	}
	if left.StashPathCount() != 1 {
		t.Fatalf("left stash should remain 1, got %d", left.StashPathCount())
	}
}

func TestPanelStashRestoreWithoutLiveSelection(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "only.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	panel := app.activePanel()
	panel.AddSelection(p)
	app.togglePanelSelectionStash()
	app.togglePanelSelectionStash()
	if panel.SelectedPathCount() != 1 {
		t.Fatalf("restore: live=%d, want 1", panel.SelectedPathCount())
	}
	if !panel.StashEmpty() {
		t.Fatalf("stash should be cleared after restore, count=%d", panel.StashPathCount())
	}
}

func TestPanelStashRestoreDialogMerge(t *testing.T) {
	dir := t.TempDir()
	stashed := filepath.Join(dir, "stashed.txt")
	live := filepath.Join(dir, "live.txt")
	for _, p := range []string{stashed, live} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	panel := app.activePanel()
	panel.AddSelection(stashed)
	app.togglePanelSelectionStash()
	panel.AddSelection(live)
	app.togglePanelSelectionStash()
	if !app.model.StashRestoreDialog.Open {
		t.Fatal("expected stash restore dialog")
	}
	app.applyStashRestoreChoice("merge")
	if panel.StashPathCount() != 0 {
		t.Fatalf("stash count after merge = %d, want 0", panel.StashPathCount())
	}
	if panel.SelectedPathCount() != 2 {
		t.Fatalf("merged selection count = %d, want 2", panel.SelectedPathCount())
	}
}

func TestActionFromKeyMapsStashToggle(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt)
	got := lookupActionForView(ev, km, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelStashToggle {
		t.Fatalf("M-insert = %q, want %s", got, keymap.ActionPanelStashToggle)
	}
}
