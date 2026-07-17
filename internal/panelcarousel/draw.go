package panelcarousel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// JobMarkFunc returns a job-queue glyph and status for an absolute path, if any.
type JobMarkFunc func(absPath string) (glyph rune, status string, ok bool)

// NewFileMarkFunc reports the new-file suffix tier for an entry.
type NewFileMarkFunc func(entry localfs.Entry) panellist.NewFileMarkTier

// RenameMarkFunc reports whether an entry was recently renamed.
type RenameMarkFunc func(entry localfs.Entry) bool

// IconPaintFunc paints the devicon strip for one listing row.
type IconPaintFunc func(screen tcell.Screen, x, y int, entry localfs.Entry, rowStyle tcell.Style, cursorThemeKey string, diskPending, diskExcluded bool)

// BodyParams holds carousel list rendering inputs (title row is painted by the caller).
type BodyParams struct {
	Frame                 geom.Rect
	Center                panel.State
	Parent                Column
	Child                 Column
	Styles                theme.Theme
	ChromeBlocked         bool
	FileListActive        bool
	ShowIcons             bool
	HeaderStyle           tcell.Style
	HeaderCarouselStyle   tcell.Style
	SurfaceStyle          tcell.Style
	JobMark               JobMarkFunc
	NewFileMark           NewFileMarkFunc
	RenameMark            RenameMarkFunc
	PaintIcon             IconPaintFunc
	DiskUsage             DiskUsage
	ShowChildColumn       bool
	ChildPreviewKind      ChildPreviewKind
	OtherPanelPath        string
	ScrollbarStyle        uiscrollbar.Style
	ScrollbarShowInactive bool
	InactiveFrameStyle    tcell.Style
	Layout                Layout
}

// DrawBody paints the column header row and three listing columns.
func DrawBody(screen tcell.Screen, p BodyParams) {
	visibleRows := geom.PanelListRows(p.Frame)
	if visibleRows == 0 {
		return
	}
	cols := SplitColumns(p.Frame, p.ShowChildColumn, p.Layout)
	drawCarouselHeader(screen, carouselHeaderParams{Body: p, Cols: cols, VisibleRows: visibleRows})

	drawCarouselColumn(carouselColumnParams{Screen: screen, Body: p, VisibleRows: visibleRows, Col: cols[0], C: p.Parent, Inactive: true, ColIdx: 0})
	drawCarouselColumn(carouselColumnParams{Screen: screen, Body: p, VisibleRows: visibleRows, Col: cols[1], C: Column{Kind: ColumnCenter, Populated: true, Active: true}, Inactive: false, ColIdx: 1})
	if p.ShowChildColumn && p.ChildPreviewKind == ChildPreviewDirectoryListing {
		drawCarouselColumn(carouselColumnParams{Screen: screen, Body: p, VisibleRows: visibleRows, Col: cols[2], C: p.Child, Inactive: true, ColIdx: 2})
	}
}

// carouselHeaderParams carries DrawBody's shared inputs to drawCarouselHeader.
type carouselHeaderParams struct {
	Body        BodyParams
	Cols        [3]geom.Rect
	VisibleRows int
}

// drawCarouselHeader paints the column header row (parent/center/child titles), moved
// verbatim out of DrawBody's body.
func drawCarouselHeader(screen tcell.Screen, hp carouselHeaderParams) {
	p, cols, visibleRows := hp.Body, hp.Cols, hp.VisibleRows
	headerY := p.Frame.Y + 1

	centerName, centerSize, _ := p.Center.ListColumnTitles(p.ShowIcons)
	sideNameTitle := listNameHeaderTitle(p.ShowIcons)
	for i, col := range cols {
		if col.Width <= 0 {
			continue
		}
		var c Column
		inactive := false
		switch i {
		case 0:
			c, inactive = p.Parent, true
		case 1:
			c = Column{Kind: ColumnCenter, Populated: true, Active: true}
		case 2:
			c, inactive = p.Child, true
		}
		hasLane := columnHasScrollbarLane(c, inactive, p.ShowChildColumn)
		columnActive := p.FileListActive && c.Active && !inactive
		showSB := columnActive || p.ScrollbarShowInactive
		total, offset := columnListingMetrics(c, p.Center)
		reserve := columnScrollbarReserve(hasLane, showSB, p.ScrollbarStyle, total, visibleRows, offset)
		listTextWidth := columnListTextWidth(col.Width, p.ShowIcons, reserve)
		var showSize bool
		switch i {
		case 0:
			showSize = p.Layout.ShowSize[0]
		case 1:
			showSize = p.Layout.ShowSize[1]
		case 2:
			showSize = p.Layout.ShowSize[2]
		}
		var hdr string
		switch i {
		case 1:
			sizeTitle := centerSize
			if !showSize {
				sizeTitle = ""
			}
			hdr = briefHeader(centerName, sizeTitle, listTextWidth, showSize)
		case 0:
			if !p.Parent.Populated {
				continue
			}
			hdr = briefHeader(sideNameTitle, "Size", listTextWidth, showSize)
		case 2:
			if !p.ShowChildColumn {
				continue
			}
			if p.ChildPreviewKind == ChildPreviewFile {
				continue
			}
			if !p.Child.Populated {
				continue
			}
			hdr = briefHeader(sideNameTitle, "Size", listTextWidth, showSize)
		}
		hdrStyle := p.HeaderCarouselStyle
		if i == 1 {
			hdrStyle = p.HeaderStyle
		}
		hdrX, listW := columnListContentOrigin(col.X, col.Width, p.ShowIcons, reserve)
		for x := col.X; x < hdrX; x++ {
			screen.SetContent(x, headerY, ' ', nil, hdrStyle)
		}
		primitive.Text(screen, hdrX, headerY, listW, hdr, hdrStyle)
		for x := hdrX + listW; x < col.X+col.Width; x++ {
			screen.SetContent(x, headerY, ' ', nil, hdrStyle)
		}
	}
}

