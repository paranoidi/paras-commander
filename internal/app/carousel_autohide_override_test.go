package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestHideInactiveToggleOverridesCarouselAutohide(t *testing.T) {
	app := testAppMinimal(t)
	app.config.Carousel.AutohideInactivePanel = true
	app.toggleCarousel(ui.PrimaryPanel, false)
	if !app.carouselAutohideInactivePanel() {
		t.Fatal("autohide should hide the inactive panel in carousel")
	}
	app.toggleHideInactivePanel()
	if app.carouselAutohideInactivePanel() || app.model.HideInactivePanel {
		t.Fatal("toggle should show the inactive panel")
	}
	app.toggleHideInactivePanel()
	if !app.carouselAutohideInactivePanel() {
		t.Fatal("second toggle should hide again")
	}
	app.toggleHideInactivePanel()
	app.toggleCarousel(ui.PrimaryPanel, false)
	if app.carouselAutohideOverride {
		t.Fatal("override should clear once carousel is off on both panels")
	}
	app.toggleCarousel(ui.PrimaryPanel, false)
	if !app.carouselAutohideInactivePanel() {
		t.Fatal("autohide should apply again after carousel re-enabled")
	}
}

func TestCarouselQuickViewPreviewTakesRightSlot(t *testing.T) {
	app := testAppMinimal(t)
	app.config.Carousel.AutohideInactivePanel = true
	app.model.Secondary.CarouselMode = true
	app.model.ActivePanel = ui.SecondaryPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.SecondaryPanel
	lay := app.layoutForTerminalSize(120, 30)
	if lay.Secondary.X >= lay.Primary.X {
		t.Fatalf("Secondary.X=%d Primary.X=%d want carousel driver on the left", lay.Secondary.X, lay.Primary.X)
	}
	app.carouselAutohideOverride = true
	lay = app.layoutForTerminalSize(120, 30)
	if lay.Primary.X >= lay.Secondary.X {
		t.Fatalf("Primary.X=%d Secondary.X=%d want no swap with override", lay.Primary.X, lay.Secondary.X)
	}
}

func TestToggleHideInactiveWhileQuickViewUsesCarouselSlot(t *testing.T) {
	app := testAppMinimal(t)
	app.config.Carousel.AutohideInactivePanel = true
	app.model.Secondary.CarouselMode = true
	app.model.ActivePanel = ui.SecondaryPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.SecondaryPanel
	if !app.carouselQuickViewForcesRightPreview() {
		t.Fatal("quick view should force right preview when carousel autohide is active")
	}
	app.toggleHideInactivePanel()
	if app.model.QuickViewEnabled {
		t.Fatal("quick view should be disabled after toggle")
	}
	if app.model.QuickViewPanel != -1 {
		t.Fatalf("quick view panel should be -1, got %d", app.model.QuickViewPanel)
	}
	if app.model.HideInactivePanel {
		t.Fatal("inactive panel should be shown, not hidden")
	}
	if app.carouselAutohideInactivePanel() {
		t.Fatal("panel should be visible (autohide should not apply)")
	}
}
