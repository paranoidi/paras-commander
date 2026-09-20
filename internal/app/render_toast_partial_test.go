package app

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const (
	longToastText  = "This is a very long transient status toast for remnant testing"
	shortToastText = "Hi"
)

func TestPartialPaintShortToastMatchesFullRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*testing.T, *App)
		paint func(*App)
	}{
		{
			name: "find overlay",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.FindDialog.Open = true
			},
			paint: func(app *App) {
				if !app.paintFindDialogOverlay() {
					app.render()
				}
			},
		},
		{
			name: "file dialog overlay",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.FileDialog.Open = true
				app.model.FileDialog.DialogType = dialog.FileDialogMkdir
			},
			paint: func(app *App) {
				if !app.paintFileDialogOverlay() {
					app.render()
				}
			},
		},
		{
			name: "browser list nav",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.ViewMode = ui.ViewBrowser
				app.model.ActiveSubFocus = ui.SubFocusFileList
			},
			paint: func(app *App) {
				app.renderBrowserListNavUpdate(ui.PrimaryPanel)
			},
		},
		{
			name: "disk usage panels",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				armToastDiskUsagePaint(t, app)
			},
			paint: func(app *App) {
				if !app.paintDiskUsageBrowserUpdate() {
					app.render()
				}
			},
		},
		{
			name: "disk usage with terminal separator",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				armToastDiskUsagePaint(t, app)
				app.model.TerminalPanel.Visible = true
				app.model.TerminalPanel.Rows = 5
			},
			paint: func(app *App) {
				if !app.paintDiskUsageBrowserUpdate() {
					app.render()
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "juniper"))
			screen := newScreen(t, 80, 24)
			app := newApp(t, screen, dir)
			app.model.ViewMode = ui.ViewBrowser
			tc.setup(t, app)

			app.model.Message = longToastText
			app.render()
			app.model.Message = shortToastText
			tc.paint(app)
			got := ui.HashScreenLogical(app.screen)

			app.render()
			want := ui.HashScreenLogical(app.screen)
			if got != want {
				t.Fatal("shorter toast after a partial paint must pixel-match a full render")
			}
			if !screenHasText(screen, shortToastText) {
				t.Fatal("expected the short toast to remain visible")
			}
			if screenHasText(screen, "remnant testing") {
				t.Fatal("long-toast remnants must not remain after replacement")
			}
		})
	}
}

func armToastDiskUsagePaint(t *testing.T, app *App) {
	t.Helper()
	dir := app.model.Primary.PathString()
	app.model.ViewMode = ui.ViewBrowser
	app.model.ActiveSubFocus = ui.SubFocusFileList
	app.model.DiskUsageShown = true
	app.model.DiskUsagePanelID = ui.PrimaryPanel
	app.model.DiskUsageScanOrigin = dir
	app.model.DiskUsageScanRoots = []string{dir}
	app.model.DiskUsage = toastDiskUsagePainter{}
}
