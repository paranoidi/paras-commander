package ui

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
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

// PanelTabAt returns the panel and slot (1-based) of the tab drawn at (x, y).
func PanelTabAt(layout Layout, model *Model, styles theme.Theme, x, y int) (panelID, slot int, ok bool) {
	for _, id := range [...]int{PrimaryPanel, SecondaryPanel} {
		rect := layout.PrimaryTabs
		if id == SecondaryPanel {
			rect = layout.SecondaryTabs
		}
		if !rect.Contains(x, y) || model.Tabs[id].Count() < 2 {
			continue
		}
		for _, sp := range tabSpans(rect.Width, model.Tabs[id], model.TabLabels, tabCapsOf(styles), func(slot int) string { return model.tabLabel(id, slot) }) {
			if x >= rect.X+sp.X && x < rect.X+sp.X+sp.W {
				return id, sp.Slot, true
			}
		}
	}
	return 0, 0, false
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
	row := newPanelTitleRow(file, state, styles, "", model.PanelVolumeInfo)
	for _, seg := range row.segments(state.PathString(), model.UserHomeDir) {
		if seg.Contains(x) {
			return seg, true
		}
	}
	return PanelTitleSegment{}, false
}

// carouselHitLayout is the geometry of one carousel pass in a panel's file list frame. Meta fit,
// the header hit test and the row hit test all derive from it, and drawPanelCarousel runs the
// same plan, meta fit and column split, so a hit is exactly what is drawn.
type carouselHitLayout struct {
	state       panel.State
	rows        int
	plan        carouselPanelPlan
	metaLayouts []MetaColumnLayout
	metaW       int
	cols        [3]Rect
}

// newCarouselHitLayout lays out panelID's carousel in the frame file; ok is false when the panel
// is not in carousel mode or the carousel does not fit there (the classic listing is drawn).
func newCarouselHitLayout(file Rect, model *Model, panelID int, hideInactive bool) (carouselHitLayout, bool) {
	state := model.PanelForFileListRender(panelID)
	if !state.CarouselMode {
		return carouselHitLayout{}, false
	}
	rows := PanelListRows(file)
	plan, ok := newCarouselPanelPlan(file, state, model.QuickViewDriverPanelID() >= 0, hideInactive, model.CarouselLayout, model.UseNerdfontIcons, model.PanelScrollbar, rows)
	if !ok {
		return carouselHitLayout{}, false
	}
	layouts, metaW := FitCarouselMetaColumns(file, model.CarouselLayout, state, model.UseNerdfontIcons, plan.ShowChild, model.PanelScrollbar, rows, plan.FitWidths, model.MetaResults[panelID])
	fit := panelcarousel.WithCenterMeta(plan.FitWidths, metaW)
	return carouselHitLayout{
		state: state, rows: rows, plan: plan, metaLayouts: layouts, metaW: metaW,
		cols: panelcarousel.SplitColumns(file, plan.ShowChild, model.CarouselLayout, fit),
	}, true
}

// centerHeader is the center column's header: its text origin (screen x) and the header builder's
// text and sortable titles (x screen-absolute), the same cells DrawBody paints.
func (l carouselHitLayout) centerHeader(model *Model, panelID int) (text string, columns []PanelHeaderColumn) {
	fileListActive := model.ActivePanel == panelID && model.renderSubFocus() == SubFocusFileList
	x, w := panelcarousel.CenterHeaderOrigin(l.cols[1], l.state, model.UseNerdfontIcons, l.plan.ShowChild, fileListActive, model.PanelScrollbarInactive, model.PanelScrollbar, l.rows)
	text, columns = carouselCenterHeader(w, l.state, l.metaLayouts, model.UseNerdfontIcons, model.CarouselLayout.ShowSize[1])
	for i := range columns {
		columns[i].X += x
	}
	return text, columns
}

// carouselCenterHeader is the center column's header text for a text width: the classic list's
// header builder over the size column (when shown) and the fitted meta columns.
func carouselCenterHeader(width int, state panel.State, metaLayouts []MetaColumnLayout, showIcons, showSize bool) (string, []PanelHeaderColumn) {
	var fileCols panel.ColumnSet
	if showSize {
		fileCols = panel.ColumnsBrief
	}
	return panelListHeader(width, state, fileCols, showIcons, len(metaLayouts) > 0, metaLayouts, false)
}

