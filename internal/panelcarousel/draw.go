package panelcarousel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// JobMarkFunc returns a job-queue glyph for an absolute path, if any.
type JobMarkFunc func(absPath string) (glyph rune, ok bool)

// IconPaintFunc paints the devicon strip for one listing row.
type IconPaintFunc func(screen tcell.Screen, x, y int, entry localfs.Entry, rowStyle tcell.Style, cursorThemeKey string)

// BodyParams holds carousel list rendering inputs (title row is painted by the caller).
type BodyParams struct {
	Frame          geom.Rect
	Center         panel.State
	Parent         Column
	Child          Column
	Styles         theme.Theme
	ChromeBlocked  bool
	FileListActive bool
	ShowIcons      bool
	HeaderStyle    tcell.Style
	SurfaceStyle   tcell.Style
	JobMark        JobMarkFunc
	PaintIcon      IconPaintFunc
}

// DrawBody paints the column header row and three listing columns.
func DrawBody(screen tcell.Screen, p BodyParams) {
	visibleRows := geom.PanelListRows(p.Frame)
	if visibleRows == 0 {
		return
	}
	cols := SplitColumns(p.Frame)
	headerY := p.Frame.Y + 1

	centerName, centerSize, _ := p.Center.ListColumnTitles(p.ShowIcons)
	for i, col := range cols {
		if col.Width <= 0 {
			continue
		}
		var hdr string
		switch i {
		case 1:
			hdr = briefHeader(centerName, centerSize, col.Width)
		case 0:
			if !p.Parent.Populated {
				continue
			}
			hdr = briefHeader("Name", "Size", col.Width)
		case 2:
			if !p.Child.Populated {
				continue
			}
			hdr = briefHeader("Name", "Size", col.Width)
		}
		primitive.Text(screen, col.X, headerY, col.Width, hdr, p.HeaderStyle)
	}

	drawColumn := func(col geom.Rect, c Column, inactive bool) {
		if col.Width <= 0 {
			return
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
			var jobGlyph rune
			if p.JobMark != nil {
				if g, ok := p.JobMark(entry.Path); ok {
					jobGlyph = g
				}
			}
			subtree := entry.Type == localfs.EntryDirectory && selState.HasSelectionInSubtree(entry.Path)
			text := formatBriefRow(entry, col.Width, p.ShowIcons, jobGlyph, subtree)
			var spans []primitive.Span
			if c.Active && (p.Center.Filter.Active || p.Center.Filter.Editing) {
				spans = fuzzySpans(entry, col.Width, p.Center.MatchRanges(entryIndex), isCursor && p.FileListActive, p.Styles, p.ShowIcons, jobGlyph, subtree, func(di int) tcell.Style {
					return style
				})
			}
			leftGutter := 0
			iconStrip := 0
			if p.ShowIcons {
				leftGutter = 1
				iconStrip = 2
			}
			listStart := col.X
			if p.ShowIcons {
				for i := 0; i < leftGutter; i++ {
					screen.SetContent(col.X+i, y, ' ', nil, style)
				}
				listStart = col.X + leftGutter + iconStrip
				if p.PaintIcon != nil {
					key := cursorIconKey(p, inactive, isCursor, selected)
					p.PaintIcon(screen, col.X+leftGutter, y, entry, style, key)
				}
			}
			listW := col.Width - leftGutter - iconStrip
			if listW < 1 {
				listW = 1
			}
			primitive.StyledTextCellwise(screen, listStart, y, listW, text, func(int) tcell.Style { return style }, spans)
		}
	}

	drawColumn(cols[0], p.Parent, true)
	drawColumn(cols[1], Column{Kind: ColumnCenter, Populated: true, Active: true}, false)
	drawColumn(cols[2], p.Child, true)
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
