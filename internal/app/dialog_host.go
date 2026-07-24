package app

import (
	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// dialogHost implements apphandler/dialog.Host for *App.
type dialogHost struct {
	appShellHost
}

func (h dialogHost) NavigatePanelToPath(panelID int, path, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h dialogHost) InactivePanel() *panel.State { return h.app.inactivePanel() }

func (h dialogHost) InactivePanelID() int { return h.app.inactivePanelID() }

func (h dialogHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h dialogHost) ActiveViewportRows() int { return h.app.activeViewportRows() }

func (h dialogHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h dialogHost) ClearTransientMessage() { h.app.clearTransientMessage() }

func (h dialogHost) Config() config.Config { return h.app.config }

func (h dialogHost) Styles() theme.Theme { return h.app.styles }

func (h dialogHost) OpenExtractDialog(p *panel.State) { h.app.openExtractDialog(p) }

func (h dialogHost) OpenFlattenDialog() { h.app.openFlattenDialog() }

func (h dialogHost) ExecuteExtract() { h.app.executeExtract() }

func (h dialogHost) ExecuteAddBookmark() { h.app.executeAddBookmark() }

func (h dialogHost) ExecuteSFTPPassword() { h.app.executeSFTPPassword() }

func (h dialogHost) TryBookmarkDialogShortcut(ev *tcell.EventKey) bool {
	return h.app.tryBookmarkDialogShortcut(ev)
}

func (h dialogHost) HandlePathPickerScrollingQueryKey(ev *tcell.EventKey) bool {
	return h.app.handleScrollingQueryKey(ev, true, h.app.pathPickerScrollingQuery())
}

func (h dialogHost) SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
	return syncFilteredListRanks(lines, query, matchRangeSlots, caseInsensitive)
}

func (h dialogHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	clampFilteredListSelection(selected, rankedLen)
}

func (h dialogHost) HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	return handleFilteredListSelectionKey(ev, focus, selected, rankedLen, listRows, ensureScroll)
}

func (h dialogHost) OpenMassRenameDialog(p *panel.State) { h.app.openMassRenameDialog(p) }

func (h dialogHost) ApplyMassRenameModeFromFocus() { h.app.applyMassRenameModeFromFocus() }

func (h dialogHost) MassRenameSyncFieldLabels() { h.app.massRenameSyncFieldLabels() }

func (h dialogHost) RecomputeMassRenamePreview() { h.app.recomputeMassRenamePreview() }

func (h dialogHost) MassRenameClampFocusAfterModeChange(prev dialog.MassRenameModeUI) {
	h.app.massRenameClampFocusAfterModeChange(prev)
}

func (h dialogHost) LaunchMassRenameExternalEditor() { h.app.launchMassRenameExternalEditor() }

func (h dialogHost) ExecuteMassRename() { h.app.executeMassRename() }

func (h dialogHost) OpenDedupEmptyDirsConfirm() { h.app.openDedupEmptyDirsConfirm() }

var _ dialogctrl.Host = dialogHost{}
