package app

import "github.com/paranoidi/paras-commander/internal/ui"

// toggleFileIndexMenu opens the fileindex menu strip (fileindex.menu), or closes it when open:
// the fileindex.* actions under letters that match their default chords ([fileindex_menu]).
func (a *App) toggleFileIndexMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuFileIndex) {
		a.closeLeaderMenu()
		return
	}
	if a.model.ViewMode != ui.ViewBrowser || a.keys == nil {
		return
	}
	items, actions := a.buildLeaderMenuItems(a.keys.FileIndexMenuEntries())
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuFileIndex, "fileindex menu", a.dispatchActionLikeKeyboardShortcut)
}
