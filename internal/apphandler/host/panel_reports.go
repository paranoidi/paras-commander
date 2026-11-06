package host

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// StatusHost shows transient status messages.
type StatusHost interface {
	SetTransientMessage(text string, urgency ui.MessageUrgency)
}

// PanelShowHost is what ShowInPanel needs from a handler's host.
type PanelShowHost interface {
	MessageHost
	PanelNavigationHost
	PanelByID(panelID int) *panel.State
	PanelViewportRows(panelID int) int
}

// ShowInPanel navigates panelID to the host path dir with name highlighted, as a fileindex:// path
// when indexPanel, else a local one (falling back to the index's view when the real directory is
// gone). Errors are shown under errTitle; it reports success.
func ShowInPanel(h PanelShowHost, panelID int, dir, name string, indexPanel bool, errTitle string) bool {
	loc, err := pathloc.ForPanel(dir, indexPanel)
	if err == nil {
		err = h.NavigatePanelToPath(panelID, loc.String(), name)
	}
	if err != nil && !indexPanel {
		// ponytail: the real directory is gone, so show it as the index remembers it
		if loc, ferr := pathloc.FileIndex(dir); ferr == nil {
			err = h.NavigatePanelToPath(panelID, loc.String(), name)
		}
	}
	if err != nil {
		h.SetErrorMessage(errTitle, err)
		return false
	}
	h.PanelByID(panelID).EnsureCursorVisible(h.PanelViewportRows(panelID))
	return true
}

// ReportOpenedInPanel says name was opened in panelID.
func ReportOpenedInPanel(h StatusHost, name string, panelID int) {
	which := "primary"
	if panelID == ui.SecondaryPanel {
		which = "secondary"
	}
	h.SetTransientMessage(fmt.Sprintf("Opened %s in %s panel", name, which), ui.MessageUrgencyInfo)
}

// ReportSelectionAdded says how adding marked results to a panel's selection went.
func ReportSelectionAdded(h StatusHost, added int, conflicts bool) {
	switch {
	case added == 0:
		h.SetTransientMessage("No new selections", ui.MessageUrgencyInfo)
	case conflicts:
		h.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	default:
		h.SetTransientMessage(fmt.Sprintf("Added %d to selection", added), ui.MessageUrgencyInfo)
	}
}
