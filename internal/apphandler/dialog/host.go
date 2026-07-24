package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// Host supplies cross-cutting app services the dialog handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost
	host.PanelNavigationHost

	ActivePanel() *panel.State
	InactivePanel() *panel.State
	InactivePanelID() int
	PanelByID(panelID int) *panel.State
	ActiveViewportRows() int
	PanelViewportRows(panelID int) int

	// ClearTransientMessage clears the status banner (e.g. when a transfer/extract dialog opens).
	ClearTransientMessage()

	// Config and Styles return the App's live config/theme (not a snapshot): both are mutable
	// at runtime from the settings and theme dialogs, so the handler must re-read them on every
	// use rather than caching a copy from construction time.
	Config() config.Config
	Styles() theme.Theme

	// OpenMessageDialog opens the generic centered message dialog (single OK button), used for
	// surfacing errors that don't fit a transient status-line message (e.g. extract-plan failures).
	OpenMessageDialog(title, message string)

	// InQuickFilterUI reports whether the active panel's quick filter is being edited, so
	// opening a bookmarks/add-bookmark dialog can cancel it first.
	InQuickFilterUI() bool

	// OpenFileInExternalEditor releases the terminal, runs the user's $EDITOR on path, and
	// repaints on return. Used by the mass-rename dialog's "External $EDITOR" mode.
	OpenFileInExternalEditor(path string) error

	// ExecuteSFTPPassword runs the SFTP password prompt's OK action; the SFTP connect/password
	// dialogs and their state remain owned by internal/app.
	ExecuteSFTPPassword()

	// HandlePathPickerScrollingQueryKey handles a focused edit key on the path picker's query
	// row through internal/app's shared scrollquery glue (internal/app/scrolling_query.go).
	HandlePathPickerScrollingQueryKey(ev *tcell.EventKey) bool

	// SyncFilteredListRanks / ClampFilteredListSelection / HandleFilteredListSelectionKey are
	// internal/app's shared fuzzy-list ranking/selection helpers (internal/app/filtered_list.go),
	// also used by the find, history, SFTP-connect, and preview-style-picker dialogs.
	SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range)
	ClampFilteredListSelection(selected *int, rankedLen int)
	HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool
}
