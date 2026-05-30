package panelcarousel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// JobMarkFunc returns a job-queue glyph and status for an absolute path, if any.
type JobMarkFunc func(absPath string) (glyph rune, status string, ok bool)

// NewFileMarkFunc reports whether an entry should show the new-file suffix.
type NewFileMarkFunc func(entry localfs.Entry) bool

// IconPaintFunc paints the devicon strip for one listing row.
type IconPaintFunc func(screen tcell.Screen, x, y int, entry localfs.Entry, rowStyle tcell.Style, cursorThemeKey string, diskPending, diskExcluded bool)

// BodyParams holds carousel list rendering inputs (title row is painted by the caller).
type BodyParams struct {
	Frame               geom.Rect
	Center              panel.State
	Parent              Column
	Child               Column
	Styles              theme.Theme
	ChromeBlocked       bool
	FileListActive      bool
	ShowIcons           bool
	HeaderStyle         tcell.Style
	HeaderCarouselStyle tcell.Style
	SurfaceStyle        tcell.Style
	JobMark             JobMarkFunc
	NewFileMark         NewFileMarkFunc
	PaintIcon           IconPaintFunc
	DiskUsage           DiskUsage
	ShowChildColumn     bool
}

// DrawBody paints the column header row and three listing columns.
func DrawBody(screen tcell.Screen, p BodyParams) {
	visibleRows := geom.PanelListRows(p.Frame)
	if visibleRows == 0 {
		return
	}
	cols := SplitColumns(p.Frame, p.ShowChildColumn)
	headerY := p.Frame.Y + 1

	centerName, centerSize, _ := p.Center.ListColumnTitles(p.ShowIcons)
	sideNameTitle := listNameHeaderTitle(p.ShowIcons)
	for i, col := range cols {
		if col.Width <= 0 {
			continue
		}
		listTextWidth := columnListTextWidth(col.Width, p.ShowIcons)
		var hdr string
		switch i {
		case 1:
			hdr = briefHeader(centerName, centerSize, listTextWidth)
		case 0:
			if !p.Parent.Populated {
				continue
			}
			hdr = briefHeader(sideNameTitle, "Size", listTextWidth)
		case 2:
			if !p.ShowChildColumn || !p.Child.Populated {
				continue
			}
			hdr = briefHeader(sideNameTitle, "Size", listTextWidth)
		}
		hdrStyle := p.HeaderCarouselStyle
		if i == 1 {
			hdrStyle = p.HeaderStyle
		}
		hdrX, listW := columnListContentOrigin(col.X, col.Width, p.ShowIcons)
		for x := col.X; x < hdrX; x++ {
			screen.SetContent(x, headerY, ' ', nil, hdrStyle)
		}
		primitive.Text(screen, hdrX, headerY, listW, hdr, hdrStyle)
	}

	drawColumn := func(col geom.Rect, c Column, inactive bool) {
		if col.Width <= 0 {
			return
		}
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
			style := entryStyle(entry, p.ChromeBlocked, p.Styles)
			selected := selState.IsSelected(entry)
			isCursor := entryIndex == cursor
			if isCursor {
				style = cursorStyle(p, inactive, selected)
			} else if selected {
				if p.ChromeBlocked {
					style = p.Styles.PanelBlockedRowSelected
				} else {
					style = p.Styles.PanelRowSelected
				}
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
			newFile := false
			if c.Active && p.NewFileMark != nil {
				newFile = p.NewFileMark(entry)
			}
			rowSuffix := panellist.RowSuffix{
				JobGlyph:         jobGlyph,
				NewFile:          newFile,
				SubtreeSelection: subtree,
			}
			var diskSrc DiskUsageSource
			if p.DiskUsage.Active {
				diskSrc = p.DiskUsage.Source
			}
			text := formatBriefRow(entry, col.Width, p.ShowIcons, rowSuffix, p.Styles, diskSrc)
			listStart, listW := columnListContentOrigin(col.X, col.Width, p.ShowIcons)
			nameColOffset := listStart - col.X
			nameWidth := nameWidthForColumn(col.Width, p.ShowIcons)
			var spans []primitive.Span
			if c.Active && (p.Center.Filter.Active || p.Center.Filter.Editing) {
				spans = fuzzySpans(entry, col.Width, p.Center.MatchRanges(entryIndex), isCursor && p.FileListActive, p.Styles, p.ShowIcons, rowSuffix, func(di int) tcell.Style {
					return blendCell(nameColOffset + di)
				})
			}
			cursorKey := ""
			if isCursor {
				cursorKey = cursorIconKey(p, inactive, isCursor, selected)
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
					key := cursorIconKey(p, inactive, isCursor, selected)
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
	}

	drawColumn(cols[0], p.Parent, true)
	drawColumn(cols[1], Column{Kind: ColumnCenter, Populated: true, Active: true}, false)
	if p.ShowChildColumn {
		drawColumn(cols[2], p.Child, true)
	}
}

func entryStyle(entry localfs.Entry, blocked bool, styles theme.Theme) tcell.Style {
	if blocked {
		switch entry.Type {
		case localfs.EntryDirectory:
			return styles.PanelBlockedRowDirectory
		case localfs.EntrySymlink:
			return styles.PanelBlockedRowSymlink
		default:
			return styles.PanelBlockedRowFile
		}
	}
	switch entry.Type {
	case localfs.EntryDirectory:
		return styles.PanelRowDirectory
	case localfs.EntrySymlink:
		return styles.PanelRowSymlink
	default:
		return styles.PanelRowFile
	}
}

func cursorStyle(p BodyParams, inactive, selected bool) tcell.Style {
	if p.ChromeBlocked {
		if selected {
			return p.Styles.PanelBlockedCursorSelected
		}
		return p.Styles.PanelBlockedCursor
	}
	if inactive {
		if selected {
			return p.Styles.PanelCarouselInactiveCursorSelected
		}
		return p.Styles.PanelCarouselInactiveCursor
	}
	if p.FileListActive {
		if selected {
			return p.Styles.PanelActiveCursorSelected
		}
		return p.Styles.PanelCursorActive
	}
	if selected {
		return p.Styles.PanelInactiveCursorSelected
	}
	return p.Styles.PanelCursorInactive
}

func cursorIconKey(p BodyParams, inactive, isCursor, selected bool) string {
	if !isCursor {
		return ""
	}
	if p.ChromeBlocked {
		if selected {
			return "panel.blocked.row.cursor.selected"
		}
		return "panel.blocked.row.cursor"
	}
	if inactive {
		if selected {
			return "panel.carousel.inactive.row.cursor.selected"
		}
		return "panel.carousel.inactive.row.cursor"
	}
	if p.FileListActive {
		if selected {
			return "panel.active.row.cursor.selected"
		}
		return "panel.active.row.cursor"
	}
	if selected {
		return "panel.inactive.row.cursor.selected"
	}
	return "panel.inactive.row.cursor"
}

// CursorIconKeyForTest exposes cursor icon theme keys for tests.
func CursorIconKeyForTest(inactive, selected, fileListActive bool) string {
	p := BodyParams{FileListActive: fileListActive}
	return cursorIconKey(p, inactive, true, selected)
}
