package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// toggleFileIndexMenu opens the fileindex menu strip (fileindex.menu), or closes it when open:
// the fileindex.* actions under letters that match their default chords ([fileindex_menu]).
// The browse action for the side the active panel is already on is dimmed.
func (a *App) toggleFileIndexMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuFileIndex) {
		a.closeLeaderMenu()
		return
	}
	if a.model.ViewMode != ui.ViewBrowser || a.keys == nil {
		return
	}
	items, actions := a.buildLeaderMenuItems(a.keys.FileIndexMenuEntries())
	inIndex := a.activePanel().Path.Scheme() == pathloc.SchemeFileIndex
	row := 0
	for i := range items {
		if items[i].GroupTitle != "" {
			continue
		}
		switch actions[row] {
		case keymap.ActionFileIndexOpen:
			items[i].Disabled = inIndex
		case keymap.ActionFileIndexReveal:
			items[i].Disabled = !inIndex
		}
		row++
	}
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuFileIndex, "fileindex menu", a.dispatchActionLikeKeyboardShortcut)
}
