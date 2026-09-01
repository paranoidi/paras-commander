package pin

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
)

// Host supplies cross-cutting app services the pin handler cannot import from internal/app.
type Host interface {
	host.LayoutHost          // LayoutForTerminalSize: dialog list-row sizing
	host.MessageHost         // SetTransientMessage / SetErrorMessage
	host.PanelNavigationHost // NavigatePanelToPath: OpenSelected cd's the target panel

	// ActivePanel resolves the active panel's cursor entry for ToggleActivePanelCursor.
	ActivePanel() *panel.State
	// PanelByID and PanelViewportRows re-fetch the just-navigated panel and its viewport
	// rows so OpenSelected can call EnsureCursorVisible after navigating.
	PanelByID(panelID int) *panel.State
	PanelViewportRows(panelID int) int
	// InQuickFilterUI reports whether the active panel's quick/strip filter is mid-edit, so
	// OpenDialog can cancel it before showing the dialog over the panel.
	InQuickFilterUI() bool
	// CancelActiveQuickFilter cancels whichever filter (plain or selections-strip) currently
	// has focus, mirroring internal/app's strip-aware cancelActiveQuickFilter exactly.
	CancelActiveQuickFilter()
	// Config returns the App's live config (not a snapshot): Filter.CaseInsensitive is
	// mutable at runtime from the settings dialog, so SyncDialogRanks must re-read it on
	// every call rather than caching a copy from construction time.
	Config() config.Config

	// SyncFilteredListRanks / ClampFilteredListSelection / HandleFilteredListSelectionKey
	// forward to internal/app's shared fuzzy-list ranking/selection helpers
	// (filtered_list.go), also used by find/history/SFTP-connect/preview-style-picker/dialog.
	SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range)
	ClampFilteredListSelection(selected *int, rankedLen int)
	HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool
}
