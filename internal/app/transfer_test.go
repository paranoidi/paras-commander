package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestCopyMoveClearsSelectionOnlyWhenQueued(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.dialogCtrl.OpenCopyDialog()
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindCopy {
		t.Fatal("copy dialog should open")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("opening copy dialog must not clear selection")
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("copy dialog should close on Esc")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling copy dialog must not clear selection")
	}

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModAlt))
	if app.model.TransferDialog.Open {
		t.Fatal("copy dialog should close on Alt+C")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling copy dialog with Alt+C must not clear selection")
	}

	app.dialogCtrl.OpenMoveDialog()
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindMove {
		t.Fatal("move dialog should open")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("opening move dialog must not clear selection")
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("move dialog should close on Esc")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling move dialog must not clear selection")
	}

	app.dialogCtrl.OpenMoveDialog()
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt))
	if app.model.TransferDialog.Open {
		t.Fatal("move dialog should close on Alt+c")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling move dialog with Alt+C must not clear selection")
	}

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ConfirmCopy()
	if len(p.SelectedPaths) != 0 {
		t.Fatal("confirming copy should clear current-directory selection")
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job after confirmCopy, got %d", len(app.jobState.AllJobs()))
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	nJobs := len(app.jobState.AllJobs())
	app.jobsCtrl.EnqueueCopyJob()
	if len(p.SelectedPaths) != 0 {
		t.Fatal("enqueueCopyJob should clear current-directory selection")
	}
	if len(app.jobState.AllJobs()) != nJobs+1 {
		t.Fatalf("expected one new job from enqueueCopyJob")
	}
}

func TestTransferDialogEnterFromDestinationConfirms(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.dialogCtrl.OpenCopyDialog()
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want destination row", app.model.TransferDialog.FocusField)
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("copy dialog should close after Enter from destination")
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job after Enter, got %d", len(app.jobState.AllJobs()))
	}
}

func TestTransferDialogDestinationTargetPanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	// Prefilled destination is the inactive (Secondary) panel's path.
	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ApplyTransferDestinationPathValidation()
	if !app.model.DestinationTargetSecondary {
		t.Fatal("expected Secondary panel marked as destination target")
	}
	if app.model.DestinationTargetPrimary {
		t.Fatal("Primary panel should not be marked as destination target")
	}

	// Retyping the destination to the active (Primary) panel's path flips the target.
	app.model.TransferDialog.Destination.Value = dir
	app.dialogCtrl.ApplyTransferDestinationPathValidation()
	if !app.model.DestinationTargetPrimary {
		t.Fatal("expected Primary panel marked as destination target")
	}
	if app.model.DestinationTargetSecondary {
		t.Fatal("Secondary panel should no longer be marked as destination target")
	}

	app.dialogCtrl.CloseTransferDialog()
	if app.model.DestinationTargetPrimary || app.model.DestinationTargetSecondary {
		t.Fatal("closing the dialog should clear destination target panels")
	}
}

func TestTransferDialogDestinationTargetPanelBorderColor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ApplyTransferDestinationPathValidation()
	app.render()

	width, height := app.screen.Size()
	layout := app.layoutForTerminalSize(width, height)
	styles := theme.Default()
	wantFG, _, _ := styles.PanelTargetFrame.Decompose()

	_, secStyle, _ := screen.Get(layout.Secondary.X, layout.Secondary.Y)
	secFG, _, _ := secStyle.Decompose()
	if secFG != wantFG {
		t.Fatalf("Secondary panel border fg = %v, want target frame fg %v", secFG, wantFG)
	}

	_, priStyle, _ := screen.Get(layout.Primary.X, layout.Primary.Y)
	priFG, _, _ := priStyle.Decompose()
	if priFG == wantFG {
		t.Fatal("Primary panel border should not use the target frame color")
	}
}

