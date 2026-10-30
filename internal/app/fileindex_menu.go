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
	var items []ui.LeaderMenuItem
	var actions []string
	for _, e := range a.keys.FileIndexMenuEntries() {
		directKey := ""
		if a.config.UI.LeaderMenuShowDirectKeys && a.keys.Global != nil {
			directKey = a.keys.Global.MenuBindingLabel(e.ActionID)
		}
		items = append(items, ui.LeaderMenuItem{Key: e.Key, Label: e.Label, DirectKey: directKey})
		actions = append(actions, e.ActionID)
	}
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuFileIndex, "fileindex menu", a.dispatchActionLikeKeyboardShortcut)
}
