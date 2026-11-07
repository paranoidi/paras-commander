package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// testAppZoomed returns a test app whose terminal is width cells wide, with a 70/30 zoom always on.
func testAppZoomed(t *testing.T, width int) *App {
	t.Helper()
	app := testAppMinimal(t)
	sim, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("test app screen is not a simulation screen")
	}
	sim.SetSize(width, 30)
	app.config.UI.Zoom.ActivePanel = true
	app.config.UI.Zoom.DisabledAboveNameWidth = 0
	app.config.UI.Zoom.ActivePercent = 70
	app.config.UI.Zoom.InactivePercent = 30
	app.model.UseNerdfontIcons = false
	app.model.ViewMode = ui.ViewBrowser
	return app
}

func TestNarrowAutohideInactivePanel(t *testing.T) {
	tests := []struct {
		name  string
		width int
		setup func(*App)
		want  bool
	}{
		// A 30% share of 40 cells is 12: 10 of row text, exactly the floor.
		{"legible at the floor", 40, nil, false},
		{"wide", 200, nil, false},
		{"narrow share hides", 40, func(a *App) {
			a.config.UI.Zoom.ActivePercent, a.config.UI.Zoom.InactivePercent = 75, 25
		}, true},
		{"sync follow keeps both", 40, func(a *App) {
			a.config.UI.Zoom.ActivePercent, a.config.UI.Zoom.InactivePercent = 75, 25
			a.model.SyncFollowEnabled = true
		}, false},
		{"quick view keeps both", 40, func(a *App) {
			a.config.UI.Zoom.ActivePercent, a.config.UI.Zoom.InactivePercent = 75, 25
			a.model.QuickViewEnabled = true
		}, false},
		{"stacked keeps both", 40, func(a *App) {
			a.config.UI.Zoom.ActivePercent, a.config.UI.Zoom.InactivePercent = 75, 25
			orientation := ui.SplitVertical
			a.paneSplitOrientationOverride = &orientation
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testAppZoomed(t, tt.width)
			if tt.setup != nil {
				tt.setup(app)
			}
			if got := app.narrowAutohideInactivePanel(); got != tt.want {
				t.Fatalf("narrowAutohideInactivePanel() = %v, want %v", got, tt.want)
			}
			if got := app.panelPaneSplit(tt.width, false).HideInactivePanel; got != tt.want {
				t.Fatalf("panelPaneSplit hide = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNarrowAutohideFollowsTab(t *testing.T) {
	app := testAppZoomed(t, 40)
	app.config.UI.Zoom.ActivePercent, app.config.UI.Zoom.InactivePercent = 75, 25
	if !app.narrowAutohideInactivePanel() {
		t.Fatal("inactive panel should be hidden")
	}
	app.switchPanelSwap()
	if app.model.ActivePanel != ui.SecondaryPanel || !app.narrowAutohideInactivePanel() {
		t.Fatalf("after Tab: active = %d, want the other panel hidden too", app.model.ActivePanel)
	}
}

func TestToggleHideInactivePanelWhileNarrowHidden(t *testing.T) {
	app := testAppZoomed(t, 40)
	app.config.UI.Zoom.ActivePercent, app.config.UI.Zoom.InactivePercent = 75, 25
	app.toggleHideInactivePanel()
	if app.model.HideInactivePanel {
		t.Fatal("toggle should leave the saved flag alone while the panel is too narrow")
	}
	if !strings.Contains(app.model.Message, "too narrow") {
		t.Fatalf("message = %q", app.model.Message)
	}
}
