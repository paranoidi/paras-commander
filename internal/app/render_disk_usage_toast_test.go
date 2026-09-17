package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// toastDiskUsagePainter is a minimal ui.DiskUsagePainter stub so paintDiskUsageBrowserUpdate
// treats the primary panel as an active scan scope.
type toastDiskUsagePainter struct{}

func (toastDiskUsagePainter) ByteSize(string) (int64, bool)      { return 0, false }
func (toastDiskUsagePainter) FileCount(string) (int64, bool)     { return 0, false }
func (toastDiskUsagePainter) PendingForPanel(string, int) bool   { return false }
func (toastDiskUsagePainter) DiskScanBusy() bool                 { return true }
func (toastDiskUsagePainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}
func (toastDiskUsagePainter) IsKnownExcluded(string) bool { return false }

// TestPaintDiskUsageBrowserUpdateRepaintsToast covers the regression where a disk-usage scan's
// panel-only partial paint wiped the transient status toast without repainting it, leaving it
// missing (or flickering back in on the next full render) whenever a scan-finished message landed
// while a scan-scope panel repainted.
func TestPaintDiskUsageBrowserUpdateRepaintsToast(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entryPath := filepath.Join(dir, "juniper")
	writeFile(t, entryPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ViewMode = ui.ViewBrowser
	app.model.ActiveSubFocus = ui.SubFocusFileList
	app.model.DiskUsageShown = true
	app.model.DiskUsagePanelID = ui.PrimaryPanel
	app.model.DiskUsageScanOrigin = dir
	app.model.DiskUsageScanRoots = []string{entryPath}
	app.model.DiskUsage = toastDiskUsagePainter{}
	app.model.Message = "Disk usage scan finished"

	app.render()

	if !app.paintDiskUsageBrowserUpdate() {
		t.Fatal("expected disk-usage partial paint to succeed")
	}

	w, h := screen.Size()
	layout := app.layoutForTerminalSize(w, h)
	line := screenLine(screen, layout.Footer.Y-1, w)
	if !strings.Contains(line, "Disk usage scan finished") {
		t.Fatalf("expected toast row to contain message, got %q", line)
	}
}
