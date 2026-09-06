package app

import (
	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func (h dialogHost) PathVolumeContendsWithActiveJob(path string) bool {
	return h.app.pathVolumeContendsWithActiveJob(path)
}

func (h dialogHost) FilterJobContendedPaths(paths []string) []string {
	return h.app.filterJobContendedPaths(paths)
}

func (h dialogHost) ClearTransientMessage() { h.app.clearTransientMessage() }

func (h dialogHost) Config() config.Config { return h.app.config }

func (h dialogHost) Styles() theme.Theme { return h.app.styles }

func (h dialogHost) ExecuteSFTPPassword() { h.app.executeSFTPPassword() }

func (h dialogHost) OpenMessageDialog(title, message string) { h.app.openMessageDialog(title, message) }

func (h dialogHost) InQuickFilterUI() bool { return h.app.inQuickFilterUI() }

func (h dialogHost) OpenFileInExternalEditor(path string) error {
	return h.app.openFileInExternalEditor(path)
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

var _ dialogctrl.Host = dialogHost{}
