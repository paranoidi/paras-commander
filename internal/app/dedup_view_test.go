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
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	if !app.model.DedupProgressDialog.Open {
		t.Fatal("DedupProgressDialog should be open after starting scan")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser during scan", app.model.ViewMode)
	}

	snap := waitDedupDone(t, app)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup after scan", app.model.ViewMode)
	}
	if snap.Phase != comparepkg.DedupDone {
		t.Fatalf("phase = %v (%s)", snap.Phase, snap.Err)
	}
	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(snap.Groups))
	}
	if len(app.model.DedupList) != 2 {
		t.Fatalf("DedupList = %d, want 2 (sub dir collapsed + a.txt)", len(app.model.DedupList))
	}

	app.render() // exercise drawDedupView; must not panic

	// Esc closes back to the browser.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("after esc ViewMode = %v, want ViewBrowser", app.model.ViewMode)
	}
}

func TestDedupViewLeftCollapsesInsteadOfClosing(t *testing.T) {
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

	// Left is tree collapse now, not close: the view must stay open.
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if quit {
		t.Fatal("KeyLeft in dedup view must not quit the application")
	}
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup after KeyLeft", app.model.ViewMode)
	}
}

func TestDedupViewCopiesPaneTabFocusAndMark(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt"), filepath.Join("meadow", "beacon.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Dirs mode starts collapsed; Down from the first directory row selects the root file.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// Cursor on a duplicate file: copies pane lists the two OTHER copies.
	if got := len(app.model.DedupCopiesList); got == 0 {
		t.Fatal("copies pane empty for selected file")
	}
	var copyFiles []string
	selAbs := app.model.DedupList[app.model.DedupView.Main.Selected].Value.AbsKey
	for _, r := range app.model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowFile {
			copyFiles = append(copyFiles, r.Value.AbsKey)
			if r.Value.AbsKey == selAbs {
				t.Fatal("copies pane contains the selected file itself")
			}
		}
	}
	if len(copyFiles) != 2 {
		t.Fatalf("copies pane files = %v, want 2 other copies", copyFiles)
	}

	// Tab focuses the copies pane; Down moves its cursor, not the main one.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if !app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not focus the copies pane")
	}
	mainSel := app.model.DedupView.Main.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.DedupView.Main.Selected != mainSel {
		t.Fatal("Down in copies pane moved the main cursor")
	}

	// Space marks the focused copy.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 1 {
		t.Fatalf("MarkedCount = %d, want 1 after marking in copies pane", app.model.DedupView.MarkedCount)
	}

	// Tab returns to the main pane; moving the main cursor rebuilds the copies pane.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if app.model.DedupView.FocusCopies {
		t.Fatal("second Tab did not return focus to the main pane")
	}

	app.render() // twin panes must render without panicking
}

func TestDedupViewExpandAutoDescendsSingleDirChain(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "orchard", "deep", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("orchard", "deep", "deeper", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.dedupCtrl.CollapseAll()
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	rows := app.model.DedupList
	if len(rows) != 2 || rows[0].ID != "d:orchard" {
		t.Fatalf("collapsed rows = %+v, want [d:orchard, root lantern]", rows)
	}

	// Right on orchard: the single-subdir chain orchard→deep→deeper opens in one
	// step and the cursor lands on the deepest chain directory.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	rows = app.model.DedupList
	sel := app.model.DedupView.Main.Selected
	if sel < 0 || sel >= len(rows) || rows[sel].ID != "d:orchard/deep/deeper" {
		t.Fatalf("cursor on %q, want d:orchard/deep/deeper", rows[sel].ID)
	}
	if !rows[sel].Expanded {
		t.Fatal("deepest chain dir should be expanded (its file visible)")
	}
	if got := len(rows); got != 5 { // orchard, deep, deeper, deeper/lantern.txt, root lantern.txt
		t.Fatalf("rows after auto-descend = %d, want 5", got)
	}
}

func TestDedupViewTreeCollapseAndModes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("dirs-mode rows = %d, want 2 (meadow dir collapsed + root lantern.txt)", got)
	}

	// Ctrl+T switches to the groups tree: one collapsed group header.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if app.model.DedupView.TreeDirs {
		t.Fatal("Ctrl+T did not switch to groups tree mode")
	}
	if got := len(app.model.DedupList); got != 1 {
		t.Fatalf("groups-mode rows = %d, want 1 (collapsed header)", got)
	}

	// Right expands the group header; the copy row appears.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("after expand rows = %d, want 2", got)
	}
	// Left collapses again.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if got := len(app.model.DedupList); got != 1 {
		t.Fatalf("after collapse rows = %d, want 1", got)
	}
	// Marking the collapsed group still marks the hidden copy.
	app.dedupCtrl.ToggleGroupMark()
	if got := len(app.dedupCtrl.MarkedPaths()); got != 2 {
		t.Fatalf("marked after group-mark on collapsed header = %d, want 2", got)
	}
	// Right again re-expands.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("after re-expand rows = %d, want 2", got)
	}

	// Ctrl+T switches back to the directory tree.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if !app.model.DedupView.TreeDirs {
		t.Fatal("Ctrl+T did not switch back to directory tree mode")
	}
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("dirs-mode rows = %d, want 2 (collapse state preserved)", got)
	}
	if got := len(app.dedupCtrl.MarkedPaths()); got != 2 {
		t.Fatalf("marks lost on mode switch: %d, want 2", got)
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
	// Confirming (Yes) opens the empty-dirs cleanup confirmation.
	app.executeDelete()
	if app.deleteDialogOpen() {
		t.Fatal("delete dialog not closed after confirm")
	}
	if !app.model.DedupEmptyDirsConfirm.Open {
		t.Fatal("empty-dirs confirm dialog did not open after delete confirm")
	}
	if app.model.DedupEmptyDirsConfirm.Focus != 0 {
		t.Fatalf("empty-dirs confirm default focus = %d, want 0 (Yes)", app.model.DedupEmptyDirsConfirm.Focus)
	}
	// Enter accepts the Yes default, enqueuing the delete and optimistically pruning the group.
	app.handleDedupEmptyDirsConfirmKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.DedupEmptyDirsConfirm.Open {
		t.Fatal("empty-dirs confirm dialog not closed after Enter")
	}
	if len(app.model.DedupSnapshot.Groups) != 0 {
		t.Fatalf("groups after delete = %d, want 0 (group drops below 2)", len(app.model.DedupSnapshot.Groups))
	}
}

