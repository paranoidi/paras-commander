package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
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
