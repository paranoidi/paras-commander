package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

// swapApp builds an app with two distinct panel directories and carousel autohide on.
func swapApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	left := filepath.Join(root, "harbor")
	right := filepath.Join(root, "meadow")
	for _, d := range []string{left, right} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
	}
	screen := uitest.Screen(t, 120, 30)
	opts := testOptions(root)
	opts.Config.Carousel.AutohideInactivePanel = true
	opts.Config.Shell.Persistent = false
	opts.StartPaths = []string{left, right}
	app := newTestApp(t, screen, opts)
	applyNextInterruptEvent(t, app, screen)
	applyNextInterruptEvent(t, app, screen)
	return app
}

// forceQuickViewOrder latches quick view on driver while it is in carousel mode, with autohide on
// and no override: the state in which the carousel forces the driver into the first slot.
func forceQuickViewOrder(app *App, driver int, preferSwap bool) {
	app.model.SwapPanes = preferSwap
	app.panelByID(driver).CarouselMode = true
	app.model.ActivePanel = driver
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = driver
}

// drawnFirst reads the panel drawn first from the real layout, never from a swap flag.
func drawnFirst(app *App) int {
	lay := app.layoutForTerminalSize(120, 30)
	if lay.Secondary.X < lay.Primary.X {
		return ui.SecondaryPanel
	}
	return ui.PrimaryPanel
}

func other(id int) int { return ui.PrimaryPanel + ui.SecondaryPanel - id }

// assertSlotsMatch checks every consumer of "the visual slot" against the panel drawn first.
func assertSlotsMatch(t *testing.T, app *App, first int) {
	t.Helper()
	if got := drawnFirst(app); got != first {
		t.Fatalf("layout draws panel %d first, want %d", got, first)
	}
	if got := app.visualPanelScope(menu.PanelScopePrimary); got != first {
		t.Errorf("visualPanelScope(Left) = %d, want %d (drawn first)", got, first)
	}
	if got := app.visualPanelScope(menu.PanelScopeSecondary); got != other(first) {
		t.Errorf("visualPanelScope(Right) = %d, want %d (drawn second)", got, other(first))
	}
	if got := app.openInPanelID(keymap.ActionOpenInPrimary); got != first {
		t.Errorf("openInPanelID(open-primary) = %d, want %d", got, first)
	}
	if got := app.openInPanelID(keymap.ActionOpenInSecondary); got != other(first) {
		t.Errorf("openInPanelID(open-secondary) = %d, want %d", got, other(first))
	}
	app.openComparePanels()
	defer app.compareCtrl.Close()
	if app.model.ViewMode != ui.ViewCompare {
		t.Fatalf("ViewMode = %v, want compare", app.model.ViewMode)
	}
	if got := app.model.CompareView.PanelID(false); got != first {
		t.Errorf("compare left column panel = %d, want %d", got, first)
	}
}

// While a carousel quick view forces its own order, the driver is drawn first whatever the
// preference says, and the slot consumers follow the drawing.
func TestForcedQuickViewOrderSlotConsumersFollowDrawing(t *testing.T) {
	for _, driver := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		for _, prefer := range []bool{false, true} {
			app := swapApp(t)
			forceQuickViewOrder(app, driver, prefer)
			if !app.model.QuickViewForcesSlotOrder() {
				t.Fatalf("driver=%d prefer=%v: order not forced", driver, prefer)
			}
			assertSlotsMatch(t, app, driver)
			if app.model.SwapPanes != prefer {
				t.Errorf("driver=%d: preference changed to %v", driver, app.model.SwapPanes)
			}
		}
	}
}

