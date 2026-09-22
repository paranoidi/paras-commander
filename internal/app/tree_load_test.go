package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestTreeExpandEnqueuesDiskUsageScan covers the tree-expand -> disk-usage feeder in
// applyTreeChildResults: expanding a directory that isn't already cached should queue a scan for
// it on the disk-usage engine, so the size column can show it without a separate selection or
// explicit disk-usage scan.
func TestTreeExpandEnqueuesDiskUsageScan(t *testing.T) {
	dir := t.TempDir()
	meadow := filepath.Join(dir, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	content := make([]byte, 777)
	if err := os.WriteFile(filepath.Join(meadow, "lantern.txt"), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.disk.engine = diskusage.New()

	// Directories sort first, so the cursor already sits on "meadow".
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	if app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatal("Space should enable tree layout on first use")
	}
	applyNextInterruptEvent(t, app, screen)

	// The scan runs on the engine's own worker and pokes engine.Updates(), not the screen, so
	// the wait is on the cache rather than on a screen event.
	var size int64
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		var ok bool
		size, ok = app.disk.engine.ByteSize(meadow)
		return ok
	})
	if size != int64(len(content)) {
		t.Fatalf("ByteSize(%q) = %d, want %d", meadow, size, len(content))
	}
}

// TestCancelTreeChildLoadsDropsInFlightFetch covers the ctx-cancellation half of
// treeChildLoader: calling CancelTreeChildLoads (as State.abandonTreeChildLoads does on
// collapse-all/re-root/leaving tree mode) right after a fetch is dispatched must stop that
// fetch's result from ever reaching treeChildResults, even though the goroutine itself still
// runs to completion.
func TestCancelTreeChildLoadsDropsInFlightFetch(t *testing.T) {
	dir := t.TempDir()
	meadow := filepath.Join(dir, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	loc, err := pathloc.Parse(meadow)
	if err != nil {
		t.Fatalf("pathloc.Parse: %v", err)
	}
	if ok := app.model.Primary.ScheduleTreeChildLoad(panel.TreeChildLoadRequest{DirID: meadow, Loc: loc}); !ok {
		t.Fatal("ScheduleTreeChildLoad returned false")
	}
	app.model.Primary.CancelTreeChildLoads()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(app.treeChildResults.drain()) != 0 {
			t.Fatal("canceled fetch still delivered a result to treeChildResults")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
