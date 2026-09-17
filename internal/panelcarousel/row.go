package panelcarousel

import (
	"sort"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Meta is the center column's pre-laid-out meta segment (from ui.LayoutMetaColumns).
// Width 0 means no meta columns are active.
type Meta struct {
	Width  int                      // total cells incl. inter-column gaps
	Header string                   // padded header segment
	Row    func(path string) string // padded row segment for an entry path
}

// formatBriefRow formats icon+name+[meta]+size for carousel columns. metaW is the pre-laid-out
// meta segment width (0 = none, center column only) and metaText its text, already padded to metaW.
func formatBriefRow(entry localfs.Entry, width int, showIcons bool, showSize bool, suffix panellist.RowSuffix, styles theme.Theme, disk DiskUsageSource, scrollbarReserve int, metaW int, metaText string) string {
	rowTextWidth := columnListTextWidth(width, showIcons, scrollbarReserve)
	nameWidth := nameWidthFromRowText(rowTextWidth, showSize, metaW)
	display := panellist.EntryDisplayRunes(entry, nameWidth, showIcons, suffix, styles)
	name := string(panellist.RunesFromDisplay(display))
	return panellist.JoinRow(nameWidth, name, metaText, metaW > 0, panellist.FormatListedSize(entry, disk), showSize)
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

// briefHeader formats the icon+name+[meta]+size header segment. metaW/metaText mirror
// formatBriefRow's (0/"" for the parent and child columns, which never carry meta).
func briefHeader(nameTitle, sizeTitle string, rowTextWidth int, showSize bool, metaW int, metaText string) string {
	nameWidth := nameWidthFromRowText(rowTextWidth, showSize, metaW)
	return panellist.JoinRow(nameWidth, nameTitle, metaText, metaW > 0, sizeTitle, showSize)
}

// nameWidthFromRowText is the single arithmetic source for how much of a row's text width goes
// to the name once size and meta segments (each with their leading gap) are reserved.
func nameWidthFromRowText(rowTextWidth int, showSize bool, metaW int) int {
	nw := rowTextWidth
	if showSize {
		nw -= 1 + panellist.SizeCells
	}
	if metaW > 0 {
		nw -= 2 + metaW
	}
	if nw < 1 {
		return 1
	}
	return nw
}

func nameWidthForColumn(colWidth int, showIcons bool, scrollbarReserve int, showSize bool, metaW int) int {
	rowTextWidth := columnListTextWidth(colWidth, showIcons, scrollbarReserve)
	return nameWidthFromRowText(rowTextWidth, showSize, metaW)
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

// Fit-to-content outlier thresholds (see fitListingTextLen).
const (
	fitOutlierMinEntries    = 3  // gap peel vs 2nd-max
	fitOutlierMinGap        = 8  // runes; 2× was too strict (28 vs 15 failed)
	fitOutlierP90MinEntries = 10 // enough samples for a stable 90th-percentile cap
)

// maxFitEntryTextLen returns the max fitEntryTextLen over entries (plain "<N" / "<N%" fit).
func maxFitEntryTextLen(entries []localfs.Entry) int {
	max := 0
	for _, e := range entries {
		if n := fitEntryTextLen(e); n > max {
			max = n
		}
	}
	return max
}

// fitListingTextLen returns the content-fit name-text rune length for a listing, ignoring
// extreme outliers so a few absurd names truncate instead of widening the column.
// Used only for "<<N" / "<<N%" tokens (ColumnSplitSpec.IgnoreOutlier).
// Peels max when it jumps ≥fitOutlierMinGap above 2nd-max (n≥3). Larger listings (n≥10)
// also cap at the 90th percentile so two near-tied giants cannot defeat the gap rule.
func fitListingTextLen(entries []localfs.Entry) int {
	if len(entries) == 0 {
		return 0
	}
	lengths := make([]int, len(entries))
	max, second := 0, 0
	for i, e := range entries {
		n := fitEntryTextLen(e)
		lengths[i] = n
		if n > max {
			second = max
			max = n
		} else if n > second {
			second = n
		}
	}
	out := max
	if len(entries) >= fitOutlierMinEntries && max-second >= fitOutlierMinGap {
		out = second
	}
	if len(entries) >= fitOutlierP90MinEntries {
		sort.Ints(lengths)
		if p90 := lengths[(len(lengths)*9-1)/10]; p90 < out {
			out = p90
		}
	}
	return out
}

// MeasureFitColumnWidths computes uncapped whole-column content-fit widths for the parent
// (index 0) and center (index 1) columns whose layout token is fit-mode; index 2 is always 0
// (fit-mode is rejected there at parse time). Scans every entry in the column's listing (not
// just the visible window) so width doesn't jitter while scrolling. Call ONCE per render pass;
// thread the same result into SplitColumns (via DrawBody / ChildPreviewPaintRect) and
// CenterNameWidth so all three agree on the same frame's column geometry.
func MeasureFitColumnWidths(layout Layout, parent Column, center panel.State, showIcons, showChild bool, style uiscrollbar.Style, visibleRows int, metaW int) [3]int {
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
		contentLen := maxFitEntryTextLen(entries)
		if layout.Splits[i].IgnoreOutlier {
			contentLen = fitListingTextLen(entries)
		}
		if contentLen > longest {
			longest = contentLen
		}
		hasLane := columnHasScrollbarLane(c, inactive, showChild)
		total, offset := columnListingMetrics(c, center)
		reserve := columnScrollbarReserve(hasLane, true, style, total, visibleRows, offset)
		w := longest
		if showIcons {
			w += columnListLeadingGutter() + columnListIconStrip()
		}
		if layout.ShowSize[i] {
			w += 1 + panellist.SizeCells
		}
		if i == 1 && metaW > 0 {
			w += 2 + metaW
		}
		w++ // 1-char right margin so content doesn't touch the next column
		w += reserve
		out[i] = w
	}
	return out
}

// CenterNameWidth returns the name-column width for the carousel center column.
func CenterNameWidth(frame geom.Rect, layout Layout, center panel.State, showIcons, showChild bool, style uiscrollbar.Style, visibleRows int, measuredFitWidth [3]int, metaW int) int {
	cols := SplitColumns(frame, showChild, layout, measuredFitWidth)
	if len(cols) < 2 {
		return 1
	}
	c := Column{Kind: ColumnCenter, Populated: true, Active: true}
	hasLane := columnHasScrollbarLane(c, false, showChild)
	total, offset := columnListingMetrics(c, center)
	reserve := columnScrollbarReserve(hasLane, true, style, total, visibleRows, offset)
	return nameWidthForColumn(cols[1].Width, showIcons, reserve, layout.ShowSize[1], metaW)
}
