package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestPanelForFileListRenderHidesQuickViewOverlayCursor(t *testing.T) {
	model := Model{
		ActivePanel:                PrimaryPanel,
		QuickViewEnabled:           true,
		QuickViewPanel:             PrimaryPanel,
		QuickViewDirOverlayActive:  true,
		QuickViewDirOverlayPanelID: SecondaryPanel,
		QuickViewDirOverlay:        panel.State{Cursor: 1},
	}
	if got := model.PanelForFileListRender(SecondaryPanel).Cursor; got != -1 {
		t.Fatalf("overlay render cursor = %d, want -1", got)
	}
	if model.QuickViewDirOverlay.Cursor != 1 {
		t.Fatalf("model overlay cursor mutated to %d", model.QuickViewDirOverlay.Cursor)
	}
}
