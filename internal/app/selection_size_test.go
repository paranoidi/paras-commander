package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// waitForSelectionSizeScanFP drains the debounced background reconcileSelectionSizeScans
// result (posted as a selectionScanNeedPayload interrupt event) so the fingerprint field is
// populated before the test inspects it.
func waitForSelectionSizeScanFP(t *testing.T, app *App, panelID int) {
	t.Helper()
	screen, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("app.screen is not a tcell.SimulationScreen")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for screen.HasPendingEvent() {
			if ev, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				app.handleInterruptPayload(ev.Data())
			}
		}
		if app.selectionSizeScanFP[panelID] != "" {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestReconcileSelectionSizeScansIdempotentFingerprint(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leaf.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testAppMinimal(t)
	app.model.ViewMode = ui.ViewBrowser
	app.disk.engine = diskusage.New()
	app.model.Primary = panel.State{
		Path:    pathloc.MustParse(dir),
		Entries: []localfs.Entry{{Name: "subdir", Path: sub, Type: localfs.EntryDirectory}},
		SelectedPaths: map[string]bool{
			sub: true,
		},
		SelectedDirPaths: map[string]bool{
			sub: true,
		},
	}
	// selectionHasDirs is unexported; SelectedDirPaths drives SelectionHasDirs().

	app.reconcileSelectionSizeScans(ui.PrimaryPanel)
	waitForSelectionSizeScanFP(t, app, ui.PrimaryPanel)
	if app.selectionSizeScanFP[ui.PrimaryPanel] == "" {
		t.Fatal("fingerprint empty after first reconcile with pending directory")
	}
	firstFP := app.selectionSizeScanFP[ui.PrimaryPanel]
	app.reconcileSelectionSizeScans(ui.PrimaryPanel)
	waitForSelectionSizeScanFP(t, app, ui.PrimaryPanel)
	if app.selectionSizeScanFP[ui.PrimaryPanel] != firstFP {
		t.Fatal("fingerprint changed without selection change")
	}
}
