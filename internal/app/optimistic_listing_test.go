package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMoveEnqueueRemovesSourcesImmediately(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	willow := filepath.Join(srcDir, "willow.txt")
	harbor := filepath.Join(srcDir, "harbor.txt")
	writeFile(t, willow)
	writeFile(t, harbor)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{willow: true, harbor: true}
	app.jobsCtrl.EnqueueMoveJob()

	if p.SelectVisibleEntry("willow.txt") || p.SelectVisibleEntry("harbor.txt") {
		t.Fatal("moved sources should vanish from the listing before async reload")
	}
	dst := app.inactivePanel()
	if !dst.SelectVisibleEntry("willow.txt") || !dst.SelectVisibleEntry("harbor.txt") {
		t.Fatal("dest panel should show optimistic inserts for moved sources")
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("jobs = %d, want 1", len(app.jobState.AllJobs()))
	}
}

func TestDeleteEnqueueRemovesSourcesImmediately(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "willow.txt")
	keep := filepath.Join(dir, "harbor.txt")
	writeFile(t, gone)
	writeFile(t, keep)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{gone: true}

	app.jobsCtrl.EnqueueDeleteJob([]string{gone}, false, true)

	if p.SelectVisibleEntry("willow.txt") {
		t.Fatal("deleted source should vanish immediately")
	}
	if !p.SelectVisibleEntry("harbor.txt") {
		t.Fatal("unrelated entry should remain")
	}
}

func TestCopyEnqueueInsertsDestWithoutRemovingSource(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	for _, d := range []string{srcDir, dstDir} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src := filepath.Join(srcDir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	applyNextInterruptEvent(t, app, screen)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}
	app.jobsCtrl.EnqueueCopyJob()

	if !p.SelectVisibleEntry("willow.txt") {
		t.Fatal("copy must not remove the source row")
	}
	if !app.inactivePanel().SelectVisibleEntry("willow.txt") {
		t.Fatal("dest panel should show optimistic copy insert")
	}
}

func TestRefreshAfterOptimisticRemoveRestoresWhenStillOnDisk(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "willow.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	if !p.RemoveEntriesByPath([]string{src}, app.activeViewportRows()) {
		t.Fatal("expected prune")
	}
	if p.SelectVisibleEntry("willow.txt") {
		t.Fatal("expected optimistic remove")
	}

	app.dialogCtrl.RefreshBothPanels()
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return app.activePanel().SelectVisibleEntry("willow.txt")
	})
	if !app.activePanel().SelectVisibleEntry("willow.txt") {
		t.Fatal("disk refresh should restore the source row while it still exists")
	}
}