// CarouselHit is a carousel listing row under the pointer.
type CarouselHit struct {
	// Column is 0 for the parent column, 1 for the center column, 2 for the child column.
	Column int
	// Index is the entry's index in the column's listing.
	Index int
	Entry localfs.Entry
	// Dir is the directory the parent or child column lists; empty for the center column.
	Dir string
}

// CarouselColumnAt returns the carousel column (0 parent, 1 center, 2 child) whose list rows
// contain (x, y) in panelID's frame file, and whether the child column shows a file preview
// rather than a directory listing. ok is false outside the rows, on an unpopulated parent or
// child column, and when the panel is not drawn as a carousel.
func CarouselColumnAt(file Rect, model *Model, panelID int, hideInactive bool, x, y int) (column int, childFilePreview, ok bool) {
	l, carousel := newCarouselHitLayout(file, model, panelID, hideInactive)
	if !carousel {
		return 0, false, false
	}
	for i, col := range l.cols {
		if col.Width <= 0 || !col.Contains(x, y) {
			continue
		}
		switch i {
		case 0:
			return i, false, l.plan.Parent.Populated
		case 2:
			if !l.plan.ShowChild {
				return i, false, false
			}
			if l.plan.ChildKind == panelcarousel.ChildPreviewFile {
				return i, true, true
			}
			return i, false, l.plan.ChildKind == panelcarousel.ChildPreviewDirectoryListing && l.plan.Child.Populated
		}
		return i, false, true
	}
	return 0, false, false
}

// CarouselEntryAt returns the listing row drawn at (x, y) in panelID's carousel frame file.
// carousel reports whether the panel is drawn as a carousel at all (false: it draws the classic
// list); ok is false outside the list rows, on an unpopulated column, on a child column showing
// a file preview, and past the end of a listing.
func CarouselEntryAt(file Rect, model *Model, panelID int, hideInactive bool, x, y int) (hit CarouselHit, carousel, ok bool) {
	l, carousel := newCarouselHitLayout(file, model, panelID, hideInactive)
	if !carousel {
		return CarouselHit{}, false, false
	}
	for i, col := range l.cols {
		if col.Width <= 0 || !col.Contains(x, y) {
			continue
		}
		hit = CarouselHit{Column: i}
		var entries []localfs.Entry
		var scroll int
		switch i {
		case 0:
			if !l.plan.Parent.Populated {
				return CarouselHit{}, true, false
			}
			entries, scroll, hit.Dir = l.plan.Parent.Snapshot.Entries, l.plan.Parent.Snapshot.Scroll, l.plan.Parent.Snapshot.Path.String()
		case 1:
			entries, scroll = l.state.Entries, l.state.ScrollOffset
		default:
			if !l.plan.ShowChild || l.plan.ChildKind != panelcarousel.ChildPreviewDirectoryListing || !l.plan.Child.Populated {
				return CarouselHit{}, true, false
			}
			entries, scroll, hit.Dir = l.plan.Child.Snapshot.Entries, l.plan.Child.Snapshot.Scroll, l.plan.Child.Snapshot.Path.String()
		}
		hit.Index = scroll + y - col.Y
		if hit.Index < 0 || hit.Index >= len(entries) {
			return CarouselHit{}, true, false
		}
		hit.Entry = entries[hit.Index]
		return hit, true, true
	}
	return CarouselHit{}, true, false
}

// PanelHeaderColumnAt reports whether (x, y) lies on the column header row of panelID's file
// list frame file and, when it is on a sortable column title, that column (onTitle). It reuses
// the header layout drawPanel paints: newPanelHeaderRow for the classic list, the center
// column's header builder for a carousel (whose parent and child titles are not sortable).
// hideInactive is the effective Model.HideInactivePanel of a render snapshot.
func PanelHeaderColumnAt(file Rect, model *Model, panelID int, hideInactive bool, x, y int) (col PanelHeaderColumn, onTitle, onHeader bool) {
	if file.Width <= 2 || PanelListRows(file) == 0 || y != file.Y+1 || x <= file.X || x >= file.X+file.Width-1 {
		return PanelHeaderColumn{}, false, false
	}
	if l, ok := newCarouselHitLayout(file, model, panelID, hideInactive); ok {
		_, columns := l.centerHeader(model, panelID)
		for _, c := range columns {
			if x >= c.X && x < c.X+c.Width {
				return c, true, true
			}
		}
		return PanelHeaderColumn{}, false, true
	}
	state := model.PanelForFileListRender(panelID)
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
