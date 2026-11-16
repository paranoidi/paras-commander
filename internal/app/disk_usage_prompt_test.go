package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func pressDiskUsageScan(t *testing.T, app *App) {
	t.Helper()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt)); quit {
		t.Fatal("handleKey quit on the disk usage scan key")
	}
}

// newDiskUsagePromptApp returns an app whose first disk usage scan request has run.
func newDiskUsagePromptApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "meadow", "lantern"))
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.model.ViewMode = ui.ViewBrowser
	pressDiskUsageScan(t, app)
	if app.model.DiskUsagePrompt.Open {
		t.Fatal("first scan request must start a scan, not ask")
	}
	if !app.model.DiskUsageShown {
		t.Fatal("first scan request should show disk usage")
	}
	pressDiskUsageScan(t, app)
	if !app.model.DiskUsagePrompt.Open {
		t.Fatal("second scan request should open the disk usage prompt")
	}
	if app.model.DiskUsagePrompt.Focus != dialog.DiskUsageRescan {
		t.Fatalf("focus = %q, want Rescan", app.model.DiskUsagePrompt.Focus)
	}
	return app
}

func TestDiskUsagePromptClear(t *testing.T) {
	t.Parallel()
	app := newDiskUsagePromptApp(t)
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModAlt))
	if app.model.DiskUsagePrompt.Open {
		t.Fatal("Clear should close the prompt")
	}
	if app.model.DiskUsageShown || app.model.DiskUsageScanOrigin != "" {
		t.Fatal("Clear should drop the disk usage view and scope")
	}
	pressDiskUsageScan(t, app)
	if app.model.DiskUsagePrompt.Open || !app.model.DiskUsageShown {
		t.Fatal("after Clear the scan request should start a scan again")
	}
}

func TestDiskUsagePromptRescan(t *testing.T) {
	t.Parallel()
	app := newDiskUsagePromptApp(t)
	app.model.Message = ""
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.DiskUsagePrompt.Open {
		t.Fatal("Rescan should close the prompt")
	}
	if !app.model.DiskUsageShown || app.model.DiskUsageScanOrigin == "" {
		t.Fatal("Rescan should keep the disk usage view")
	}
	if app.model.Message == "" {
		t.Fatal("Rescan should start a scan (scan-started toast)")
	}
}

func TestDiskUsagePromptCancel(t *testing.T) {
	t.Parallel()
	app := newDiskUsagePromptApp(t)
	app.model.Message = ""
	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.model.DiskUsagePrompt.Open {
		t.Fatal("Esc should close the prompt")
	}
	if !app.model.DiskUsageShown || app.model.Message != "" {
		t.Fatal("Cancel should leave disk usage untouched")
	}
}
