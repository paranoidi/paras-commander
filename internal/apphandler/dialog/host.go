package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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

	// Extract, flatten, and bookmark/SFTP dialogs: still owned by internal/app (reserved for a
	// later extraction step), reached through Host until then.
	OpenExtractDialog(p *panel.State)
	OpenFlattenDialog()
	ExecuteExtract()
	ExecuteAddBookmark()
	ExecuteSFTPPassword()

	// TryBookmarkDialogShortcut handles [dialog.bookmark] chords (delete/open-other) while the
	// bookmarks path picker (a PathPickerPurposeNavigate-purposed PathPicker) is open; the
	// bookmarks dialog itself is still owned by internal/app.
	TryBookmarkDialogShortcut(ev *tcell.EventKey) bool
	// HandlePathPickerScrollingQueryKey handles a focused edit key on the path picker's query
	// row through internal/app's shared scrollquery glue (internal/app/scrolling_query.go).
	HandlePathPickerScrollingQueryKey(ev *tcell.EventKey) bool

	// SyncFilteredListRanks / ClampFilteredListSelection / HandleFilteredListSelectionKey are
	// internal/app's shared fuzzy-list ranking/selection helpers (internal/app/filtered_list.go),
	// also used by the find, history, SFTP-connect, and preview-style-picker dialogs.
	SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range)
	ClampFilteredListSelection(selected *int, rankedLen int)
	HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool

	// Mass-rename dialog: opening and the key-handling machinery live in this package, but the
	// dialog's own preview/execution logic is still owned by internal/app (reserved for a later
	// extraction step).
	OpenMassRenameDialog(p *panel.State)
	ApplyMassRenameModeFromFocus()
	MassRenameSyncFieldLabels()
	RecomputeMassRenamePreview()
	MassRenameClampFocusAfterModeChange(prev dialog.MassRenameModeUI)
	LaunchMassRenameExternalEditor()
	ExecuteMassRename()

	// OpenDedupEmptyDirsConfirm opens the dedup-view "N directories left empty" confirmation
	// (internal/app dedup_view.go) when Delete is pressed while the dedup view's own delete
	// dialog is overlaid on it.
	OpenDedupEmptyDirsConfirm()
}
