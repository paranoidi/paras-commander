package ui

import "github.com/paranoidi/paras-commander/internal/panel"

// FileListFrame returns the file-list panel rect inside a browser column after reserving
// the selections strip when applicable. This must match browser Render() split logic.
func FileListFrame(column Rect, p *panel.State, panelID, activePanel int, themePickerOpen bool, split SelectionsStripSplitParams) Rect {
	stripN := SelectionsStripLayoutItemCount(p, panelID, activePanel, themePickerOpen)
	split.StripItemCount = stripN
	return FileListFrameWithStripCount(column, split)
}

// FileListFrameWithStripCount reserves selections-strip chrome using a precomputed strip row count.
func FileListFrameWithStripCount(column Rect, split SelectionsStripSplitParams) Rect {
	file, _ := SplitPanelForSelections(column, split)
	return file
}

// FileListViewportRows returns visible file-list row count for a browser panel column.
func FileListViewportRows(column Rect, p *panel.State, panelID, activePanel int, themePickerOpen bool, split SelectionsStripSplitParams) int {
	return PanelListRows(FileListFrame(column, p, panelID, activePanel, themePickerOpen, split))
}

// browserSelectionsStripSplit builds SplitPanelForSelections params for one browser column.
func browserSelectionsStripSplit(model Model, panelID, stripN int) SelectionsStripSplitParams {
	return SelectionsStripSplitParams{
		StripItemCount:     stripN,
		MaxRows:            model.SelectionsPanelMaxRows,
		ActivePercent:      model.SelectionsPanelActivePercent,
		StripFocused:       model.ActivePanel == panelID && model.renderSubFocus() == SubFocusSelectionsStrip,
		Orientation:        model.SplitOrientation,
		MinFileContentRows: MinFileListContentRows,
	}
}
