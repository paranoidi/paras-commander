package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// waitForDiskUsageByteSize polls the disk-usage engine cache for absPath, draining any pending
// screen interrupts along the way (the scan itself runs on the engine's own worker goroutine and
// pokes engine.Updates(), not the screen — see disk_usage.go's pollDiskUsageUpdates), until it
// reports a size or the deadline passes.
func waitForDiskUsageByteSize(t *testing.T, app *App, absPath string) (int64, bool) {
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
		if size, ok := app.disk.engine.ByteSize(absPath); ok {
			return size, true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return 0, false
}

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

	size, ok := waitForDiskUsageByteSize(t, app, meadow)
	if !ok {
		t.Fatal("timeout waiting for disk-usage engine to cache the expanded directory's size")
	}
	if size != int64(len(content)) {
		t.Fatalf("ByteSize(%q) = %d, want %d", meadow, size, len(content))
	}
}