func TestTransferDialogDestinationLeftRightMoveCursor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.dialogCtrl.OpenCopyDialog()
	d := &app.model.TransferDialog
	if d.FocusField != 0 || d.Phase != dialog.TransferPhaseDestination {
		t.Fatalf("unexpected initial dialog state: focus=%d phase=%v", d.FocusField, d.Phase)
	}
	startCursor := d.Destination.Cursor
	if startCursor == 0 {
		t.Fatalf("expected destination prefill cursor > 0, got %d", startCursor)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if d.Destination.Cursor != startCursor-1 {
		t.Fatalf("Left: cursor = %d, want %d", d.Destination.Cursor, startCursor-1)
	}
	if d.DestSubFocus != dialog.TransferDestSubFocusText {
		t.Fatalf("Left changed sub-focus to %v, want text", d.DestSubFocus)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if d.Destination.Cursor != startCursor-2 {
		t.Fatalf("Left again: cursor = %d, want %d", d.Destination.Cursor, startCursor-2)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if d.Destination.Cursor != startCursor-1 {
		t.Fatalf("Right: cursor = %d, want %d", d.Destination.Cursor, startCursor-1)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	wantInsertPos := startCursor - 1
	runes := []rune(d.Destination.Value)
	if wantInsertPos < 0 || wantInsertPos >= len(runes) || runes[wantInsertPos] != 'X' {
		t.Fatalf("expected 'X' inserted at rune %d in %q", wantInsertPos, d.Destination.Value)
	}
	if d.Destination.Cursor != wantInsertPos+1 {
		t.Fatalf("after insert: cursor = %d, want %d", d.Destination.Cursor, wantInsertPos+1)
	}
}

func TestTransferDestinationFooterShowsActiveAndInactive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.dialogCtrl.OpenCopyDialog()
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want 0 (destination)", app.model.TransferDialog.FocusField)
	}
	keys := app.activeFooterKeys()
	if !footerHasHint(keys, "Active path ◄", "S-left") {
		t.Fatalf("footer = %+v, want Active S-left hint", keys)
	}
	if !footerHasHint(keys, "Inactive path ►", "S-right") {
		t.Fatalf("footer = %+v, want Inactive S-right hint", keys)
	}
}

func TestTransferDestinationShortcutSetsActiveAndInactivePath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dstDir

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.dialogCtrl.OpenCopyDialog()
	wantActive := dialogctrl.TransferPrefilledDestination(app.activePanel().PathString()).Value
	wantInactive := dialogctrl.TransferPrefilledDestination(app.inactivePanel().PathString()).Value

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if app.model.TransferDialog.Destination.Value != wantInactive {
		t.Fatalf("after Shift+Right destination = %q, want %q", app.model.TransferDialog.Destination.Value, wantInactive)
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	if app.model.TransferDialog.Destination.Value != wantActive {
		t.Fatalf("after Shift+Left destination = %q, want %q", app.model.TransferDialog.Destination.Value, wantActive)
	}
}

func TestTransferDestinationShortcutNoOpDuringSelfCopyRename(t *testing.T) {
	dir := t.TempDir()
	aaa := filepath.Join(dir, "aaa")
	if err := os.Mkdir(aaa, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aaa: true}
	app.jobsCtrl.EnqueueCopyJob()
	if app.model.TransferDialog.Phase != dialog.TransferPhaseSelfCopyRename || app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("want self-copy rename dialog with FocusField 0, got %+v", app.model.TransferDialog)
	}
	want := app.model.TransferDialog.SelfCopyNewName.Value

	keys := app.activeFooterKeys()
	if footerHasHint(keys, "Active path ◄", "S-left") || footerHasHint(keys, "Inactive path ►", "S-right") {
		t.Fatalf("footer = %+v, must not show Active/Inactive during self-copy rename", keys)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	if app.model.TransferDialog.SelfCopyNewName.Value != want {
		t.Fatalf("Shift+Left mutated SelfCopyNewName = %q, want unchanged %q", app.model.TransferDialog.SelfCopyNewName.Value, want)
	}
}

func TestTransferSelfCopyRenameFlow(t *testing.T) {
	t.Run("dialog OK enters rename phase without queueing", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}

		app.dialogCtrl.OpenCopyDialog()
		app.dialogCtrl.ConfirmCopy()
		if !app.model.TransferDialog.Open {
			t.Fatal("dialog should stay open")
		}
		if app.model.TransferDialog.Phase != dialog.TransferPhaseSelfCopyRename {
			t.Fatalf("phase = %v, want SelfCopyRename", app.model.TransferDialog.Phase)
		}
		if len(app.jobState.AllJobs()) != 0 {
			t.Fatal("no job should be queued yet")
		}
		if len(p.SelectedPaths) == 0 {
			t.Fatal("selection should remain until the job is queued")
		}
	})

	t.Run("enqueueCopyJob opens rename phase", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}
		app.jobsCtrl.EnqueueCopyJob()
		if !app.model.TransferDialog.Open || app.model.TransferDialog.Phase != dialog.TransferPhaseSelfCopyRename {
			t.Fatalf("want self-copy rename dialog, got %+v", app.model.TransferDialog)
		}
		if len(p.SelectedPaths) == 0 {
			t.Fatal("enqueue should not clear selection until confirm")
		}
	})

	t.Run("same new name shows error", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}
		app.dialogCtrl.OpenCopyDialog()
		app.dialogCtrl.ConfirmCopy()
		app.model.TransferDialog.SelfCopyNewName = dialog.FileDialogField{
			Value:          "aaa",
			Prefill:        "aaa",
			Cursor:         len([]rune("aaa")),
			PrefillPending: false,
		}
		app.dialogCtrl.ConfirmCopy()
		if !strings.Contains(app.model.Message, "New name must differ") {
			t.Fatalf("message = %q", app.model.Message)
		}
		if len(app.jobState.AllJobs()) != 0 {
			t.Fatal("job should not enqueue")
		}
	})

	t.Run("distinct new name queues job", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}
		app.dialogCtrl.OpenCopyDialog()
		app.dialogCtrl.ConfirmCopy()
		app.model.TransferDialog.SelfCopyNewName = dialog.FileDialogField{
			Value:          "aaa2",
			Prefill:        "aaa2",
			Cursor:         len([]rune("aaa2")),
			PrefillPending: false,
		}
		app.dialogCtrl.ConfirmCopy()
		if len(app.jobState.AllJobs()) != 1 {
			t.Fatalf("expected 1 job, got %d", len(app.jobState.AllJobs()))
		}
		j := app.jobState.AllJobs()[0]
		wantDest := filepath.Join(dir, "aaa2")
		if filepath.Clean(j.Destination.String()) != filepath.Clean(wantDest) {
			t.Fatalf("Destination = %q, want %q", j.Destination, wantDest)
		}
		if len(p.SelectedPaths) != 0 {
			t.Fatal("selection cleared after queue")
		}
		waitUntilAppJobsFinished(t, app, 5*time.Second)
	})
}

