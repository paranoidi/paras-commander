package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
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
