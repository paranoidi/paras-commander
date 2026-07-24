package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// setupMultiDirSelection creates root/alpha and root/bravo, each with a couple of files,
// and opens the app at alpha (away from the common root, root).
func setupMultiDirSelection(t *testing.T) (app *App, root, alpha, bravo string) {
	t.Helper()
	root = t.TempDir()
	alpha = filepath.Join(root, "alpha")
	bravo = filepath.Join(root, "bravo")
	for _, d := range []string{alpha, bravo} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeFile(t, filepath.Join(alpha, "river.txt"))
	writeFile(t, filepath.Join(alpha, "pond.txt"))
	writeFile(t, filepath.Join(bravo, "stone.txt"))
	screen := newScreen(t, 80, 24)
	app = newApp(t, screen, alpha)
	return app, root, alpha, bravo
}

func TestTransferMultiDirSelectionOpensTransferDialogWithPreview(t *testing.T) {
	app, root, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should open directly for multi-directory selection away from common root")
	}
	d := app.model.TransferDialog
	if d.CommonRoot != root {
		t.Fatalf("common root = %q, want %q", d.CommonRoot, root)
	}
	if d.Kind != dialog.TransferKindCopy {
		t.Fatalf("kind = %v, want Copy", d.Kind)
	}
	if !d.MultiLocation() {
		t.Fatal("MultiLocation() should be true")
	}
	wantLabels := map[string]bool{"alpha/river.txt": true, "bravo/stone.txt": true}
	if len(d.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(d.Entries))
	}
	for _, e := range d.Entries {
		if !wantLabels[e.Name] {
			t.Errorf("unexpected entry label %q", e.Name)
		}
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want unchanged %q", got, alpha)
	}
}

// TestTransferMultiDirSelectionOpensAtCommonRoot verifies the multi-location Source/Result
// preview triggers even when the active panel is already at the deepest common ancestor of
// the selection (previously suppressed in that case).
func TestTransferMultiDirSelectionOpensAtCommonRoot(t *testing.T) {
	app, root, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))
	if err := p.Load(root); err != nil {
		t.Fatalf("load root: %v", err)
	}

	app.dialogCtrl.ActivateCopyAction()
	if !app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should open")
	}
	d := app.model.TransferDialog
	if d.CommonRoot != root {
		t.Fatalf("common root = %q, want %q", d.CommonRoot, root)
	}
	if !d.MultiLocation() {
		t.Fatal("MultiLocation() should be true even when the panel is already at the common root")
	}
}

func TestTransferMultiDirSelectionMoveKind(t *testing.T) {
	app, root, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	app.dialogCtrl.ActivateMoveAction()
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindMove {
		t.Fatal("transfer dialog should open with Move kind")
	}
	if app.model.TransferDialog.CommonRoot != root {
		t.Fatalf("common root = %q, want %q", app.model.TransferDialog.CommonRoot, root)
	}
}

func TestTransferSingleDirSelectionSkipsMultiLocation(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(alpha, "pond.txt"))
	if err := p.Load(bravo); err != nil {
		t.Fatalf("load bravo: %v", err)
	}

	app.dialogCtrl.ActivateCopyAction()
	if !app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should open for single-directory selection")
	}
	if app.model.TransferDialog.CommonRoot != "" {
		t.Fatalf("common root = %q, want empty for single-directory selection", app.model.TransferDialog.CommonRoot)
	}
	if app.model.TransferDialog.MultiLocation() {
		t.Fatal("MultiLocation() should be false for single-directory selection")
	}
}

func TestTransferFlattenToggleAltI(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))
	app.dialogCtrl.ActivateCopyAction()
	if !app.model.TransferDialog.MultiLocation() {
		t.Fatal("expected multi-location copy dialog")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt))
	if !app.model.TransferDialog.FlattenIntoDest {
		t.Fatal("Alt+I should toggle FlattenIntoDest on")
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'I', tcell.ModAlt))
	if app.model.TransferDialog.FlattenIntoDest {
		t.Fatal("Alt+I should toggle FlattenIntoDest off")
	}
}

func TestTransferFlattenToggleAltIWorksForMove(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))
	app.dialogCtrl.ActivateMoveAction()
	if !app.model.TransferDialog.MultiLocation() {
		t.Fatal("expected multi-location move dialog")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt))
	if !app.model.TransferDialog.FlattenIntoDest {
		t.Fatal("Alt+I should toggle FlattenIntoDest on for move too")
	}
}