func TestTransferSelfCopyMultipleSourcesRejected(t *testing.T) {
	dir := t.TempDir()
	aaa := filepath.Join(dir, "aaa")
	bbb := filepath.Join(dir, "bbb.txt")
	if err := os.Mkdir(aaa, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, bbb)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aaa: true, bbb: true}
	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ConfirmCopy()
	if app.model.TransferDialog.Phase != dialog.TransferPhaseDestination {
		t.Fatalf("phase = %v, want Destination", app.model.TransferDialog.Phase)
	}
	if !strings.Contains(app.model.Message, "multiple items") {
		t.Fatalf("message = %q", app.model.Message)
	}
}

func TestEnqueueMoveJobSameDirUnsupportedDoesNotClearSelection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "only.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	app.jobsCtrl.EnqueueMoveJob()
	if len(p.SelectedPaths) == 0 {
		t.Fatal("unsupported same-directory move should leave selection intact")
	}
}

func TestEnqueueCopyJobClearsCrossDirectorySelections(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "here")
	other := filepath.Join(dir, "other")
	dst := filepath.Join(dir, "dest")
	if err := os.MkdirAll(here, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(here, "here.txt"))
	writeFile(t, filepath.Join(other, "other.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if err := app.activePanel().Load(here); err != nil {
		t.Fatalf("Load here: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, active panel enters here
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dest: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dst

	p := app.activePanel()
	hereTxt := filepath.Join(here, "here.txt")
	otherTxt := filepath.Join(other, "other.txt")
	p.SelectedPaths = map[string]bool{hereTxt: true, otherTxt: true}
	p.SelectionsStripOrder = []string{otherTxt}

	app.jobsCtrl.EnqueueCopyJob()

	if p.SelectedPaths != nil {
		t.Fatalf("expected all queued sources removed from selection, got %#v", p.SelectedPaths)
	}
	if len(p.SelectionsStripOrder) != 0 {
		t.Fatalf("expected selections strip order cleared, got %v", p.SelectionsStripOrder)
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job, got %d", len(app.jobState.AllJobs()))
	}
}

func TestEnqueueMoveJobClearsCrossDirectorySelections(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "here")
	other := filepath.Join(dir, "other")
	dst := filepath.Join(dir, "dest")
	if err := os.MkdirAll(here, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(here, "here.txt"))
	writeFile(t, filepath.Join(other, "other.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if err := app.activePanel().Load(here); err != nil {
		t.Fatalf("Load here: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, active panel enters here
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dest: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, inactive panel enters dst

	p := app.activePanel()
	hereTxt := filepath.Join(here, "here.txt")
	otherTxt := filepath.Join(other, "other.txt")
	p.SelectedPaths = map[string]bool{hereTxt: true, otherTxt: true}
	p.SelectionsStripOrder = []string{otherTxt}

	app.jobsCtrl.EnqueueMoveJob()

	if p.SelectedPaths != nil {
		t.Fatalf("expected all queued sources removed from selection, got %#v", p.SelectedPaths)
	}
	if len(p.SelectionsStripOrder) != 0 {
		t.Fatalf("expected selections strip order cleared, got %v", p.SelectionsStripOrder)
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job, got %d", len(app.jobState.AllJobs()))
	}
}

func TestPathPickerHostFooterShowsPathsOnCopyAndSymlinkDialogs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"))
	dst := filepath.Join(root, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("m : %s\n", dst)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	screen := newScreen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.dialogCtrl.OpenCopyDialog()
	if !app.model.TransferDialog.Open {
		t.Fatal("copy dialog should open")
	}
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want 0 (destination)", app.model.TransferDialog.FocusField)
	}
	keys := app.activeFooterKeys()
	if len(keys) != 6 {
		t.Fatalf("footer len = %d, want Esc + Default + Bookmarks + Active + Inactive + F10", len(keys))
	}
	if keys[1].Hint != "Default" || keys[1].KeyLabel != "C-r" {
		t.Fatalf("restore footer = %+v, want C-r Default", keys[1])
	}
	if keys[2].Hint != "Bookmarks" || keys[2].KeyLabel != "C-b" {
		t.Fatalf("bookmarks footer = %+v, want C-b Bookmarks", keys[2])
	}
	if !footerHasHint(keys, "Active path ◄", "S-left") {
		t.Fatalf("footer = %+v, want Active S-left hint", keys)
	}
	if !footerHasHint(keys, "Inactive path ►", "S-right") {
		t.Fatalf("footer = %+v, want Inactive S-right hint", keys)
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	app.dispatch(keymap.ActionFileSymlink)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogSymlink {
		t.Fatal("symlink dialog should be open")
	}
	keys = app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("symlink footer len = %d, want Esc + Paths + F10", len(keys))
	}
	if keys[1].Hint != "Bookmarks" {
		t.Fatalf("symlink footer middle = %+v, want Bookmarks hint", keys[1])
	}
}

func TestPathPickerHostBookmarkOpenOpensPickerFromCopyAndSymlinkDialogs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"))
	dst := filepath.Join(root, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("m : %s\n", dst)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	screen := newScreen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.dialogCtrl.OpenCopyDialog()
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want 0", app.model.TransferDialog.FocusField)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
	if !app.model.PathPicker.Open || app.model.PathPicker.Purpose != dialog.PathPickerPurposeApplyTransferDestination {
		t.Fatalf("path picker = open %v purpose %v, want ApplyTransferDestination",
			app.model.PathPicker.Open, app.model.PathPicker.Purpose)
	}
	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.PathPicker.Open {
		t.Fatal("path picker should close")
	}
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	app.dispatch(keymap.ActionFileSymlink)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogSymlink {
		t.Fatal("symlink dialog should be open")
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
	if !app.model.PathPicker.Open || app.model.PathPicker.Purpose != dialog.PathPickerPurposeApplyFileDialogField {
		t.Fatalf("path picker = open %v purpose %v, want ApplyFileDialogField",
			app.model.PathPicker.Open, app.model.PathPicker.Purpose)
	}
}

func TestDuplicateOpensRenameLikeDialogForSingleDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "project")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileDuplicate)
	if !app.model.FileDialog.Open {
		t.Fatal("expected duplicate dialog open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogDuplicate {
		t.Fatalf("dialog type = %v, want FileDialogDuplicate", app.model.FileDialog.DialogType)
	}
	if app.model.FileDialog.DuplicateSource != src {
		t.Fatalf("DuplicateSource = %q, want %q", app.model.FileDialog.DuplicateSource, src)
	}
	if len(app.model.FileDialog.Fields) != 1 || app.model.FileDialog.Fields[0].Value != "project" {
		t.Fatalf("name field = %+v, want prefilled project", app.model.FileDialog.Fields)
	}
}

func TestDuplicateRejectsMultipleSelections(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	for _, p := range []string{a, b} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{a: true, b: true}

	app.dispatch(keymap.ActionFileDuplicate)
	if app.model.FileDialog.Open {
		t.Fatal("dialog should stay closed for multiple selections")
	}
	if !strings.Contains(app.model.Message, "single file or directory") {
		t.Fatalf("message = %q, want single file-or-directory error", app.model.Message)
	}
}

func TestDuplicateOpensDialogForFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	writeFile(t, file)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{file: true}

	app.dispatch(keymap.ActionFileDuplicate)
	if !app.model.FileDialog.Open {
		t.Fatal("expected duplicate dialog open for file selection")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogDuplicate {
		t.Fatalf("dialog type = %v, want FileDialogDuplicate", app.model.FileDialog.DialogType)
	}
	if app.model.FileDialog.DuplicateSource != file {
		t.Fatalf("DuplicateSource = %q, want %q", app.model.FileDialog.DuplicateSource, file)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "note.txt" {
		t.Fatalf("prefill value = %q, want %q", got, "note.txt")
	}
}

func TestDuplicateDialogFocusCheckboxToggle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "project")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileDuplicate)
	if app.model.FileDialog.RenameFocusAfter {
		t.Fatal("RenameFocusAfter = true, want false (default)")
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))
	if !app.model.FileDialog.RenameFocusAfter {
		t.Fatal("Alt+A should toggle focus-after checkbox on")
	}
	okIdx := dialog.FileDialogOKFocusIndex(app.model.FileDialog)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("Down from field: focus = %d, want 1 (checkbox)", app.model.FileDialog.FocusedField)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.RenameFocusAfter {
		t.Fatal("Enter on checkbox should toggle focus-after off")
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != okIdx {
		t.Fatalf("Down from checkbox: focus = %d, want OK %d", app.model.FileDialog.FocusedField, okIdx)
	}
}

