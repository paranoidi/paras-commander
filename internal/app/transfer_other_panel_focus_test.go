package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
)

func TestCopyToOtherPanelFocusesTransferredEntry(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "source")
	dstDir := filepath.Join(root, "dest")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Pad dest so the transferred name is not already under the cursor by accident.
	for _, name := range []string{"anchor.txt", "beacon.txt", "meadow.txt"} {
		writeFile(t, filepath.Join(dstDir, name))
	}
	src := filepath.Join(srcDir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	selectPanelEntryByName(t, app.inactivePanel(), "meadow.txt")

	p := app.activePanel()
	selectPanelEntryByName(t, p, "willow.txt")
	p.SelectedPaths = map[string]bool{src: true}

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ConfirmCopy()

	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		app.jobsCtrl.PollEvents()
		_ = app.jobsCtrl.ApplyRefreshes()
		e, ok := app.inactivePanel().CurrentEntry()
		return ok && e.Name == "willow.txt"
	})

	e, ok := app.inactivePanel().CurrentEntry()
	if !ok || e.Name != "willow.txt" {
		t.Fatalf("inactive cursor = %v ok=%v, want willow.txt", e, ok)
	}
}

func TestCopyToOtherPanelSelectsFirstInPanelOrder(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "source")
	dstDir := filepath.Join(root, "dest")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dstDir, "anchor.txt"))
	// Source names sort as cedar before zebra; select zebra first in SelectedPaths order
	// so candidate source order differs from panel listing order.
	cedar := filepath.Join(srcDir, "cedar.txt")
	zebra := filepath.Join(srcDir, "zebra.txt")
	writeFile(t, cedar)
	writeFile(t, zebra)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	selectPanelEntryByName(t, app.inactivePanel(), "anchor.txt")

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{zebra: true, cedar: true}

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ConfirmCopy()

	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		app.jobsCtrl.PollEvents()
		_ = app.jobsCtrl.ApplyRefreshes()
		e, ok := app.inactivePanel().CurrentEntry()
		return ok && e.Name == "cedar.txt"
	})

	e, ok := app.inactivePanel().CurrentEntry()
	if !ok || e.Name != "cedar.txt" {
		t.Fatalf("inactive cursor = %v ok=%v, want cedar.txt (first in panel order)", e, ok)
	}
}

func TestCopyToOtherPanelSkipsWhenUserMovedCursor(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "source")
	dstDir := filepath.Join(root, "dest")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dstDir, "anchor.txt"))
	writeFile(t, filepath.Join(dstDir, "meadow.txt"))
	src := filepath.Join(srcDir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	selectPanelEntryByName(t, app.inactivePanel(), "anchor.txt")

	p := app.activePanel()
	selectPanelEntryByName(t, p, "willow.txt")
	p.SelectedPaths = map[string]bool{src: true}

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ConfirmCopy()
	// Move inactive cursor before any refresh/reconcile applies pending focus.
	selectPanelEntryByName(t, app.inactivePanel(), "meadow.txt")

	flushBackgroundJobs(t, app)
	_ = app.jobsCtrl.ApplyRefreshes()
	app.dialogCtrl.ReconcilePendingPanelFocus()

	e, ok := app.inactivePanel().CurrentEntry()
	if !ok || e.Name != "meadow.txt" {
		t.Fatalf("inactive cursor = %v ok=%v, want meadow.txt (user choice preserved)", e, ok)
	}
}

func TestCopyToOtherPanelFocusDisabledByConfig(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "source")
	dstDir := filepath.Join(root, "dest")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dstDir, "anchor.txt"))
	src := filepath.Join(srcDir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 24)
	cfg := config.Default()
	cfg.Operations.FocusOtherPanelAfterTransfer = false
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return srcDir, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	app.config.UI.KeyRepeatDebounceMS = 0
	app.config.Shell.Persistent = false
	t.Cleanup(func() {
		if !app.jobStopOnce {
			flushBackgroundJobs(t, app)
		}
		app.stopWorker()
	})
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	selectPanelEntryByName(t, app.inactivePanel(), "anchor.txt")

	p := app.activePanel()
	selectPanelEntryByName(t, p, "willow.txt")
	p.SelectedPaths = map[string]bool{src: true}

	app.dialogCtrl.OpenCopyDialog()
	app.dialogCtrl.ConfirmCopy()
	flushBackgroundJobs(t, app)
	_ = app.jobsCtrl.ApplyRefreshes()
	app.dialogCtrl.ReconcilePendingPanelFocus()

	e, ok := app.inactivePanel().CurrentEntry()
	if !ok || e.Name != "anchor.txt" {
		t.Fatalf("inactive cursor = %v ok=%v, want anchor.txt (focus disabled)", e, ok)
	}
}

func TestCopyToNonOtherPanelDoesNotFocusInactive(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "source")
	dstDir := filepath.Join(root, "dest")
	otherDir := filepath.Join(root, "other")
	for _, d := range []string{srcDir, dstDir, otherDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dstDir, "anchor.txt"))
	writeFile(t, filepath.Join(otherDir, "keeper.txt"))
	src := filepath.Join(srcDir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	selectPanelEntryByName(t, app.inactivePanel(), "anchor.txt")

	p := app.activePanel()
	selectPanelEntryByName(t, p, "willow.txt")
	p.SelectedPaths = map[string]bool{src: true}

	app.dialogCtrl.OpenCopyDialog()
	app.model.TransferDialog.Destination.Value = otherDir
	app.model.TransferDialog.Destination.PrefillPending = false
	app.dialogCtrl.ConfirmCopy()
	flushBackgroundJobs(t, app)
	_ = app.jobsCtrl.ApplyRefreshes()
	app.dialogCtrl.ReconcilePendingPanelFocus()

	e, ok := app.inactivePanel().CurrentEntry()
	if !ok || e.Name != "anchor.txt" {
		t.Fatalf("inactive cursor = %v ok=%v, want anchor.txt (dest was not other panel)", e, ok)
	}
	if _, err := os.Stat(filepath.Join(otherDir, "willow.txt")); err != nil {
		t.Fatalf("expected copy into otherDir: %v", err)
	}
}