func TestTransferFlattenToggleFocusedSpace(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))
	app.dialogCtrl.ActivateCopyAction()

	flattenIdx := dialog.TransferDialogEffectiveNumContent(app.model.TransferDialog) - 1
	if flattenIdx != 3 {
		t.Fatalf("copy multi-location flatten focus index = %d, want 3", flattenIdx)
	}
	app.model.TransferDialog.FocusField = flattenIdx
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !app.model.TransferDialog.FlattenIntoDest {
		t.Fatal("space on the focused flatten row should toggle FlattenIntoDest")
	}

	app.dialogCtrl.CloseTransferDialog()
	app.dialogCtrl.ActivateMoveAction()
	flattenIdx = dialog.TransferDialogEffectiveNumContent(app.model.TransferDialog) - 1
	if flattenIdx != 1 {
		t.Fatalf("move multi-location flatten focus index = %d, want 1", flattenIdx)
	}
	app.model.TransferDialog.FocusField = flattenIdx
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !app.model.TransferDialog.FlattenIntoDest {
		t.Fatal("space on the focused flatten row should toggle FlattenIntoDest for move")
	}
}

func TestTransferPreviewListPgUpPgDnClampsScroll(t *testing.T) {
	root := t.TempDir()
	cedar := filepath.Join(root, "cedar")
	willow := filepath.Join(root, "willow")
	for _, d := range []string{cedar, willow} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	var selected []string
	for i := 0; i < 6; i++ {
		p := filepath.Join(cedar, fmt.Sprintf("leaf%d.txt", i))
		writeFile(t, p)
		selected = append(selected, p)
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(willow, fmt.Sprintf("branch%d.txt", i))
		writeFile(t, p)
		selected = append(selected, p)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, cedar)
	panel := app.activePanel()
	for _, s := range selected {
		panel.AddSelection(s)
	}
	app.dialogCtrl.ActivateCopyAction()
	d := &app.model.TransferDialog
	if !d.MultiLocation() {
		t.Fatal("expected multi-location dialog")
	}
	if len(d.Entries) != 12 {
		t.Fatalf("entries = %d, want 12", len(d.Entries))
	}

	vp := app.dialogCtrl.TransferPreviewListViewportRows()
	if vp < 1 {
		t.Fatalf("viewport rows = %d, want >= 1", vp)
	}
	maxScroll := len(d.Entries) - vp
	if maxScroll < 0 {
		maxScroll = 0
	}

	for range 20 {
		app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	}
	if d.EntriesScroll != maxScroll {
		t.Fatalf("scroll after repeated PgDn = %d, want clamped max %d", d.EntriesScroll, maxScroll)
	}

	for range 20 {
		app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	}
	if d.EntriesScroll != 0 {
		t.Fatalf("scroll after repeated PgUp = %d, want 0", d.EntriesScroll)
	}
}

func TestTransferConfirmWithFlattenEnqueuesFlatJob(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	app.stopWorker()
	dst := filepath.Join(t.TempDir(), "dest")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	app.dialogCtrl.ActivateCopyAction()
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt))
	if !app.model.TransferDialog.FlattenIntoDest {
		t.Fatal("expected FlattenIntoDest set before confirm")
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should close after confirm")
	}

	all := app.jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	job := all[0]
	if job.Type != jobs.TypeCopy {
		t.Fatalf("job type = %v, want copy", job.Type)
	}
	if !job.FlattenIntoDest {
		t.Fatal("job.FlattenIntoDest should be true")
	}
	if !job.FlatDestNames() {
		t.Fatal("job.FlatDestNames() should be true")
	}
}

func TestTransferEscClearsDestinationTargetMarks(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	dst := filepath.Join(t.TempDir(), "dest")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	app.dialogCtrl.ActivateCopyAction()
	app.dialogCtrl.ApplyTransferDestinationPathValidation()
	if !app.model.DestinationTargetSecondary {
		t.Fatal("expected Secondary (inactive) panel marked as destination target")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("transfer dialog should close on Esc")
	}
	if app.model.DestinationTargetPrimary || app.model.DestinationTargetSecondary {
		t.Fatal("Esc should clear destination target panels")
	}
}

func TestOpenSelectionsRootNavigatesToCommonRoot(t *testing.T) {
	app, root, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(bravo, "stone.txt"))

	ev := tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModAlt)
	if quit, _ := app.handleKey(ev); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != root {
		t.Fatalf("panel path = %q, want common root %q", got, root)
	}

	// Already at the root: stays put.
	if quit, _ := app.handleKey(ev); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != root {
		t.Fatalf("panel path = %q, want unchanged %q", got, root)
	}
}

func TestOpenSelectionsRootSingleDirNavigatesToParent(t *testing.T) {
	app, _, alpha, bravo := setupMultiDirSelection(t)
	p := app.activePanel()
	p.AddSelection(filepath.Join(alpha, "river.txt"))
	p.AddSelection(filepath.Join(alpha, "pond.txt"))
	if err := p.Load(bravo); err != nil {
		t.Fatalf("load bravo: %v", err)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want single selection dir %q", got, alpha)
	}
}

func TestOpenSelectionsRootWithoutSelectionsKeepsLocation(t *testing.T) {
	app, _, alpha, _ := setupMultiDirSelection(t)

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().PathString(); got != alpha {
		t.Fatalf("panel path = %q, want unchanged %q", got, alpha)
	}
}
