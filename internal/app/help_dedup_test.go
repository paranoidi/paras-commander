package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func setupDedupViewApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup", app.model.ViewMode)
	}
	return app
}

func helpEntryIndex(entries []dialog.HelpEntry, actionID string) int {
	for i, e := range entries {
		if e.ActionID == actionID {
			return i
		}
	}
	return -1
}

func selectHelpEntry(app *App, actionID string) bool {
	idx := helpEntryIndex(app.model.HelpView.Entries, actionID)
	if idx < 0 {
		return false
	}
	for i, entIdx := range app.model.HelpView.Ranked {
		if entIdx == idx {
			app.model.HelpView.Selected = i
			app.model.HelpView.Focus = 0
			return true
		}
	}
	return false
}

func TestOpenHelpDialogInDedupView(t *testing.T) {
	app := setupDedupViewApp(t)
	app.openHelpDialog()
	if !app.model.HelpView.Open {
		t.Fatal("HelpView should open in find-duplicates view")
	}
	if len(app.model.HelpView.Entries) == 0 {
		t.Fatal("dedup help entries should not be empty")
	}
}

func TestBuildDedupHelpEntriesIncludesAllDedupActions(t *testing.T) {
	app := setupDedupViewApp(t)
	entries := app.buildDedupHelpEntries()
	byID := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		byID[e.ActionID] = struct{}{}
	}
	for _, spec := range keymap.DefaultActionSpecs() {
		if spec.Views&keymap.HelpDedup == 0 || !strings.HasPrefix(spec.ID, "dedup.") {
			continue
		}
		if _, ok := byID[spec.ID]; !ok {
			t.Fatalf("dedup help missing %q", spec.ID)
		}
	}
	for _, forbidden := range []string{keymap.ActionCopy, keymap.ActionMove, keymap.ActionPanelFindDuplicates} {
		if _, ok := byID[forbidden]; ok {
			t.Fatalf("dedup help should not include browser-only action %q", forbidden)
		}
	}
}

func TestDedupHelpUsesOverlayKeys(t *testing.T) {
	app := setupDedupViewApp(t)
	entries := app.buildDedupHelpEntries()
	idx := helpEntryIndex(entries, keymap.ActionDedupMarkKeep)
	if idx < 0 {
		t.Fatal("dedup help missing dedup.mark-keep")
	}
	if !strings.Contains(entries[idx].Keys, "Ctrl+K") {
		t.Fatalf("dedup.mark-keep keys = %q, want Ctrl+K from overlay", entries[idx].Keys)
	}
}

func TestHelpEnterRunsDedupAction(t *testing.T) {
	app := setupDedupViewApp(t)
	before := app.model.DedupView.IgnoreEmpty
	app.openHelpDialog()
	if !selectHelpEntry(app, keymap.ActionDedupToggleEmpty) {
		t.Fatal("dedup help missing dedup.toggle-empty")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter on dedup.toggle-empty should not quit")
	}
	if app.model.HelpView.Open {
		t.Fatal("HelpView should close after activating action")
	}
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup after help action", app.model.ViewMode)
	}
	if app.model.DedupView.IgnoreEmpty == before {
		t.Fatal("dedup.toggle-empty from help should toggle IgnoreEmpty")
	}
}

func TestHelpEnterNavUpInDedupView(t *testing.T) {
	app := setupDedupViewApp(t)
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	before := app.model.DedupView.Main.Selected
	if before == 0 {
		t.Fatal("need non-zero main selection for nav.up test")
	}
	app.openHelpDialog()
	if !selectHelpEntry(app, keymap.ActionNavUp) {
		t.Fatal("dedup help missing nav.up")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter on nav.up should not quit")
	}
	if got := app.model.DedupView.Main.Selected; got != before-1 {
		t.Fatalf("Main.Selected = %d, want %d after nav.up from help", got, before-1)
	}
}
