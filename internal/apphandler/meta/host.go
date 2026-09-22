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
	// ResortPanel re-applies the panel's current Sort (panel.State.Sort, possibly just mutated
	// through PanelByID) and refreshes cursor/scroll for it immediately. Used only for the
	// structural fallback to SortName when meta columns are cleared — never for an ordinary
	// value update, which goes through NoteMetaColumnResolved instead.
	ResortPanel(panelID int)
	// NoteMetaColumnResolved signals that panelID's active SortMeta target column just finished
	// (ColumnResolved) after a batch of results arrived. The host does not sort immediately: it
	// arms/re-arms an idle timer (mirroring the disk-usage idle sort) so a fast-finishing
	// directory doesn't reshuffle the list out from under the user's cursor or scroll.
	NoteMetaColumnResolved(panelID int)
	// IconMetaRunning returns the theme icon shown for a meta command still in flight.
	// Fetched per call (not snapshotted) because the active theme can change at runtime.
	IconMetaRunning() string
	OpenFileInExternalEditor(path string) error
	MessageLogWrapCols() int
	AppendTransientMessageLines(banner string, lines []string, urgency ui.MessageUrgency)
	ClearTransientMessage()
}
