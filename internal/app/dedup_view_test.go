package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func waitDedupDone(t *testing.T, app *App) comparepkg.DedupSnapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDedupUpdates(dedupctrl.WakePayload{})
		snap := app.model.DedupSnapshot
		if snap.Phase == comparepkg.DedupDone || snap.Phase == comparepkg.DedupError {
			return snap
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("dedup scan did not finish")
	return comparepkg.DedupSnapshot{}
}

func TestDedupViewFindsDuplicates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unique.txt"), []byte("only-me"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup", app.model.ViewMode)
	}

	snap := waitDedupDone(t, app)
	if snap.Phase != comparepkg.DedupDone {
		t.Fatalf("phase = %v (%s)", snap.Phase, snap.Err)
	}
	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(snap.Groups))
	}
	if len(app.model.DedupList) != 2 {
		t.Fatalf("DedupList = %d, want 2", len(app.model.DedupList))
	}

	app.render() // exercise drawDedupView; must not panic

	// Esc closes back to the browser.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("after esc ViewMode = %v, want ViewBrowser", app.model.ViewMode)
	}
}

func TestDedupViewLeftClosesViewDoesNotQuit(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup", app.model.ViewMode)
	}

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if quit {
		t.Fatal("KeyLeft in dedup view must not quit the application")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after KeyLeft", app.model.ViewMode)
	}
}

func TestDedupViewMarkAndDelete(t *testing.T) {
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

	// Insert marks the selected file for deletion.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if got := len(app.dedupCtrl.MarkedPaths()); got != 1 {
		t.Fatalf("marked = %d, want 1", got)
	}
	if app.model.DedupView.MarkedCount != 1 {
		t.Fatalf("MarkedCount = %d, want 1", app.model.DedupView.MarkedCount)
	}
	if app.model.DedupView.MarkedReclaimBytes != int64(len("dup")) {
		t.Fatalf("MarkedReclaimBytes = %d, want %d", app.model.DedupView.MarkedReclaimBytes, len("dup"))
	}
	// F8 opens the standard delete confirmation dialog listing the marked files.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !app.deleteDialogOpen() {
		t.Fatal("delete key did not open the delete dialog")
	}
	if got := len(app.model.FileDialog.DeleteEntries); got != 1 {
		t.Fatalf("delete entries = %d, want 1", got)
	}
	// Confirming (Yes) enqueues the delete and optimistically prunes the group.
	app.executeDelete()
	if app.deleteDialogOpen() {
		t.Fatal("delete dialog not closed after confirm")
	}
	if len(app.model.DedupSnapshot.Groups) != 0 {
		t.Fatalf("groups after delete = %d, want 0 (group drops below 2)", len(app.model.DedupSnapshot.Groups))
	}
}

func TestDedupViewMenuOmitsBrowserItems(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.openFindDuplicates()

	for _, def := range menu.ActiveDefinitions(app.model.MenuDefinitions) {
		switch def.ID {
		case menu.TopPanelLeft, menu.TopPanelRight, menu.TopCommand, menu.TopOptions:
			t.Fatalf("dedup view menu bar includes unavailable top menu %q", def.Label)
		}
		for _, item := range def.Items {
			switch item.Action {
			case "file.edit", "file.view", "copy", "move", "panel.sort-dialog":
				t.Fatalf("dedup view menu includes unavailable item %q", item.Label)
			}
		}
	}
}

func TestDedupViewFooterEscFirst(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.openFindDuplicates()
	keys := app.activeFooterKeys()
	if len(keys) == 0 {
		t.Fatal("footer keys empty")
	}
	if keys[0] != menu.FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	var foundDelete, foundRefresh bool
	for _, fk := range keys {
		if fk.Hint == "Delete" {
			foundDelete = true
		}
		if fk.Hint == "Refresh" {
			foundRefresh = true
		}
		if fk.Hint == "Edit" || fk.Hint == "View" || fk.Hint == "Copy" {
			t.Fatalf("footer lists unavailable browser action %q: %+v", fk.Hint, keys)
		}
	}
	if !foundDelete {
		t.Fatalf("footer missing Delete: %+v", keys)
	}
	if !foundRefresh {
		t.Fatalf("footer missing Refresh: %+v", keys)
	}
}

func TestDedupAwaitHashConfirmFooterShowsOnlyEscAndF10(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewDedup
	app.model.DedupSnapshot.Phase = comparepkg.DedupAwaitConfirm
	app.model.DedupSnapshot.HashTotal = 168857

	keys := app.activeFooterKeys()
	if len(keys) != 2 {
		t.Fatalf("footer keys = %+v, want Esc and F10 only", keys)
	}
	if keys[0] != menu.FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	if keys[1].Key != tcell.KeyF10 || keys[1].Hint != "Quit" {
		t.Fatalf("footer[1] = %+v, want F10 Quit", keys[1])
	}
}

func TestDedupAwaitHashConfirmMenuBarNotInteractive(t *testing.T) {
	m := ui.Model{
		ViewMode:      ui.ViewDedup,
		DedupSnapshot: comparepkg.DedupSnapshot{Phase: comparepkg.DedupAwaitConfirm},
	}
	if m.MenuBarInteractive() {
		t.Fatal("dedup hash-confirm gate must hide interactive menu bar")
	}
}
