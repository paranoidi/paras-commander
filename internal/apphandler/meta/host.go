package meta

import (
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Host supplies cross-cutting app services the meta handler cannot import from internal/app.
type Host interface {
	host.MessageHost

	PanelByID(panelID int) *panel.State
	// SymbolMetaRunning returns the theme glyph shown for a meta command still in flight.
	// Fetched per call (not snapshotted) because the active theme can change at runtime.
	SymbolMetaRunning() string
	OpenFileInExternalEditor(path string) error
	MessageLogWrapCols() int
	AppendTransientMessageLines(banner string, lines []string, urgency ui.MessageUrgency)
	ClearTransientMessage()
	// Render repaints the screen. Used after synchronous main-thread work (dialog actions,
	// editor round-trips) that changes model state outside the normal input-handling render.
	Render()
}
