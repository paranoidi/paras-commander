package app

import (
	"os"
	"path/filepath"
	"testing"

	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestHistoryDialogMissingScanAppliesAndIgnoresStaleGen(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpaca")
	beta := filepath.Join(root, "bramble")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	navigatePanelIntoDir(t, app, screen, ui.PrimaryPanel, "alpaca")
	if err := app.navigatePanelToDirectory(ui.PrimaryPanel, root, ""); err != nil {
		t.Fatalf("navigatePanelToDirectory(root): %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load triggered by navigatePanelToDirectory
	navigatePanelIntoDir(t, app, screen, ui.PrimaryPanel, "bramble")

	app.openHistoryDialog(ui.PrimaryPanel)
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	if len(app.model.HistoryDialog.Paths) == 0 {
		t.Fatal("expected some panel history")
	}
	for _, missing := range app.model.HistoryDialog.PathMissing {
		if missing {
			t.Fatal("PathMissing true right after open, want false (scan is async)")
		}
	}

	staleGen := app.historyMissingGen

	// A fabricated payload at the pre-toggle generation must still apply if nothing else has
	// bumped the generation since.
	missingPath := app.model.HistoryDialog.Paths[0]
	app.applyHistoryDialogMissing(dialogctrl.PathsMissingPayload{
		Target:  "history",
		Gen:     staleGen,
		Missing: map[string]bool{missingPath: true},
	})
	if !app.model.HistoryDialog.PathMissing[0] {
		t.Fatal("expected matching-gen payload to apply")
	}

	// Now bump the generation (as toggleHistoryDialogBothPanels does) and confirm the old
	// generation's payload is dropped.
	app.toggleHistoryDialogBothPanels()
	if app.historyMissingGen == staleGen {
		t.Fatal("expected toggle to bump historyMissingGen")
	}
	for i, missing := range app.model.HistoryDialog.PathMissing {
		if missing {
			t.Fatalf("PathMissing[%d] true right after toggle, want false (reset, scan pending)", i)
		}
	}
	app.applyHistoryDialogMissing(dialogctrl.PathsMissingPayload{
		Target:  "history",
		Gen:     staleGen,
		Missing: map[string]bool{app.model.HistoryDialog.Paths[0]: true},
	})
	for i, missing := range app.model.HistoryDialog.PathMissing {
		if missing {
			t.Fatalf("PathMissing[%d] true, want stale-gen payload ignored after toggle", i)
		}
	}
}
