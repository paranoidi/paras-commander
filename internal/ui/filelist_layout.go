package ui

import "github.com/paranoidi/paras-commander/internal/panel"

// FileListFrame returns the file-list panel rect inside a browser column after reserving
// the selections strip when applicable. This must match browser Render() split logic.
func FileListFrame(column Rect, p *panel.State, panelID, activePanel int, themePickerOpen bool, selectionsPanelMaxRows int) Rect {
	stripN := SelectionsStripLayoutItemCount(p, panelID, activePanel, themePickerOpen)
	file, _ := SplitPanelColumn(column, stripN, selectionsPanelMaxRows, MinFileListContentRows)
	return file
}

// FileListViewportRows returns visible file-list row count for a browser panel column.
func FileListViewportRows(column Rect, p *panel.State, panelID, activePanel int, themePickerOpen bool, selectionsPanelMaxRows int) int {
	return PanelListRows(FileListFrame(column, p, panelID, activePanel, themePickerOpen, selectionsPanelMaxRows))
}
