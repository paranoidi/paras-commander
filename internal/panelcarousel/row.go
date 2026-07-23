package panelcarousel

import (
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

const listSizeCells = 5

// formatBriefRow formats icon+name+size for carousel columns.
func formatBriefRow(entry localfs.Entry, width int, showIcons bool, showSize bool, suffix panellist.RowSuffix, styles theme.Theme, disk DiskUsageSource, scrollbarReserve int) string {
	rowTextWidth := columnListTextWidth(width, showIcons, scrollbarReserve)
	nameWidth := rowTextWidth
	if showSize {
		nameWidth = rowTextWidth - 1 - listSizeCells
		if nameWidth < 1 {
			nameWidth = 1
		}
	}
	if nameWidth < 1 {
		nameWidth = 1
	}
	display := panellist.EntryDisplayRunes(entry, nameWidth, showIcons, suffix, styles)
	name := string(panellist.RunesFromDisplay(display))
	if !showSize {
		return fmt.Sprintf("%-*s", nameWidth, name)
	}
	return fmt.Sprintf("%-*s %*s", nameWidth, name, listSizeCells, formatListedSize(entry, disk))
}

func formatListedSize(entry localfs.Entry, disk DiskUsageSource) string {
	if entry.Type == localfs.EntryDirectory {
		if disk != nil {
			if sz, ok := disk.ByteSize(entry.Path); ok {
				return formatByteSizeCompact(sz, listSizeCells)
			}
		}
		return ""
	}
	return formatByteSizeCompact(entry.Size, listSizeCells)
}

var byteCompactSuffixes = []byte{'K', 'M', 'G', 'T', 'P', 'E'}

func formatByteSizeCompact(n int64, maxW int) string {
	const KiB = int64(1024)
	if maxW < 1 {
		return ""
	}
	if n < 0 {
		n = 0
	}
	if n < KiB {
		s := strconv.FormatInt(n, 10)
		if len(s) > maxW {
			return s[:maxW]
		}
		return s
	}
	v := float64(n)
	suffixes := byteCompactSuffixes[:]
	v /= float64(KiB)
	sfxIdx := 0
	for v >= 1024 && sfxIdx < len(suffixes)-1 {
		v /= 1024
		sfxIdx++
	}
	return formatHumanScaled(v, suffixes[sfxIdx], maxW)
}

func formatHumanScaled(v float64, sfx byte, maxW int) string {
	if v >= 10 || math.Abs(v-math.Round(v)) < 1e-3 {
		s := fmt.Sprintf("%.0f%c", v, sfx)
		if len(s) <= maxW {
			return s
		}
	}
	s := fmt.Sprintf("%.1f%c", v, sfx)
	if len(s) <= maxW {
		return s
	}
	s = fmt.Sprintf("%.0f%c", v, sfx)
	if len(s) > maxW {
		return s[:maxW]
	}
	return s
}

func listNameHeaderTitle(showIcons bool) string {
	if showIcons {
		return " Name"
	}
	return "Name"
}

func columnListLeadingGutter() int { return 1 }

func columnListIconStrip() int { return 2 }

func columnListTextWidth(colWidth int, showIcons bool, scrollbarReserve int) int {
	leftGutter, iconStrip := 0, 0
	if showIcons {
		leftGutter = columnListLeadingGutter()
		iconStrip = columnListIconStrip()
	}
	rowTextWidth := colWidth - leftGutter - iconStrip - scrollbarReserve
	if rowTextWidth < 1 {
		return 1
	}
	return rowTextWidth
}

func columnListContentOrigin(colX, colWidth int, showIcons bool, scrollbarReserve int) (listX, listW int) {
	if !showIcons {
		return colX, columnListTextWidth(colWidth, false, scrollbarReserve)
	}
	leftGutter := columnListLeadingGutter()
	iconStrip := columnListIconStrip()
	return colX + leftGutter + iconStrip, columnListTextWidth(colWidth, true, scrollbarReserve)
}

// columnHasScrollbarLane reports whether a carousel column owns a vertical scrollbar track.
func columnHasScrollbarLane(c Column, inactive, showChild bool) bool {
	return c.Populated && (!c.Active || inactive || showChild)
}

// columnListingMetrics returns scroll inputs for one carousel column.
func columnListingMetrics(c Column, center panel.State) (total, offset int) {
	if c.Active {
		return len(center.Entries), center.ScrollOffset
	}
	return len(c.Snapshot.Entries), c.Snapshot.Scroll
}

// columnScrollbarNeeded reports whether a vertical scrollbar is painted in the column lane.
func columnScrollbarNeeded(hasLane, showSB bool, style uiscrollbar.Style, total, visibleRows, offset int) bool {
	if !hasLane || !showSB || style == uiscrollbar.StyleNone {
		return false
	}
	_, ok := uiscrollbar.ComputeMetrics(total, visibleRows, offset)
	return ok
}

// columnScrollbarReserve returns list cells withheld for the column scrollbar (0 or 1).
func columnScrollbarReserve(hasLane, showSB bool, style uiscrollbar.Style, total, visibleRows, offset int) int {
	if columnScrollbarNeeded(hasLane, showSB, style, total, visibleRows, offset) {
		return 1
	}
	return 0
}

func briefHeader(nameTitle, sizeTitle string, rowTextWidth int, showSize bool) string {
	nameWidth := rowTextWidth
	if showSize {
		nameWidth = rowTextWidth - 1 - listSizeCells
		if nameWidth < 1 {
			nameWidth = 1
		}
		return fmt.Sprintf("%-*s %*s", nameWidth, nameTitle, listSizeCells, sizeTitle)
	}
	if nameWidth < 1 {
		nameWidth = 1
	}
	return fmt.Sprintf("%-*s", nameWidth, nameTitle)
}

func nameWidthForColumn(colWidth int, showIcons bool, scrollbarReserve int, showSize bool) int {
	rowTextWidth := columnListTextWidth(colWidth, showIcons, scrollbarReserve)
	if !showSize {
		if rowTextWidth < 1 {
			return 1
		}
		return rowTextWidth
	}
	nw := rowTextWidth - 1 - listSizeCells
	if nw < 1 {
		return 1
	}
	return nw
}

// fitEntryTextLen returns one entry's rendered name-text rune length (leading prefix rune +
// name + '@' suffix for symlinks), matching panellist.EntryDisplayRunes' body construction.
// Per-row transient decorations (job marks, new-file/rename badges) are excluded on purpose:
// those change independent of directory content and would make column width flicker.
func fitEntryTextLen(e localfs.Entry) int {
	n := 1 + utf8.RuneCountInString(e.Name)
	if e.Type == localfs.EntrySymlink {
		n++
	}
	return n
}

// MeasureFitColumnWidths computes uncapped whole-column content-fit widths for the parent
// (index 0) and center (index 1) columns whose layout token is fit-mode; index 2 is always 0
// (fit-mode is rejected there at parse time). Scans every entry in the column's listing (not
// just the visible window) so width doesn't jitter while scrolling. Call ONCE per render pass;
// thread the same result into SplitColumns (via DrawBody / ChildPreviewPaintRect) and
// CenterNameWidth so all three agree on the same frame's column geometry.
func MeasureFitColumnWidths(layout Layout, parent Column, center panel.State, showIcons, showChild bool, style uiscrollbar.Style, visibleRows int) [3]int {
	var out [3]int
	for i := 0; i < 2; i++ {
		if k := layout.Splits[i].Kind; k != SplitFitChars && k != SplitFitPercent {
			continue
		}
		var entries []localfs.Entry
		var c Column
		var nameTitle string
		inactive := false
		if i == 0 {
			if !parent.Populated {
				continue
			}
			entries = parent.Snapshot.Entries
			c = parent
			inactive = true
			nameTitle = listNameHeaderTitle(showIcons)
		} else {
			entries = center.Entries
			c = Column{Kind: ColumnCenter, Populated: true, Active: true}
			nameTitle, _, _ = center.ListColumnTitles(showIcons)
		}
		if len(entries) == 0 {
			continue
		}
		// Header title ("Name" / " Name" / "↓Name" ...) must always fit uncropped, so the
		// fit-to-content width can never shrink below it (see briefHeader).
		longest := utf8.RuneCountInString(nameTitle)
		for _, e := range entries {
			if n := fitEntryTextLen(e); n > longest {
				longest = n
			}
		}
		hasLane := columnHasScrollbarLane(c, inactive, showChild)
		total, offset := columnListingMetrics(c, center)
		reserve := columnScrollbarReserve(hasLane, true, style, total, visibleRows, offset)
		w := longest
		if showIcons {
			w += columnListLeadingGutter() + columnListIconStrip()
		}
		if layout.ShowSize[i] {
			w += 1 + listSizeCells
		}
		w++ // 1-char right margin so content doesn't touch the next column
		w += reserve
		out[i] = w
	}
	return out
}

// CenterNameWidth returns the name-column width for the carousel center column.
func CenterNameWidth(frame geom.Rect, layout Layout, center panel.State, showIcons, showChild bool, style uiscrollbar.Style, visibleRows int, measuredFitWidth [3]int) int {
	cols := SplitColumns(frame, showChild, layout, measuredFitWidth)
	if len(cols) < 2 {
		return 1
	}
	c := Column{Kind: ColumnCenter, Populated: true, Active: true}
	hasLane := columnHasScrollbarLane(c, false, showChild)
	total, offset := columnListingMetrics(c, center)
	reserve := columnScrollbarReserve(hasLane, true, style, total, visibleRows, offset)
	return nameWidthForColumn(cols[1].Width, showIcons, reserve, layout.ShowSize[1])
}
