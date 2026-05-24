package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestToggleCarouselFlipsActivePanel(t *testing.T) {
	app := testAppMinimal(t)
	if app.model.Left.CarouselMode {
		t.Fatal("carousel should start off")
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if !app.model.Left.CarouselMode {
		t.Fatal("carousel should be on after toggle")
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if app.model.Left.CarouselMode {
		t.Fatal("carousel should be off after second toggle")
	}
}

func TestCycleListingFormatBlockedInCarousel(t *testing.T) {
	app := testAppMinimal(t)
	app.model.Left.CarouselMode = true
	before := app.model.Left.ListFormat
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelCycleListingFormat)
	if app.model.Left.ListFormat != before {
		t.Fatal("listing format should not change in carousel mode")
	}
}

func TestCarouselTogglePerPanelFromMenuScope(t *testing.T) {
	app := testAppMinimal(t)
	app.model.ActivePanel = ui.RightPanel
	app.activateScopedPanelMenu(ui.RightPanel, menu.Item{
		Action: keymap.ActionPanelToggleCarousel,
		Label:  "Carousel view",
	})
	if !app.model.Right.CarouselMode {
		t.Fatal("right panel carousel should be on")
	}
	if app.model.Left.CarouselMode {
		t.Fatal("left panel carousel should stay off")
	}
}

func TestCarouselForcesPanelZoomRegardlessOfConfigAndWidth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(160, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = false
	cfg.UI.ZoomActivePanelDisabledAboveWidth = 155
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.model.Left.CarouselMode = true
	lay := app.layoutForTerminalSize(160, 30)
	if lay.Left.Width != 112 || lay.Right.Width != 48 {
		t.Fatalf("carousel on active panel Left=%d Right=%d want 70/30 zoom at width 160", lay.Left.Width, lay.Right.Width)
	}

	app.model.ActivePanel = ui.RightPanel
	app.model.Right.CarouselMode = true
	layRight := app.layoutForTerminalSize(160, 30)
	if layRight.Left.Width != 48 || layRight.Right.Width != 112 {
		t.Fatalf("carousel on right panel Left=%d Right=%d want 30/70 zoom", layRight.Left.Width, layRight.Right.Width)
	}

	app.model.Right.CarouselMode = false
	app.model.Left.CarouselMode = true
	layInactive := app.layoutForTerminalSize(160, 30)
	if layInactive.Left.Width != 80 || layInactive.Right.Width != 80 {
		t.Fatalf("carousel only on inactive panel Left=%d Right=%d want even split", layInactive.Left.Width, layInactive.Right.Width)
	}
}

func TestToggleZoomIgnoredInCarouselView(t *testing.T) {
	app := testAppMinimal(t)
	app.model.Left.CarouselMode = true
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleZoomActivePanel)
	if !strings.Contains(app.model.Message, "carousel") {
		t.Fatalf("message = %q, want carousel zoom hint", app.model.Message)
	}
}
