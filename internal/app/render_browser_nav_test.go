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
