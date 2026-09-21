package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestPaintDiskUsageBrowserUpdateMatchesFullRenderTerminalFocusSelectionsStrip(t *testing.T) {
	t.Parallel()
	app := newBrowserPlanApp(t)
	armToastDiskUsagePaint(t, app)
	armSelectionsStrip(t, app)
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	app.model.TerminalPanel.Visible = true
	app.model.TerminalPanel.Focused = true
	app.model.TerminalPanel.Rows = 5

	app.render()
	want := ui.HashScreenLogical(app.screen)
	if !app.paintDiskUsageBrowserUpdate() {
		t.Fatal("expected disk-usage partial paint to succeed")
	}
	if got := ui.HashScreenLogical(app.screen); got != want {
		t.Fatal("disk-usage partial paint with terminal focus and a selections strip must pixel-match a full render")
	}
}

func TestCarouselKeyboardNavPaintDeferredUntilParentSnapshotLands(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "walnut")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 160, 30)
	app := newApp(t, screen, inner)
	app.model.Primary.CarouselMode = true
	left := app.panelByID(ui.PrimaryPanel)

	left.CarouselSideCache.ParentOK = true
	left.CarouselSideCache.ParentSourceDir = root
	if !app.carouselParentPaintPending(ui.PrimaryPanel) {
		t.Fatal("parent paint should be pending while the parent cache is stale for the current dir")
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); quit {
		t.Fatal("nav.parent quit")
	}
	if id, ok := app.keys.Global.Lookup(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)); !ok || id != keymap.ActionNavParent {
		t.Fatalf("Left lookup = %q ok=%v, want %s", id, ok, keymap.ActionNavParent)
	}
	if !app.carouselPaintDefer[ui.PrimaryPanel].active {
		t.Fatal("keyboard folder-change partial paint should defer while the carousel parent snapshot is stale")
	}
}

func TestBrowserPartialPaintMatchesFullRenderListedStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*testing.T, *App)
	}{
		{
			name: "terminal focus",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.TerminalPanel.Visible = true
				app.model.TerminalPanel.Focused = true
				app.model.TerminalPanel.Rows = 5
			},
		},
		{
			name: "selections strip",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				armSelectionsStrip(t, app)
				app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
			},
		},
		{
			name: "quick view",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.QuickViewEnabled = true
				app.model.QuickViewPanel = ui.PrimaryPanel
			},
		},
		{
			name: "swap panes",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.SwapPanes = true
			},
		},
		{
			name: "stacked split",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				orient := ui.SplitVertical
				app.paneSplitOrientationOverride = &orient
			},
		},
		{
			name: "carousel autohide",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.config.Carousel.AutohideInactivePanel = true
				app.model.Primary.CarouselMode = true
			},
		},
		{
			name: "transfer target",
			setup: func(t *testing.T, app *App) {
				t.Helper()
				app.model.DestinationTargetPrimary = true
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newBrowserPlanApp(t)
			armToastDiskUsagePaint(t, app)
			tc.setup(t, app)

			app.render()
			want := ui.HashScreenLogical(app.screen)
			if !app.paintDiskUsageBrowserUpdate() {
				t.Fatal("expected disk-usage partial paint to succeed")
			}
			if got := ui.HashScreenLogical(app.screen); got != want {
				t.Fatalf("%s: disk-usage partial paint must pixel-match a full render", tc.name)
			}
		})
	}
}

func newBrowserPlanApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "juniper"))
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	app.model.ViewMode = ui.ViewBrowser
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.ActiveSubFocus = ui.SubFocusFileList
	return app
}

func armSelectionsStrip(t *testing.T, app *App) {
	t.Helper()
	p := &app.model.Primary
	outside := filepath.Join(t.TempDir(), "harbor.txt")
	p.BulkAddSelections([]string{outside}, func(string) bool { return false })
	if p.SelectionsStripCount() == 0 {
		t.Fatal("expected selections strip to have a foreign path")
	}
}
