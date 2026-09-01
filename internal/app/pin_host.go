package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
)

// pinHost implements apphandler/pin.Host for *App.
type pinHost struct {
	appShellHost
}

func (h pinHost) PanelByID(panelID int) *panel.State { return h.app.panelByID(panelID) }

func (h pinHost) PanelViewportRows(panelID int) int { return h.app.panelViewportRows(panelID) }

func (h pinHost) NavigatePanelToPath(panelID int, path, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h pinHost) InQuickFilterUI() bool { return h.app.inQuickFilterUI() }

func (h pinHost) CancelActiveQuickFilter() { h.app.cancelActiveQuickFilter() }

func (h pinHost) Config() config.Config { return h.app.config }

func (h pinHost) SyncFilteredListRanks(lines []string, query string, matchRangeSlots int, caseInsensitive bool) (ranked []int, matchRanges [][]search.Range) {
	return syncFilteredListRanks(lines, query, matchRangeSlots, caseInsensitive)
}

func (h pinHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	clampFilteredListSelection(selected, rankedLen)
}

func (h pinHost) HandleFilteredListSelectionKey(ev *tcell.EventKey, focus int, selected *int, rankedLen int, listRows func() int, ensureScroll func()) bool {
	return handleFilteredListSelectionKey(ev, focus, selected, rankedLen, listRows, ensureScroll)
}
