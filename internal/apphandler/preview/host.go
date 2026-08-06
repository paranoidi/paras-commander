package preview

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// Host supplies cross-cutting app services the preview handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost
	host.ShellHost

	ActivePanel() *panel.State
	PanelByID(panelID int) *panel.State
	InactivePanelID() int
	ActiveViewportRows() int
	PanelViewportRows(panelID int) int
	SelectionsStripViewportRows(panelID int) int
	InQuickFilterUI() bool
	SwitchPanel()
	SyncFollowTargetPath(driver *panel.State) (string, bool)
	PanelSyncFollowHeldListNav(resolvedAction string, event *tcell.EventKey) bool
	ArmPanelSyncFollowNavCoalesceAfterListNav()
	ClearPanelSyncFollowNavCoalesce()
	ArmCursorNameHintNavCoalesceAfterListNav()
	PathVolumeContendsWithActiveJob(path string) bool
	QuickViewGitStatusScheduler() panel.GitStatusScheduler
	EffectivePaneSplitOrientation() ui.SplitOrientation
	PanelPaneSplit(width int, filePreviewOpen bool) ui.PanelPaneSplit
	LayoutForTerminalSizePreview(width, height int, filePreviewOpen bool) ui.Layout
	TerminalLayoutRows() int
	BrowserMenuDefinitions() []menu.Definition
	CarouselAutohideInactivePanel() bool
	EditActiveFile()
	EditFullscreenPreviewFile()
	OpenDeleteDialogForPreviewedFile()
	OpenPreviewLeaderMenu()
	HandleFileDialogFieldKey(ev *tcell.EventKey, f *dialog.FileDialogField, afterEdit func()) bool
	PersistPartial(patch map[string]interface{}) error

	// Config and Styles return the App's live config/theme (not a snapshot): both are mutable at
	// runtime from the settings and theme dialogs, so the handler must re-read them on every use
	// rather than caching a copy from construction time.
	Config() config.Config
	Styles() theme.Theme
	// SetPreviewStyle mutates the live config's preview.style in place (no persist, no message);
	// used for the style picker's live-preview-while-navigating and Esc-revert behavior.
	SetPreviewStyle(style string)
	// ApplyPreviewStyle validates, persists, and reports the final Chroma style selection.
	ApplyPreviewStyle(name string) bool

	// SyncFilteredListRanks, ClampFilteredListSelection, and HandleFilteredListSelectionKey
	// forward to the shared filtered-list ranking helpers in internal/app (also used by the
	// history, path-picker, and SFTP-connect dialogs), keeping ranking logic single-sourced.
	SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range)
	ClampFilteredListSelection(selected *int, rankedLen int)
	HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool
}
