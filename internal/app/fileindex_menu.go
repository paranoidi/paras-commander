package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// toggleFileIndexMenu opens the fileindex menu strip (fileindex.menu), or closes it when open:
// the fileindex.* actions under letters that match their default chords ([fileindex_menu]).
// The browse action for the side the active panel is already on is dimmed, and the scan label
// names what fileindex.scan will scan.
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
		case keymap.ActionFileIndexScan:
			items[i].Label = "Scan directory"
			if len(a.activePanel().SelectedPaths) > 0 {
				items[i].Label = "Scan selection"
			}
		}
		row++
	}
	a.openLeaderMenuDispatch(items, actions, ui.LeaderMenuFileIndex, "File Index menu", a.dispatchActionLikeKeyboardShortcut)
}
