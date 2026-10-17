package ui

import "github.com/paranoidi/paras-commander/internal/panel"

// FileListFrame returns the file-list panel rect inside a browser column after reserving
// the selected strip when applicable. This must match browser Render() split logic.
func FileListFrame(column Rect, p *panel.State, panelID, activePanel int, themePickerOpen bool, split SelectedStripSplitParams) Rect {
	stripN := SelectedStripLayoutItemCount(p, panelID, activePanel, themePickerOpen)
	split.StripItemCount = stripN
	return FileListFrameWithStripCount(column, split)
}

// FileListFrameWithStripCount reserves selected-strip chrome using a precomputed strip row count.
func FileListFrameWithStripCount(column Rect, split SelectedStripSplitParams) Rect {
	file, _ := SplitPanelForSelected(column, split)
	return file
}

// FileListViewportRows returns visible file-list row count for a browser panel column.
func FileListViewportRows(column Rect, p *panel.State, panelID, activePanel int, themePickerOpen bool, split SelectedStripSplitParams) int {
	return PanelListRows(FileListFrame(column, p, panelID, activePanel, themePickerOpen, split))
}

// browserSelectedStripSplit builds SplitPanelForSelected params for one browser column.
func browserSelectedStripSplit(model Model, panelID, stripN int) SelectedStripSplitParams {
	return SelectedStripSplitParams{
		StripItemCount:     stripN,
		MaxRows:            model.SelectedPanelMaxRows,
		ActivePercent:      model.SelectedPanelActivePercent,
		StripFocused:       model.ActivePanel == panelID && model.renderSubFocus() == SubFocusSelectedStrip,
		Orientation:        model.SplitOrientation,
		MinFileContentRows: MinFileListContentRows,
	}
}
