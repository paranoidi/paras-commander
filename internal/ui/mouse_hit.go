package ui

import (
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// MenuBarMenuAt returns the index of the top menu whose label is drawn at (x, y). It reuses the
// menu bar's own label layout, so it is false wherever drawMenuBar shows no label.
func MenuBarMenuAt(layout Layout, model *Model, x, y int) (int, bool) {
	if !model.MenuBarInteractive() || layout.Menu.Width <= 0 || y != layout.Menu.Y {
		return 0, false
	}
	menus := menu.ActiveDefinitions(model.MenuDefinitions)
	tailW := menuBarRightTailRuneCount(model.MenuBarJobsAttention, model.MenuBarPermission, model.MenuBarActivitySpinner)
	spans, _ := menuBarLabelSpans(layout.Menu, menus, menuBarMenusClipExclusive(layout.Menu, tailW))
	for index, span := range spans {
		if x >= span.X && x < span.X+span.Width {
			return index, true
		}
	}
	return 0, false
}

// PulldownItemAt reports whether (x, y) lies on the open pulldown's frame and, when it is on an
// item row, that item's index; item is -1 on the border and on separators.
func PulldownItemAt(layout Layout, model *Model, x, y int) (item int, inside bool) {
	if !model.Menu.Open || !model.MenuBarInteractive() {
		return -1, false
	}
	menus := menu.ActiveDefinitions(model.MenuDefinitions)
	rect, ok := pulldownRect(layout, model.Menu, menus)
	if !ok || !rect.Contains(x, y) {
		return -1, false
	}
	row := y - rect.Y - 1
	items := menus[model.Menu.ActiveMenu].Items
	if x == rect.X || x == rect.X+rect.Width-1 || row < 0 || row >= rect.Height-2 || row >= len(items) || items[row].Separator {
		return -1, true
	}
	return row, true
}

// FooterKeyAt returns the footer entry drawn at (x, y): its key label or its hint, not the gaps.
func FooterKeyAt(layout Layout, model *Model, x, y int) (menu.FunctionKey, bool) {
	if layout.Footer.Height <= 0 || y != layout.Footer.Y {
		return menu.FunctionKey{}, false
	}
	for _, span := range footerHintSpans(layout.Footer, model.FooterKeys) {
		if x >= span.X && x < span.X+span.Width {
			return span.Item, true
		}
	}
	return menu.FunctionKey{}, false
}

// PanelTitleSegmentAt returns the path segment drawn at (x, y) in the title on the top border of
// panelID's file list frame file. It reuses the title painter's layout (newPanelTitleRow), so it
// is false wherever no segment is drawn and while the title shows the quick filter or a
// quick-view directory overlay.
func PanelTitleSegmentAt(file Rect, model *Model, panelID int, styles theme.Theme, x, y int) (PanelTitleSegment, bool) {
	if file.Width <= 0 || file.Height <= 0 || y != file.Y {
		return PanelTitleSegment{}, false
	}
	state := model.PanelForFileListRender(panelID)
	fileListActive := model.ActivePanel == panelID && model.renderSubFocus() == SubFocusFileList
	if panelTitleShowsFilter(state, fileListActive) {
		return PanelTitleSegment{}, false
	}
	if _, _, overlay := model.quickViewDirOverlayTitleChrome(panelID, state.PathString()); overlay {
		return PanelTitleSegment{}, false
	}
	row := newPanelTitleRow(file, state, styles, "")
	for _, seg := range row.segments(state.PathString(), model.UserHomeDir) {
		if seg.Contains(x) {
			return seg, true
		}
	}
	return PanelTitleSegment{}, false
}
