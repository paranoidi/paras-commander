package app

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestReconcileFindDialogSelectionSizeScansEarlyExitOnUnchangedGen(t *testing.T) {
	t.Parallel()
	app := &App{
		model: ui.Model{
			ViewMode: ui.ViewBrowser,
			FindDialog: ui.FindDialogState{
				Open:    true,
				PanelID: ui.PrimaryPanel,
				MarkedPaths: map[string]bool{
					filepath.Clean("/tmp/a"): true,
				},
				PathIsDir: map[string]bool{
					filepath.Clean("/tmp/a"): false,
				},
			},
		},
		diskUsage: diskusage.New(),
	}
	st := &app.model.FindDialog
	st.InvalidateMarkedSelectionDerived()
	app.findDialogSelectionScanGen = st.MarkedSelGen()
	beforeFP := app.findDialogSelectionScanFP

	app.reconcileFindDialogSelectionSizeScans()
	app.reconcileFindDialogSelectionSizeScans()

	if app.findDialogSelectionScanFP != beforeFP {
		t.Fatalf("scan fp changed on unchanged gen: %q -> %q", beforeFP, app.findDialogSelectionScanFP)
	}
}
