package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

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
	if app.selectionSizeScanFP[ui.PrimaryPanel] == "" {
		t.Fatal("fingerprint empty after first reconcile with pending directory")
	}
	firstFP := app.selectionSizeScanFP[ui.PrimaryPanel]
	app.reconcileSelectionSizeScans(ui.PrimaryPanel)
	if app.selectionSizeScanFP[ui.PrimaryPanel] != firstFP {
		t.Fatal("fingerprint changed without selection change")
	}
}
