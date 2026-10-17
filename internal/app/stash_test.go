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

	left := app.panelByID(ui.PrimaryPanel)
	right := app.panelByID(ui.SecondaryPanel)
	left.AddSelection(aPath)
	app.togglePanelSelectionStash()
	if left.StashPathCount() != 1 || left.SelectedPathCount() != 0 {
		t.Fatalf("left after stash: stash=%d live=%d", left.StashPathCount(), left.SelectedPathCount())
	}

	app.model.ActivePanel = ui.SecondaryPanel
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
	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModCtrl|tcell.ModAlt)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelStashToggle {
		t.Fatalf("C-M-insert = %q, want %s", got, keymap.ActionPanelStashToggle)
	}
}

func TestActionFromKeyMapsPinToggle(t *testing.T) {
	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt)
	got := lookupActionForView(ev, km, nil, nil, nil, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionPanelPinToggle {
		t.Fatalf("M-insert = %q, want %s", got, keymap.ActionPanelPinToggle)
	}
}

func TestStripCursorSurvivesStash(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	var sel []string
	for _, n := range []string{"lantern.txt", "quartz.txt", "willow.txt", "ember.txt"} {
		p := filepath.Join(other, n)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		sel = append(sel, p)
	}
	app := newApp(t, newScreen(t, 80, 24), dir)
	p := app.activePanel()
	for _, s := range sel {
		p.AddSelection(s)
	}
	if !app.focusSelectedStrip() {
		t.Fatal("focus failed")
	}
	p.MoveSelectedStrip(2, 10)
	want, _ := p.SelectedPathAtStripIndex(p.SelectedStripCursor)
	app.reconcileAfterEvent()
	app.togglePanelSelectionStash()
	app.reconcileAfterEvent()
	app.togglePanelSelectionStash()
	if !app.focusSelectedStrip() {
		t.Fatal("refocus failed")
	}
	if got, _ := p.SelectedPathAtStripIndex(p.SelectedStripCursor); got != want {
		t.Fatalf("cursor path %q, want %q", got, want)
	}
}

func TestStripCursorBelongsToSelectionSet(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	mk := func(names ...string) []string {
		var out []string
		for _, n := range names {
			p := filepath.Join(other, n)
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			out = append(out, p)
		}
		return out
	}
	old := mk("lantern.txt", "quartz.txt", "willow.txt", "ember.txt")
	fresh := mk("meadow.txt", "falcon.txt", "copper.txt")
	app := newApp(t, newScreen(t, 80, 24), dir)
	p := app.activePanel()
	for _, s := range old {
		p.AddSelection(s)
	}
	if !app.focusSelectedStrip() {
		t.Fatal("focus failed")
	}
	p.MoveSelectedStrip(2, 10)
	want, _ := p.SelectedPathAtStripIndex(p.SelectedStripCursor)
	app.reconcileAfterEvent()
	app.togglePanelSelectionStash()
	app.reconcileAfterEvent()

	for _, s := range fresh {
		p.AddSelection(s)
	}
	if !app.focusSelectedStrip() {
		t.Fatal("focus fresh failed")
	}
	if p.SelectedStripCursor != 0 {
		t.Fatalf("fresh selection cursor %d, want 0", p.SelectedStripCursor)
	}
	app.reconcileAfterEvent()
	p.ClearSelection()
	app.reconcileAfterEvent()
	app.togglePanelSelectionStash()
	if !app.focusSelectedStrip() {
		t.Fatal("refocus failed")
	}
	if got, _ := p.SelectedPathAtStripIndex(p.SelectedStripCursor); got != want {
		t.Fatalf("cursor path %q, want %q", got, want)
	}
}
