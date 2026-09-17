package panellist

import (
	"fmt"
	"math"
	"strconv"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// SizeCells is the fixed display width (terminal cells) of the right-aligned size column shared
// by the plain panel list and the carousel.
const SizeCells = 5

// ByteSizer surfaces a cached directory byte size, used for the size column's directory rows
// (e.g. from a background disk-usage scan). A nil ByteSizer means no size cache is available.
type ByteSizer interface {
	ByteSize(absPath string) (int64, bool)
}

// FormatListedSize renders entry's size cell: a directory shows its cached disk-usage size (via
// disk, if non-nil and the cache has an entry) or "" when unavailable; a file shows its compact
// byte size at SizeCells width.
func FormatListedSize(entry localfs.Entry, disk ByteSizer) string {
	if entry.Type == localfs.EntryDirectory {
		if disk != nil {
			if sz, ok := disk.ByteSize(entry.Path); ok {
				return FormatByteSizeCompact(sz, SizeCells)
			}
		}
		return ""
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

// JoinRow lays out one name/meta/size row: name is left-padded to nameWidth, meta (already
// padded by the caller) follows with a two-space gap when showMeta, and size is right-aligned
// to SizeCells with a one-space gap when showSize.
func JoinRow(nameWidth int, name, meta string, showMeta bool, size string, showSize bool) string {
	if showMeta {
		if showSize {
			return fmt.Sprintf("%-*s  %s %*s", nameWidth, name, meta, SizeCells, size)
		}
		return fmt.Sprintf("%-*s  %s", nameWidth, name, meta)
	}
	if showSize {
		return fmt.Sprintf("%-*s %*s", nameWidth, name, SizeCells, size)
	}
	return fmt.Sprintf("%-*s", nameWidth, name)
}
