package panelcarousel

import (
	"fmt"
	"math"
	"strconv"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

const listSizeCells = 5

type displayRune struct {
	Rune    rune
	NameIdx int
}

// formatBriefRow formats icon+name+size for carousel columns.
func formatBriefRow(entry localfs.Entry, width int, showIcons bool, jobMarkRune rune, subtreeMark bool, disk DiskUsageSource) string {
	rowTextWidth := columnListTextWidth(width, showIcons)
	nameWidth := rowTextWidth - 1 - listSizeCells
	if nameWidth < 1 {
		nameWidth = 1
	}
	display := entryDisplayRunes(entry, nameWidth, showIcons, jobMarkRune, subtreeMark)
	name := string(runesFromDisplay(display))
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

func entryListingSuffixDecorationLen(width int, jobMarkRune rune, subtreeForDir bool) int {
	n := 0
	if jobMarkRune != 0 && width > n+2 {
		n += 2
	}
	if subtreeForDir && width > n+2 {
		n += 2
	}
	return n
}

func entryDisplayRunes(entry localfs.Entry, width int, showFileIcons bool, jobMarkRune rune, subtreeSelectionMark bool) []displayRune {
	showJob := jobMarkRune != 0 && width > 2
	suffixUsed := 0
	if showJob {
		suffixUsed = 2
	}
	showSub := subtreeSelectionMark && entry.Type == localfs.EntryDirectory && width > suffixUsed+2
	suffixLen := entryListingSuffixDecorationLen(width, jobMarkRune, subtreeSelectionMark && entry.Type == localfs.EntryDirectory)
	innerW := width - suffixLen
	if innerW < 1 {
		innerW = 1
	}
	prefix := " "
	if entry.Type == localfs.EntryDirectory && !showFileIcons {
		prefix = "/"
	}
	entryRunes := []rune(entry.Name)
	var body []displayRune
	body = append(body, displayRune{Rune: []rune(prefix)[0], NameIdx: -1})
	for i, r := range entryRunes {
		body = append(body, displayRune{Rune: r, NameIdx: i})
	}
	if entry.Type == localfs.EntrySymlink {
		body = append(body, displayRune{Rune: '@', NameIdx: -1})
	}
	if width <= 0 {
		return nil
	}
	var core []displayRune
	if len(body) <= innerW {
		core = body
	} else if innerW <= 3 {
		core = body[:innerW]
	} else {
		prefixWidth := (innerW - 1) / 2
		suffixWidth := innerW - prefixWidth - 1
		truncated := make([]displayRune, 0, innerW)
		truncated = append(truncated, body[:prefixWidth]...)
		truncated = append(truncated, displayRune{Rune: '~', NameIdx: -1})
		truncated = append(truncated, body[len(body)-suffixWidth:]...)
		core = truncated
	}
	if suffixLen == 0 {
		return core
	}
	out := make([]displayRune, 0, len(core)+suffixLen)
	out = append(out, core...)
	if showJob {
		out = append(out, displayRune{Rune: ' ', NameIdx: -1}, displayRune{Rune: jobMarkRune, NameIdx: -1})
	}
	if showSub {
		out = append(out, displayRune{Rune: ' ', NameIdx: -1}, displayRune{Rune: '○', NameIdx: -1})
	}
	return out
}

func runesFromDisplay(display []displayRune) []rune {
	runes := make([]rune, len(display))
	for i, dr := range display {
		runes[i] = dr.Rune
	}
	return runes
}

func listNameHeaderTitle(showIcons bool) string {
	if showIcons {
		return " Name"
	}
	return "Name"
}

func columnListLeadingGutter() int { return 1 }

func columnListIconStrip() int { return 2 }

func columnListTextWidth(colWidth int, showIcons bool) int {
	leftGutter, iconStrip := 0, 0
	if showIcons {
		leftGutter = columnListLeadingGutter()
		iconStrip = columnListIconStrip()
	}
	rowTextWidth := colWidth - leftGutter - iconStrip
	if rowTextWidth < 1 {
		return 1
	}
	return rowTextWidth
}

func columnListContentOrigin(colX, colWidth int, showIcons bool) (listX, listW int) {
	if !showIcons {
		return colX, columnListTextWidth(colWidth, false)
	}
	leftGutter := columnListLeadingGutter()
	iconStrip := columnListIconStrip()
	return colX + leftGutter + iconStrip, columnListTextWidth(colWidth, true)
}

func briefHeader(nameTitle, sizeTitle string, rowTextWidth int) string {
	nameWidth := rowTextWidth - 1 - listSizeCells
	if nameWidth < 1 {
		nameWidth = 1
	}
	nameTitle = truncateHeaderRunes(nameWidth, nameTitle)
	sizeTitle = truncateHeaderRunes(listSizeCells, sizeTitle)
	return fmt.Sprintf("%-*s %*s", nameWidth, nameTitle, listSizeCells, sizeTitle)
}

func truncateHeaderRunes(max int, s string) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func nameWidthForColumn(colWidth int, showIcons bool) int {
	rowTextWidth := columnListTextWidth(colWidth, showIcons)
	nw := rowTextWidth - 1 - listSizeCells
	return max(1, nw)
}

// CenterNameWidth returns the name-column width for the carousel center column.
func CenterNameWidth(frame geom.Rect, showIcons bool, showChild bool) int {
	cols := SplitColumns(frame, showChild)
	if len(cols) < 2 {
		return 1
	}
	return nameWidthForColumn(cols[1].Width, showIcons)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
