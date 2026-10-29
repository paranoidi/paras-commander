package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// openAdvancedMove opens the move dialog for one source file with a second panel as destination
// and switches it to the Advanced screen.
func openAdvancedMove(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	srcDir, dstDir := filepath.Join(root, "source"), filepath.Join(root, "dest")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src := filepath.Join(srcDir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 100, 40)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	app.activePanel().SelectedPaths = map[string]bool{src: true}

	app.dialogCtrl.OpenMoveDialog()
	app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dialog.TransferAdvanced})
	if !app.model.TransferDialog.Advanced {
		t.Fatal("Advanced button did not open the rules screen")
	}
	return app
}

func TestTransferAdvancedQueuesPausedMoveWithRemoveEmpty(t *testing.T) {
	app := openAdvancedMove(t)
	for _, id := range []dlg.ID{dialog.TransferStartPaused, dialog.TransferRemoveEmpty} {
		app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: id})
	}
	app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dlg.OK})

	all := app.jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	j := all[0]
	if j.Type != jobs.TypeMove || !j.PausedAfterScan || !j.RemoveDanglingDirs || j.PromptDanglingDirs {
		t.Fatalf("job = type %v paused %v removeDangling %v prompt %v", j.Type, j.PausedAfterScan, j.RemoveDanglingDirs, j.PromptDanglingDirs)
	}
	if j.ConflictRules != nil {
		t.Fatalf("untouched rules must leave ConflictRules nil, got %+v", *j.ConflictRules)
	}
}

func TestTransferAdvancedRuleChangeSetsConflictRules(t *testing.T) {
	app := openAdvancedMove(t)
	// Row 1 is "Destination older", column 1 is Overwrite.
	app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dialog.ConflictRuleID(1, 1)})
	app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dlg.OK})

	all := app.jobState.AllJobs()
	if len(all) != 1 || all[0].ConflictRules == nil || all[0].ConflictRules.Time[1] != jobs.RuleOverwrite {
		t.Fatalf("jobs = %+v, want one with Time[1]=overwrite", all)
	}
}

func TestTransferAdvancedCancelReturnsToMainScreen(t *testing.T) {
	app := openAdvancedMove(t)
	app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dlg.Cancel})
	if d := app.model.TransferDialog; !d.Open || d.Advanced {
		t.Fatalf("dialog open=%v advanced=%v, want open on the main screen", d.Open, d.Advanced)
	}
}
