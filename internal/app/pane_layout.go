package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// splitOrientationFromConfig maps [ui.zoom].orientation to geom orientation.
func splitOrientationFromConfig(value string) ui.SplitOrientation {
	if strings.EqualFold(strings.TrimSpace(value), config.PaneSplitStacked) {
		return ui.SplitVertical
	}
	return ui.SplitHorizontal
}

func (a *App) savedPaneSplitOrientation() ui.SplitOrientation {
	return splitOrientationFromConfig(a.config.UI.Zoom.Orientation)
}

// viewSplitOrientation is the shown view's own orientation ([ui].other_copies_orientation), nil when
// it uses the file panels'.
func (a *App) viewSplitOrientation() *ui.SplitOrientation {
	if a.model.ViewMode == ui.ViewCopies {
		return a.model.CopiesView.Split
	}
	return nil
}

// effectivePaneSplitOrientation is the shown view's own orientation, else the saved preference plus
// optional session-only override.
func (a *App) effectivePaneSplitOrientation() ui.SplitOrientation {
	if own := a.viewSplitOrientation(); own != nil {
		return *own
	}
	if a.paneSplitOrientationOverride != nil {
		return *a.paneSplitOrientationOverride
	}
	return a.savedPaneSplitOrientation()
}

func paneSplitOrientationLabel(o ui.SplitOrientation) string {
	if o == ui.SplitVertical {
		return "stacked"
	}
	return "side by side"
}

// togglePaneSplitOrientation flips the visible twin-pane layout for this session only, or just the
// shown view's when it has its own.
func (a *App) togglePaneSplitOrientation() {
	current := a.effectivePaneSplitOrientation()
	var next ui.SplitOrientation
	if current == ui.SplitVertical {
		next = ui.SplitHorizontal
	} else {
		next = ui.SplitVertical
	}
	switch {
	case a.viewSplitOrientation() != nil:
		a.model.CopiesView.Split = &next
	case next == a.savedPaneSplitOrientation():
		a.paneSplitOrientationOverride = nil
	default:
		a.paneSplitOrientationOverride = &next
	}
	a.setTransientMessage("Split: "+paneSplitOrientationLabel(next), ui.MessageUrgencyInfo)
}

func (a *App) toggleSwapPanes() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	a.model.SwapPanes = !a.model.SwapPanes
}
