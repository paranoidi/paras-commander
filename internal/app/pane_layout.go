package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// splitOrientationFromConfig maps [ui].pane_split_orientation to geom orientation.
func splitOrientationFromConfig(value string) ui.SplitOrientation {
	if strings.EqualFold(strings.TrimSpace(value), config.PaneSplitStacked) {
		return ui.SplitVertical
	}
	return ui.SplitHorizontal
}

func (a *App) savedPaneSplitOrientation() ui.SplitOrientation {
	return splitOrientationFromConfig(a.config.UI.PaneSplitOrientation)
}

// effectivePaneSplitOrientation returns saved preference plus optional session-only override.
func (a *App) effectivePaneSplitOrientation() ui.SplitOrientation {
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

// togglePaneSplitOrientation flips the visible twin-pane layout for this session only.
func (a *App) togglePaneSplitOrientation() {
	current := a.effectivePaneSplitOrientation()
	var next ui.SplitOrientation
	if current == ui.SplitVertical {
		next = ui.SplitHorizontal
	} else {
		next = ui.SplitVertical
	}
	saved := a.savedPaneSplitOrientation()
	if next == saved {
		a.paneSplitOrientationOverride = nil
	} else {
		v := next
		a.paneSplitOrientationOverride = &v
	}
	a.setTransientMessage("Split: "+paneSplitOrientationLabel(next), ui.MessageUrgencyInfo)
}
