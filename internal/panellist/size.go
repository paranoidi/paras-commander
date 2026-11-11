package panellist

import (
	"fmt"
	"math"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DiskUsageSliver is the thin left-edge glyph drawn in the blank left gutter cell for rows whose
// disk-usage share is under one cell; the proportional tint needs at least one full cell.
const DiskUsageSliver = '\u258e'

// DiskUsageSliverStyle paints the sliver: the bar accent's background as foreground over the row style.
func DiskUsageSliverStyle(rowStyle, usageAccent tcell.Style) tcell.Style {
	_, accentBG, _ := usageAccent.Decompose()
	return rowStyle.Foreground(accentBG)
}

// SizeCells is the fixed display width (terminal cells) of the right-aligned size column shared
// by the plain panel list and the carousel.
const SizeCells = 5

// ByteSizer surfaces a cached directory byte size, used for the size column's directory rows
// (e.g. from a background disk-usage scan). A nil ByteSizer means no size cache is available.
type ByteSizer interface {
	ByteSize(absPath string) (int64, bool)
}

// EntryUsageBytes is the byte total a disk-usage bar draws for entry: a directory's cached size
// (0 while unknown), a file's own listed size. Files never read the cache: its key is a path, and
// a file reusing the name of a removed directory would inherit that directory's total.
func EntryUsageBytes(entry localfs.Entry, disk ByteSizer) int64 {
	if entry.Type != localfs.EntryDirectory {
		return entry.Size
	}
	if disk != nil {
		if sz, ok := disk.ByteSize(entry.Path); ok {
			return sz
		}
	}
	return 0
}

// FormatListedSize renders entry's size cell: a directory shows its cached disk-usage size (via
// disk, if non-nil and the cache has an entry), or pendingIcon while that size is still being
// calculated (queued or walking, right-aligned in SizeCells), or "" when
// neither applies; a file shows its compact byte size at SizeCells width.
func FormatListedSize(entry localfs.Entry, disk ByteSizer, pendingIcon string) string {
	if entry.Type == localfs.EntryDirectory {
		if disk != nil {
			if sz, ok := disk.ByteSize(entry.Path); ok {
				return FormatByteSizeCompact(sz, SizeCells)
			}
		}
		if pendingIcon == "" {
			return ""
		}
		return runewidth.FillLeft(pendingIcon, SizeCells)
	}
	return FormatByteSizeCompact(entry.Size, SizeCells)
}

// ByteCompactSuffixes are the binary-scale unit suffixes used by FormatByteSizeCompact, in
// ascending order (KiB, MiB, GiB, ...).
var ByteCompactSuffixes = [...]byte{'K', 'M', 'G', 'T', 'P', 'E'}

// FormatByteSizeCompact renders n using binary scaling (KiB steps) with a K/M/G/... suffix,
// fit into maxW cells.
func FormatByteSizeCompact(n int64, maxW int) string {
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
	suffixes := ByteCompactSuffixes[:]
	v /= float64(KiB)
	sfxIdx := 0
	for v >= 1024 && sfxIdx < len(suffixes)-1 {
		v /= 1024
		sfxIdx++
	}
	return FormatHumanScaled(v, suffixes[sfxIdx], maxW)
}

// FormatHumanScaled renders v (already divided down to its unit tier) with suffix sfx, fit into
// maxW cells: whole numbers (or values >= 10) render without a decimal, otherwise one decimal
// place is used if it fits, falling back to a truncated whole-number rendering.
func FormatHumanScaled(v float64, sfx byte, maxW int) string {
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

// metaGap separates the name from the meta segment in a row.
const metaGap = "  "

// MetaStart is the cell where JoinRow starts the meta segment for a name of nameWidth cells.
func MetaStart(nameWidth int) int { return nameWidth + len(metaGap) }

// MetaAlert is one alerting meta column of a row: its cells, relative to the start of the meta
// segment, and the theme's alert style. Cells rather than a primitive.Span: spans count runes,
// which drift from the column on a row with a wide-character name.
type MetaAlert struct {
	Start, End int
	Style      tcell.Style
}

// MetaCellStyle styles a row's cell at metaCell, relative to the start of the meta segment: an
// alerting cell takes the alert's foreground and attributes on the row background, like a row
// indicator (Theme.PanelRowIconForeground, so the cursor's icon color wins and it stays legible
// on the cursor bar; then it is bold, since the row's other cells share that colour). Any other cell
// keeps info.
func MetaCellStyle(info tcell.Style, metaCell int, alerts []MetaAlert, cursorStyleKey string, th theme.Theme) tcell.Style {
	for _, a := range alerts {
		if metaCell >= a.Start && metaCell < a.End {
			_, bg, _ := info.Decompose()
			own, _, attrs := a.Style.Decompose()
			fg := th.PanelRowIconForeground(cursorStyleKey, a.Style)
			st := tcell.StyleDefault.Foreground(fg).Background(bg).Attributes(attrs)
			if fg != own {
				st = st.Bold(true) // the cursor took its colour (its neighbours have it too): bold keeps it apart
			}
			return st
		}
	}
	return info
}

// sizeGap separates the size column from what is before it; SizeGap is its width.
const (
	sizeGap = " "
	SizeGap = len(sizeGap)
)

// JoinRow lays out one name/meta/size row: name is left-padded to nameWidth, meta (already
// padded by the caller) follows with a two-space gap when showMeta, and size is right-aligned
// to SizeCells with a one-space gap when showSize.
func JoinRow(nameWidth int, name, meta string, showMeta bool, size string, showSize bool) string {
	// Pad by terminal cells (%-*s counts runes, which overflows on 2-cell CJK glyphs).
	name = runewidth.FillRight(name, nameWidth)
	if showMeta {
		if showSize {
			return fmt.Sprintf("%s%s%s%s%*s", name, metaGap, meta, sizeGap, SizeCells, size)
		}
		return name + metaGap + meta
	}
	if showSize {
		return fmt.Sprintf("%s%s%*s", name, sizeGap, SizeCells, size)
	}
	return name
}