// TestDedupViewEmptyDirsConfirmDefaultsYesAndRemoves verifies that answering
// the empty-dirs confirmation with the Yes default removes the now-empty
// directory left behind by the delete, while answering No leaves it.
func TestDedupViewEmptyDirsConfirmDefaultsYesAndRemoves(t *testing.T) {
	for _, removeEmpty := range []bool{true, false} {
		t.Run(map[bool]string{true: "yes", false: "no"}[removeEmpty], func(t *testing.T) {
			// sub is the scan root itself, containing only the duplicate pair, so
			// deleting both leaves it empty.
			parent := t.TempDir()
			sub := filepath.Join(parent, "sub")
			if err := os.Mkdir(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("dup"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("dup"), 0o644); err != nil {
				t.Fatal(err)
			}

			screen := newScreen(t, 80, 24)
			app := newApp(t, screen, sub)
			app.openFindDuplicates()
			waitDedupDone(t, app)

			// Mark both duplicate rows (Insert marks + advances cursor, per panel.select-toggle).
			app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
			app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
			if got := len(app.dedupCtrl.MarkedPaths()); got != 2 {
				t.Fatalf("marked paths = %d, want 2", got)
			}

			app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
			if !app.deleteDialogOpen() {
				t.Fatal("delete key did not open the delete dialog")
			}
			app.executeDelete()
			if !app.model.DedupEmptyDirsConfirm.Open {
				t.Fatal("empty-dirs confirm dialog did not open")
			}
			if app.model.DedupEmptyDirsConfirm.Focus != 0 {
				t.Fatalf("empty-dirs confirm default focus = %d, want 0 (Yes)", app.model.DedupEmptyDirsConfirm.Focus)
			}
			if removeEmpty {
				app.handleDedupEmptyDirsConfirmKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			} else {
				app.handleDedupEmptyDirsConfirmKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
			}

			flushBackgroundJobs(t, app)
			waitUntilAppJobsFinished(t, app, 5*time.Second)

			_, err := os.Stat(sub)
			if removeEmpty {
				if !os.IsNotExist(err) {
					t.Fatalf("sub dir stat err = %v, want IsNotExist (dir should be removed)", err)
				}
			} else if err != nil {
				t.Fatalf("sub dir stat err = %v, want nil (dir should remain)", err)
			}
		})
	}
}

func TestDedupViewMenuOmitsBrowserItems(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.openFindDuplicates()
	waitDedupDone(t, app)

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
	waitDedupDone(t, app)
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

func TestDedupProgressDialogFooterShowsEscAndF10(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.DedupProgressDialog.Open = true
	app.model.DedupSnapshot.Phase = comparepkg.DedupHashing

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

func TestDedupProgressDialogMenuBarNotInteractive(t *testing.T) {
	m := ui.Model{
		DedupProgressDialog: dialog.DedupProgressDialogState{Open: true},
		DedupSnapshot:       comparepkg.DedupSnapshot{Phase: comparepkg.DedupHashing},
	}
	if m.MenuBarInteractive() {
		t.Fatal("dedup progress dialog must hide interactive menu bar")
	}
	if !m.ModalDialogOpen() {
		t.Fatal("dedup progress dialog must count as modal")
	}
}

func TestDedupProgressDialogCancelClosesScan(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	if !app.model.DedupProgressDialog.Open {
		t.Fatal("expected progress dialog open")
	}

	app.handleDedupProgressDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.DedupProgressDialog.Open {
		t.Fatal("progress dialog should close after cancel")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after cancel", app.model.ViewMode)
	}
}