func TestDuplicateWithFocusAfterSelectsAfterJob(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d", i)
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src := filepath.Join(dir, "10")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	selectPanelEntryByName(t, p, "10")
	p.SelectedPaths = map[string]bool{src: true}

	newName := "99"
	app.dispatch(keymap.ActionFileDuplicate)
	for _, r := range newName {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Drain screen events continuously while the job runs (rather than in one big batch
	// afterward): tcell's event queue is a bounded, non-blocking buffer (10 slots) that silently
	// drops a PostEvent when full, and the job's own progress/completion events share that same
	// queue with our panel-async-load events — letting it fill up between big drain calls is what
	// makes this flaky, not the async-load mechanism itself.
	applied := false
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		app.jobsCtrl.PollEvents()
		if !applied {
			applied = app.jobsCtrl.ApplyRefreshes()
		}
		e, ok := app.activePanel().CurrentEntry()
		return ok && e.Name == newName
	})

	p = app.activePanel()
	entry, ok := p.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false after duplicate job")
	}
	if entry.Name != newName {
		t.Fatalf("cursor entry = %q, want %s", entry.Name, newName)
	}
	vp := app.activeViewportRows()
	wantScroll := p.Cursor - vp/2
	if wantScroll < 0 {
		wantScroll = 0
	}
	maxOffset := p.VisibleEntryCount() - vp
	if maxOffset < 0 {
		maxOffset = 0
	}
	if wantScroll > maxOffset {
		wantScroll = maxOffset
	}
	if p.ScrollOffset != wantScroll {
		t.Fatalf("ScrollOffset = %d, want %d (centered on copied entry)", p.ScrollOffset, wantScroll)
	}
}

