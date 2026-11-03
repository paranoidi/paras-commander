package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"

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
	// Known results survive the rebuild, so a missing row does not flash back to normal.
	for i, path := range app.model.HistoryDialog.Paths {
		if app.model.HistoryDialog.PathMissing[i] != (path == missingPath) {
			t.Fatalf("PathMissing[%d] = %v after toggle, want only %q missing", i, path != missingPath, missingPath)
		}
	}
	app.applyHistoryDialogMissing(dialogctrl.PathsMissingPayload{
		Target:  "history",
		Gen:     staleGen,
		Missing: map[string]bool{missingPath: false},
	})
	for i, path := range app.model.HistoryDialog.Paths {
		if path == missingPath && !app.model.HistoryDialog.PathMissing[i] {
			t.Fatal("stale-gen payload applied after toggle, want it ignored")
		}
	}
}

func TestHistoryDialogOpensAtOnceAndAppliesResultsOnce(t *testing.T) {
	app := newApp(t, newScreen(t, 80, 24), t.TempDir())
	gone, meadow := "/srv/lantern/orchard", "/opt/meadow/thistle"
	app.model.Primary.History = []string{gone}
	app.openHistoryDialog(ui.PrimaryPanel)
	st := &app.model.HistoryDialog
	if !st.Open || !st.Loading {
		t.Fatalf("open=%v loading=%v, want drawn at once while loading", st.Open, st.Loading)
	}
	zox, ok := app.historyView().Item(dialog.HistoryZoxide)
	if !ok || !zox.Disabled {
		t.Fatal("zoxide checkbox should reserve its row, disabled, while loading")
	}
	rows := app.historyDialogListRows()

	gen := app.historyOpenGen
	app.applyHistoryOpen(historyOpenPayload{
		Gen: gen, Zoxide: []string{meadow}, Installed: true,
		Missing: map[string]bool{gone: true, meadow: false},
	})
	if st.Loading || !slices.Equal(st.Paths, []string{gone, meadow}) || !slices.Equal(st.PathMissing, []bool{true, false}) {
		t.Fatalf("loading=%v paths=%q missing=%v", st.Loading, st.Paths, st.PathMissing)
	}
	if zox, _ := app.historyView().Item(dialog.HistoryZoxide); zox.Disabled || app.historyDialogListRows() != rows {
		t.Fatal("dialog changed shape when results arrived")
	}
	app.closeHistoryDialog()
	app.applyHistoryOpen(historyOpenPayload{Gen: gen, Installed: true})
	if app.model.HistoryDialog.Open {
		t.Fatal("stale payload reopened a closed dialog")
	}
}
