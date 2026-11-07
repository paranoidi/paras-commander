package app

import (
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// toggleFileIndexMenu opens the fileindex menu strip (fileindex.menu), or closes it when open:
// the fileindex.* actions under letters that match their default chords ([fileindex_menu]).
// fileindex.reveal is dimmed on a plain panel, where it has nothing to leave (fileindex.browse
// toggles, so it stays enabled), and the scan labels name what fileindex.scan and
// fileindex.scan-fast will scan.
func (a *App) toggleFileIndexMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuFileIndex) {
		a.closeLeaderMenu()
		return
	}
	if a.model.ViewMode != ui.ViewBrowser || a.keys == nil {
		return
	}
	items, actions := a.buildLeaderMenuItems(a.keys.FileIndexMenuEntries())
	p := a.activePanel()
	inIndex := p.WithIndex || p.Path.Scheme() == pathloc.SchemeFileIndex
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
		case keymap.ActionFileIndexScanFast:
			items[i].Label = "Fast scan directory"
			if len(a.activePanel().SelectedPaths) > 0 {
				items[i].Label = "Fast scan selection"
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
	{localfs.IndexStatusUnchecked, "browser scan depth exceeded"},
	{localfs.IndexStatusIndexed, "indexed"},
}

// fileIndexLegend lists the index column's state icons with the theme's glyph and style, the
// same lookups paintIndexColumn uses.
func (a *App) fileIndexLegend() []primitive.LegendItem {
	legend := make([]primitive.LegendItem, 0, len(fileIndexLegendStates))
	for _, s := range fileIndexLegendStates {
		if r := a.styles.IconFileIndexState(s.state); r != 0 {
			legend = append(legend, primitive.LegendItem{Icon: string(r), Style: a.styles.PanelFileIndexStyle(s.state), Label: s.label})
		}
	}
	return legend
}