// Without a forced order every consumer equals the preference, in both preference states.
func TestUnforcedOrderSlotConsumersFollowPreference(t *testing.T) {
	for _, prefer := range []bool{false, true} {
		first := ui.PrimaryPanel
		if prefer {
			first = ui.SecondaryPanel
		}
		for name, setup := range map[string]func(*App){
			"plain": func(*App) {},
			"quick view without carousel": func(a *App) {
				a.model.QuickViewEnabled = true
				a.model.QuickViewPanel = ui.SecondaryPanel
			},
			"carousel without quick view": func(a *App) { a.model.Secondary.CarouselMode = true },
			"override set": func(a *App) {
				forceQuickViewOrder(a, ui.SecondaryPanel, prefer)
				a.setCarouselAutohideOverride(true)
			},
			"autohide off": func(a *App) {
				forceQuickViewOrder(a, ui.SecondaryPanel, prefer)
				a.config.Carousel.AutohideInactivePanel = false
				a.syncCarouselQuickViewOrdering()
			},
		} {
			app := swapApp(t)
			app.model.SwapPanes = prefer
			setup(app)
			if app.model.QuickViewForcesSlotOrder() {
				t.Fatalf("%s prefer=%v: order forced", name, prefer)
			}
			t.Run(name, func(t *testing.T) { assertSlotsMatch(t, app, first) })
		}
	}
}

// Forced-order render output is what it was when the snapshot overwrote SwapPanes with the
// effective value: the same cells, the same first panel.
func TestForcedQuickViewOrderRenderUnchanged(t *testing.T) {
	for _, driver := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		for _, prefer := range []bool{false, true} {
			app := swapApp(t)
			forceQuickViewOrder(app, driver, prefer)
			snap := app.snapshotModelForBrowserPaint()

			legacy := snap
			legacy.SwapPanes = snap.EffectiveSwapPanes()
			legacy.CarouselQuickViewOrdering = false

			render := func(m ui.Model) []tcell.SimCell {
				screen := uitest.Screen(t, 120, 30)
				ui.Render(screen, m, app.styles)
				cells, _, _ := screen.GetContents()
				return append([]tcell.SimCell(nil), cells...)
			}
			if !reflect.DeepEqual(render(snap), render(legacy)) {
				t.Errorf("driver=%d prefer=%v: render differs from the effective-swap snapshot", driver, prefer)
			}
			if got := drawnFirst(app); got != driver {
				t.Errorf("driver=%d prefer=%v: first drawn = %d", driver, prefer, got)
			}
		}
	}
}

// CarouselQuickViewOrdering follows the hide-inactive-panel override and config changes.
func TestCarouselQuickViewOrderingFlagFollowsOverrideAndConfig(t *testing.T) {
	app := swapApp(t)
	if !app.model.CarouselQuickViewOrdering {
		t.Fatal("flag off at startup with autohide on")
	}
	app.toggleCarousel(ui.SecondaryPanel, false)
	app.model.QuickViewEnabled, app.model.QuickViewPanel = true, ui.SecondaryPanel
	app.toggleHideInactivePanel() // forced quick view: gives the slot back and sets the override
	if app.model.CarouselQuickViewOrdering {
		t.Fatal("flag still on after the override was set")
	}
	app.toggleCarousel(ui.SecondaryPanel, false) // carousel off on both panels clears the override
	if !app.model.CarouselQuickViewOrdering {
		t.Fatal("flag off after the override cleared")
	}

	dir := t.TempDir()
	app.paths = config.Paths{ConfigDir: dir, ConfigFile: filepath.Join(dir, "config.toml")}
	app.runExternalEditor = func(_ context.Context, path string, _ bool) error {
		return os.WriteFile(path, []byte("[carousel]\nautohide_inactive_panel = false\n"), 0o644)
	}
	app.openConfigTOMLInEditor(app.paths.ConfigFile)
	if app.config.Carousel.AutohideInactivePanel || app.model.CarouselQuickViewOrdering {
		t.Fatalf("after reload autohide=%v flag=%v, want both off", app.config.Carousel.AutohideInactivePanel, app.model.CarouselQuickViewOrdering)
	}
	app.resetConfigToDefaults()
	if !app.config.Carousel.AutohideInactivePanel || !app.model.CarouselQuickViewOrdering {
		t.Fatalf("after reset autohide=%v flag=%v, want both on", app.config.Carousel.AutohideInactivePanel, app.model.CarouselQuickViewOrdering)
	}
}