func TestDuplicateConfirmsFromOKButtonWithFocusAfter(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	writeFile(t, file)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	selectPanelEntryByName(t, p, "note.txt")
	p.SelectedPaths = map[string]bool{file: true}

	app.dispatch(keymap.ActionFileDuplicate)
	// Replace the prefilled name.
	for _, r := range "copy.txt" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// Enable focus-after, then navigate focus down to the OK button and confirm
	// from there (the real user flow that used to drop the copy silently).
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))
	okIdx := dialog.FileDialogOKFocusIndex(app.model.FileDialog)
	for app.model.FileDialog.FocusedField != okIdx {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after OK")
	}
	flushBackgroundJobs(t, app)
	app.jobsCtrl.ApplyRefreshes()

	if _, err := os.Stat(filepath.Join(dir, "copy.txt")); err != nil {
		t.Fatalf("expected duplicated file at copy.txt: %v", err)
	}
}

func TestDuplicateQueuesJobWithNewName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "alpha")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileDuplicate)
	for _, r := range "beta" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	jobsList := app.jobState.AllJobs()
	if len(jobsList) != 1 {
		t.Fatalf("expected 1 copy job, got %d", len(jobsList))
	}
	wantDest := filepath.Join(dir, "beta")
	if got := filepath.Clean(jobsList[0].Destination.String()); got != filepath.Clean(wantDest) {
		t.Fatalf("job destination = %q, want %q", got, wantDest)
	}
	if len(jobsList[0].Sources) != 1 || filepath.Clean(jobsList[0].Sources[0].String()) != filepath.Clean(src) {
		t.Fatalf("job sources = %+v, want [%q]", jobsList[0].Sources, src)
	}
}