// carouselColumnParams carries DrawBody's shared inputs plus one column's identity to
// drawCarouselColumn (promoted from an inline closure that closed over p, screen, and
// visibleRows); Col/C/Inactive/ColIdx vary per call, the rest are shared across all three.
type carouselColumnParams struct {
	Screen      tcell.Screen
	Body        BodyParams
	VisibleRows int
	Col         geom.Rect
	C           Column
	Inactive    bool
	ColIdx      int
}

// drawCarouselColumn paints one column's listing rows plus its scrollbar, moved verbatim out
// of DrawBody's drawColumn closure.
func drawCarouselColumn(cp carouselColumnParams) {
	screen, col, c, inactive, colIdx := cp.Screen, cp.Col, cp.C, cp.Inactive, cp.ColIdx
	p, visibleRows := cp.Body, cp.VisibleRows
	if col.Width <= 0 {
		return
	}
	hasLane := columnHasScrollbarLane(c, inactive, p.ShowChildColumn)
	columnActive := p.FileListActive && c.Active && !inactive
	showSB := columnActive || p.ScrollbarShowInactive
	total, offset := columnListingMetrics(c, p.Center)
	reserve := columnScrollbarReserve(hasLane, showSB, p.ScrollbarStyle, total, visibleRows, offset)
	var diskDenom int64
	if p.DiskUsage.Active && p.DiskUsage.Source != nil {
		var denomEntries []localfs.Entry
		if c.Active {
			denomEntries = p.Center.Entries
		} else {
			denomEntries = c.Snapshot.Entries
		}
		diskDenom = diskUsageDenom(p.DiskUsage.Source, denomEntries)
	}
	for row := 0; row < visibleRows; row++ {
		y := col.Y + row
		primitive.Fill(screen, primitive.Rect{X: col.X, Y: y, Width: col.Width, Height: 1}, ' ', p.SurfaceStyle)
		if !c.Populated {
			continue
		}
		var entries []localfs.Entry
		var cursor, scroll int
		var selState *panel.State
		if c.Active {
			selState = &p.Center
			entries = p.Center.Entries
			cursor = p.Center.Cursor
			scroll = p.Center.ScrollOffset
		} else {
			selState = &p.Center
			entries = c.Snapshot.Entries
			cursor = c.Snapshot.Cursor
			scroll = c.Snapshot.Scroll
		}
		entryIndex := scroll + row
		if entryIndex < 0 || entryIndex >= len(entries) {
			continue
		}
		entry := entries[entryIndex]
		style := p.Styles.PanelListingEntryStyle(entry.Type, p.ChromeBlocked)
		selected := selState.IsSelected(entry)
		isCursor := entryIndex == cursor
		if isCursor {
			style = p.Styles.PanelListingCursorStyle(style, theme.PanelListingCursorOpts{
				ChromeBlocked:     p.ChromeBlocked,
				FileListActive:    p.FileListActive && !inactive,
				CarouselInactive:  inactive,
				Selected:          selected,
				FilterUniqueMatch: p.FileListActive && !inactive && p.Center.FilterUniqueMatch(),
			})
		} else if selected {
			style = p.Styles.PanelListingSelectedStyle(p.ChromeBlocked)
		}
		fillCols := 0
		if !p.ChromeBlocked && diskDenom > 0 {
			fillCols = diskUsageFillColumns(entryDiskUsageBytes(entry, p.DiskUsage.Source), diskDenom, col.Width)
		}
		blendCell := func(absCol int) tcell.Style {
			if fillCols > 0 && absCol >= 0 && absCol < fillCols {
				return mergeDiskUsageBackground(style, p.Styles.DiskUsageBarStyle(p.FileListActive && c.Active && !inactive, isCursor, selected))
			}
			return style
		}
		var jobGlyph rune
		var jobStatus string
		if p.JobMark != nil {
			if g, st, ok := p.JobMark(entry.Path); ok {
				jobGlyph = g
				jobStatus = st
			}
		}
		subtree := entry.Type == localfs.EntryDirectory && selState.HasSelectionInSubtree(entry.Path)
		newFileTier := panellist.NewFileMarkNone
		if c.Active && p.NewFileMark != nil {
			newFileTier = p.NewFileMark(entry)
		}
		renameMark := false
		if c.Active && p.RenameMark != nil {
			renameMark = p.RenameMark(entry)
		}
		rowSuffix := panellist.RowSuffix{
			JobGlyph:         jobGlyph,
			NewFileTier:      newFileTier,
			RenameMark:       renameMark,
			SubtreeSelection: subtree,
		}
		var diskSrc DiskUsageSource
		if p.DiskUsage.Active {
			diskSrc = p.DiskUsage.Source
		}
		showSize := p.Layout.ShowSize[colIdx]
		text := formatBriefRow(entry, col.Width, p.ShowIcons, showSize, rowSuffix, p.Styles, diskSrc, reserve)
		listStart, listW := columnListContentOrigin(col.X, col.Width, p.ShowIcons, reserve)
		nameColOffset := listStart - col.X
		nameWidth := nameWidthForColumn(col.Width, p.ShowIcons, reserve, showSize)
		var spans []primitive.Span
		if c.Active && (p.Center.Filter.Active || p.Center.Filter.Editing) {
			spans = fuzzySpans(entry, col.Width, p.Center.MatchRanges(entryIndex), isCursor && p.FileListActive, p.Styles, p.ShowIcons, showSize, rowSuffix, reserve, func(di int) tcell.Style {
				return blendCell(nameColOffset + di)
			})
		}
		cursorKey := ""
		if isCursor {
			cursorKey = p.Styles.PanelListingCursorIconKey(theme.PanelListingCursorOpts{
				ChromeBlocked:     p.ChromeBlocked,
				FileListActive:    p.FileListActive && !inactive,
				CarouselInactive:  inactive,
				Selected:          selected,
				FilterUniqueMatch: p.FileListActive && !inactive && p.Center.FilterUniqueMatch(),
			})
		}
		if suffixSpans := panellist.ListingSuffixSpans(entry, nameWidth, p.ShowIcons, rowSuffix, jobStatus, p.Styles, p.ChromeBlocked, cursorKey, func(di int) tcell.Style {
			return blendCell(nameColOffset + di)
		}); len(suffixSpans) > 0 {
			spans = append(suffixSpans, spans...)
		}
		if p.ShowIcons {
			leftGutter := columnListLeadingGutter()
			for i := 0; i < leftGutter; i++ {
				screen.SetContent(col.X+i, y, ' ', nil, blendCell(i))
			}
			if p.PaintIcon != nil {
				key := cursorKey
				diskPending := false
				diskExcluded := false
				if p.DiskUsage.Active && p.DiskUsage.Source != nil && entry.Type == localfs.EntryDirectory {
					diskPending = p.DiskUsage.Source.PendingForPanel(entry.Path, p.DiskUsage.PanelID)
					diskExcluded = p.DiskUsage.Source.DiskScanExcluded(
						entry.Path,
						p.DiskUsage.DescendIntoMountPoints,
						p.DiskUsage.ListingDevice,
						p.DiskUsage.ListingDeviceValid,
						p.DiskUsage.GoduIgnore,
					)
				}
				p.PaintIcon(screen, col.X+leftGutter, y, entry, blendCell(leftGutter), key, diskPending, diskExcluded)
			}
		}
		primitive.StyledTextCellwise(screen, listStart, y, listW, text, func(ci int) tcell.Style {
			return blendCell(nameColOffset + ci)
		}, spans)
	}
	if columnScrollbarNeeded(hasLane, showSB, p.ScrollbarStyle, total, visibleRows, offset) {
		metrics, _ := uiscrollbar.ComputeMetrics(total, visibleRows, offset)
		uiscrollbar.Draw(uiscrollbar.DrawParams{
			Screen:     screen,
			X:          col.X + col.Width - 1,
			ListTopY:   col.Y,
			Visible:    visibleRows,
			Metrics:    metrics,
			Style:      p.ScrollbarStyle,
			Active:     columnActive,
			Blocked:    p.ChromeBlocked,
			FrameStyle: p.InactiveFrameStyle,
			Theme:      p.Styles,
		})
	}
}

// CursorIconKeyForTest exposes cursor icon theme keys for tests.
func CursorIconKeyForTest(inactive, selected, fileListActive bool) string {
	th := theme.Theme{}
	return th.PanelListingCursorIconKey(theme.PanelListingCursorOpts{
		FileListActive:   fileListActive && !inactive,
		CarouselInactive: inactive,
		Selected:         selected,
	})
}
