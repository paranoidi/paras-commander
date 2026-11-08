package meta

import (
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
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
	// PanelMetaFit reports the meta columns panelID draws right now; ok is false while its file
	// list is not on screen.
	PanelMetaFit(panelID int) (fit ui.PanelMetaFit, ok bool)
	// IconMetaRunning returns the theme icon shown for a meta command still in flight.
	// Fetched per call (not snapshotted) because the active theme can change at runtime.
	IconMetaRunning() string
	// DialogMarkerWidth is the theme's checkbox marker width (dlg.MarkerWidth), cached in the
	// picker state at open so its two-column sections line up.
	DialogMarkerWidth() int
	OpenFileInExternalEditor(path string) error
	// SaveDefaultProfile writes name to [panels].default_column_profile in config.toml and, on
	// success, updates the app in-memory config. It fails when config cannot be persisted.
	SaveDefaultProfile(name string) error
	// ReloadPanelListing re-reads the panel's directory: a change of columns needs a fact the
	// current listing did not fetch.
	ReloadPanelListing(panelID int)
	// SyncFilteredListRanks and ClampFilteredListSelection rank the profile pickers like every
	// other fuzzy-filtered list.
	SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range)
	ClampFilteredListSelection(selected *int, rankedLen int)
	// PickerSearchIcon is the theme's search icon, cached in the dialog state at open.
	PickerSearchIcon() string
	MessageLogWrapCols() int
	AppendTransientMessageLines(banner string, lines []string, urgency ui.MessageUrgency)
	ClearTransientMessage()
}
