package panelcarousel

import (
	"fmt"
	"math"
	"strconv"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

const listSizeCells = 5

// formatBriefRow formats icon+name+size for carousel columns.
func formatBriefRow(entry localfs.Entry, width int, showIcons bool, suffix panellist.RowSuffix, styles theme.Theme, disk DiskUsageSource, scrollbarReserve int) string {
	rowTextWidth := columnListTextWidth(width, showIcons, scrollbarReserve)
	nameWidth := rowTextWidth - 1 - listSizeCells
	if nameWidth < 1 {
		nameWidth = 1
	}
	display := panellist.EntryDisplayRunes(entry, nameWidth, showIcons, suffix, styles)
	name := string(panellist.RunesFromDisplay(display))
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

// columnScrollbarReserve returns list cells withheld for the column scrollbar (0 or 1).
func columnScrollbarReserve(hasLane bool, scrollbarEnabled bool) int {
	if hasLane && scrollbarEnabled {
		return 1
	}
	return 0
}

func briefHeader(nameTitle, sizeTitle string, rowTextWidth int) string {
	nameWidth := rowTextWidth - 1 - listSizeCells
	if nameWidth < 1 {
		nameWidth = 1
	}
	return fmt.Sprintf("%-*s %*s", nameWidth, nameTitle, listSizeCells, sizeTitle)
}

func nameWidthForColumn(colWidth int, showIcons bool, scrollbarReserve int) int {
	rowTextWidth := columnListTextWidth(colWidth, showIcons, scrollbarReserve)
	nw := rowTextWidth - 1 - listSizeCells
	if nw < 1 {
		return 1
	}
	return nw
}

// CenterNameWidth returns the name-column width for the carousel center column.
func CenterNameWidth(frame geom.Rect, showIcons bool, showChild bool) int {
	cols := SplitColumns(frame, showChild)
	if len(cols) < 2 {
		return 1
	}
	reserve := columnScrollbarReserve(showChild, true)
	return nameWidthForColumn(cols[1].Width, showIcons, reserve)
}
