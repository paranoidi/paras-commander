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

// StatusCommandsAt reports whether (x, y) lies in the top-left status-command area
// (layout.StatusCmd), which is only reserved while at least one status command is drawn.
func StatusCommandsAt(layout Layout, model *Model, x, y int) bool {
	return model.MenuBarInteractive() && layout.StatusCmd.Contains(x, y)
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

// FooterKeyAt returns the footer entry drawn at (x, y) (its key label or its hint, not the gaps)
// and whether x is on its Shift-alternative word (menu.FunctionKey.ShiftEvent).
func FooterKeyAt(layout Layout, model *Model, x, y int) (fk menu.FunctionKey, shift, ok bool) {
	hover, fk := footerHit(layout.Footer, model.FooterKeys, x, y)
	return fk, hover.Shift, hover.Active
}

// FooterHoverAt is the footer entry under (x, y) as a FooterHover (the hit FooterKeyAt reports).
func FooterHoverAt(layout Layout, model *Model, x, y int) FooterHover {
	hover, _ := footerHit(layout.Footer, model.FooterKeys, x, y)
	return hover
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

// PanelHeaderColumnAt reports whether (x, y) lies on the column header row of panelID's file
// list frame file and, when it is on a sortable column title, that column (onTitle). It reuses
// drawPanel's header layout (newPanelHeaderRow). A carousel panel has no header row here.
func PanelHeaderColumnAt(file Rect, model *Model, panelID, x, y int) (col PanelHeaderColumn, onTitle, onHeader bool) {
	if file.Width <= 2 || PanelListRows(file) == 0 || y != file.Y+1 || x <= file.X || x >= file.X+file.Width-1 {
		return PanelHeaderColumn{}, false, false
	}
	state := model.PanelForFileListRender(panelID)
	if state.CarouselMode {
		return PanelHeaderColumn{}, false, false
	}
	display := PanelDisplayConfig{ShowIcons: model.UseNerdfontIcons, NarrowPanelsNameOnly: model.NarrowPanelsNameOnly}
	layoutRes := panelColumnLayout(file, state, display)
	metaLayouts, _ := fitPanelMetaColumns(layoutRes, state, model.MetaResults[panelID])
	row := newPanelHeaderRow(layoutRes, state, display.ShowIcons, metaLayouts)
	for _, c := range row.Columns {
		if x >= c.X && x < c.X+c.Width {
			return c, true, true
		}
	}
	return PanelHeaderColumn{}, false, true
}
