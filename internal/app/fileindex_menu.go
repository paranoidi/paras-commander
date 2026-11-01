package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// toggleFileIndexMenu opens the fileindex menu strip (fileindex.menu), or closes it when open:
// the fileindex.* actions under letters that match their default chords ([fileindex_menu]).
// fileindex.reveal is dimmed off a fileindex:// panel (fileindex.open toggles, so it stays
// enabled), and the scan label names what fileindex.scan will scan.
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
	if a.leaderMenuOpen(ui.LeaderMenuFileIndex) {
		a.model.LeaderMenu.Legend = a.fileIndexLegend()
	}
}

// fileIndexLegendStates are the index state icons in the panel's precedence order
// (docs/fileindex.md) with their legend labels.
var fileIndexLegendStates = []struct{ state, label string }{
	{localfs.IndexStatusScanning, "scanning"},
	{localfs.IndexStatusMissing, "only in index"},
	{localfs.IndexStatusChanged, "changed"},
	{localfs.IndexStatusExcluded, "excluded"},
	{localfs.IndexStatusNew, "not in index"},
	{localfs.IndexStatusDiffers, "differs below"},
	{localfs.IndexStatusUnhashed, "not hashed yet"},
	{localfs.IndexStatusTagged, "tagged"},
	{localfs.IndexStatusUnchecked, "scan depth exceeded"},
	{localfs.IndexStatusIndexed, "indexed"},
}

// fileIndexLegend lists the index column's state icons with the theme's glyph and style, the
// same lookups paintIndexColumn uses.
func (a *App) fileIndexLegend() []ui.LeaderMenuLegendItem {
	legend := make([]ui.LeaderMenuLegendItem, 0, len(fileIndexLegendStates))
	for _, s := range fileIndexLegendStates {
		if r := a.styles.IconFileIndexState(s.state); r != 0 {
			legend = append(legend, ui.LeaderMenuLegendItem{Icon: string(r), Style: a.styles.PanelFileIndexStyle(s.state), Label: s.label})
		}
	}
	return legend
}
