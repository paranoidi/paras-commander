package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func TestEffectivePaneSplitOrientationUsesOverride(t *testing.T) {
	a := &App{
		config: config.Default(),
	}
	a.config.UI.Zoom.Orientation = config.PaneSplitSideBySide
	stacked := ui.SplitVertical
	a.paneSplitOrientationOverride = &stacked
	if got := a.effectivePaneSplitOrientation(); got != ui.SplitVertical {
		t.Fatalf("effective = %v, want SplitVertical", got)
	}
	a.paneSplitOrientationOverride = nil
	if got := a.effectivePaneSplitOrientation(); got != ui.SplitHorizontal {
		t.Fatalf("effective = %v, want SplitHorizontal", got)
	}
}

func TestSyncFollowToastPartsStacked(t *testing.T) {
	arrow, driver, follower := ui.SyncFollowToastParts(ui.PrimaryPanel, ui.SplitVertical)
	if arrow != "↓" || driver != "Primary" || follower != "Secondary" {
		t.Fatalf("primary stacked = %q %q %q", arrow, driver, follower)
	}
	arrow, driver, follower = ui.SyncFollowToastParts(ui.SecondaryPanel, ui.SplitVertical)
	if arrow != "↑" || driver != "Secondary" || follower != "Primary" {
		t.Fatalf("secondary stacked = %q %q %q", arrow, driver, follower)
	}
}

func TestSwapPanesMovesSecondaryToFirstSlot(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "harbor")
	right := filepath.Join(root, "meadow")
	if err := os.Mkdir(left, 0o755); err != nil {
		t.Fatalf("Mkdir left: %v", err)
	}
	if err := os.Mkdir(right, 0o755); err != nil {
		t.Fatalf("Mkdir right: %v", err)
	}
	screen := uitest.Screen(t, 100, 30)
	app := newTestApp(t, screen, Options{
		CWD:        func() (string, error) { return root, nil },
		Config:     config.Default(),
		StartPaths: []string{left, right},
	})
	applyNextInterruptEvent(t, app, screen)
	applyNextInterruptEvent(t, app, screen)

	app.model.ActivePanel = ui.SecondaryPanel
	if got := app.visualPanelScope(menu.PanelScopePrimary); got != menu.PanelScopePrimary {
		t.Fatalf("visualPanelScope before swap = %d, want Primary", got)
	}

	app.dispatch(keymap.ActionPanelSwapPanes)

	if !app.model.SwapPanes {
		t.Fatal("SwapPanes = false, want true")
	}
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("ActivePanel = %d, want Secondary", app.model.ActivePanel)
	}
	if got := app.model.Primary.PathString(); got != left {
		t.Fatalf("Primary path = %q, want %q", got, left)
	}
	if got := app.model.Secondary.PathString(); got != right {
		t.Fatalf("Secondary path = %q, want %q", got, right)
	}
	if got := app.visualPanelScope(menu.PanelScopePrimary); got != menu.PanelScopeSecondary {
		t.Fatalf("visualPanelScope after swap = %d, want Secondary", got)
	}

	lay := app.layoutForTerminalSize(100, 30)
	if lay.Secondary.X != 0 {
		t.Fatalf("Secondary.X = %d, want 0 (first slot)", lay.Secondary.X)
	}
	if lay.Primary.X <= lay.Secondary.X {
		t.Fatalf("Primary.X = %d, Secondary.X = %d, want Primary in second slot", lay.Primary.X, lay.Secondary.X)
	}

	app.dispatch(keymap.ActionPanelSwapPanes)
	if app.model.SwapPanes {
		t.Fatal("SwapPanes = true after second toggle, want false")
	}
}
