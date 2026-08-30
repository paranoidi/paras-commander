package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestBrowserListNavPartialRenderEligibleSkipsSyncWithoutDebounce(t *testing.T) {
	t.Parallel()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewBrowser
	app.model.ActiveSubFocus = ui.SubFocusFileList
	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.PrimaryPanel
	app.syncFollowNavSkipReconcile.Store(false)
	if app.browserListNavPartialRenderEligible() {
		t.Fatal("expected full render when latched sync is not debouncing")
	}
	app.syncFollowNavSkipReconcile.Store(true)
	if !app.browserListNavPartialRenderEligible() {
		t.Fatal("expected partial render while sync nav is debouncing")
	}
}

// TestBrowserListNavPartialRenderEligibleSkipsWhileOverlayOpen covers the regression where an
// async panel load landing while the find (or any other) dialog was open repainted the panel
// column straight over the dialog.
func TestBrowserListNavPartialRenderEligibleSkipsWhileOverlayOpen(t *testing.T) {
	t.Parallel()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewBrowser
	app.model.ActiveSubFocus = ui.SubFocusFileList
	if !app.browserListNavPartialRenderEligible() {
		t.Fatal("expected partial render with no overlay open")
	}
	app.model.FindDialog.Open = true
	if app.browserListNavPartialRenderEligible() {
		t.Fatal("expected full render while the find dialog is open")
	}
	if app.paintDiskUsageBrowserUpdate() {
		t.Fatal("expected disk-usage partial paint to decline while the find dialog is open")
	}
	app.model.FindDialog.Open = false
	app.model.FileDialog.Open = true
	if app.browserListNavPartialRenderEligible() {
		t.Fatal("expected full render while a file dialog (mass rename) is open")
	}
}
