package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// panelSelectionSizePadded wraps raw selection-size text with one space on each side for the bottom row.
func panelSelectionSizePadded(raw string) string {
	if raw == "" {
		return ""
	}
	return " " + raw + " "
}

// panelSelectionSizeCenterLayout places the padded label in the horizontal center of the panel bottom interior row.
func panelSelectionSizeCenterLayout(rect Rect, rawLabel string) (padded string, startX, endX int, ok bool) {
	padded = panelSelectionSizePadded(rawLabel)
	w := utf8.RuneCountInString(padded)
	if w == 0 {
		return "", 0, 0, false
	}
	firstIn := rect.X + 1
	lastIn := rect.X + rect.Width - 2
	if lastIn < firstIn {
		return "", 0, 0, false
	}
	interiorW := lastIn - firstIn + 1
	if w > interiorW {
		return "", 0, 0, false
	}
	leftPad := (interiorW - w) / 2
	startX = firstIn + leftPad
	endX = startX + w - 1
	return padded, startX, endX, true
}

// FormatSelectionByteSize renders n with binary units (B, KiB, MiB, …) using at most two fractional digits.
func FormatSelectionByteSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		KiB = int64(1024)
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case n < KiB:
		return fmt.Sprintf("%d B", n)
	case n < MiB:
		return formatSelectionUnit(float64(n)/float64(KiB), "KiB")
	case n < GiB:
		return formatSelectionUnit(float64(n)/float64(MiB), "MiB")
	case n < TiB:
		return formatSelectionUnit(float64(n)/float64(GiB), "GiB")
	default:
		return formatSelectionUnit(float64(n)/float64(TiB), "TiB")
	}
}

func formatSelectionUnit(v float64, unit string) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f %s", v, unit)
	}
	if v >= 10 && math.Abs(v-math.Round(v)) < 1e-6 {
		return fmt.Sprintf("%.0f %s", v, unit)
	}
	s := fmt.Sprintf("%.2f", v)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s + " " + unit
}

// SelectionSizeLabel builds the selection count/size indicator text for a panel.
// workingSym is appended (with a leading space) when directory sizes are still being resolved.
func SelectionSizeLabel(
	state panel.State,
	remote bool,
	painter DiskUsagePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
	workingSym string,
) (label string, ok bool) {
	count := state.SelectedPathCount()
	if count == 0 {
		return "", false
	}

	paths := make([]string, 0, count)
	for p, on := range state.SelectedPaths {
		if on {
			paths = append(paths, p)
		}
	}
	pruned := panel.PruneNestedPaths(paths)

	byPath := make(map[string]localfs.Entry, len(state.Entries))
	for _, e := range state.Entries {
		byPath[e.Path] = e
	}

	var total int64
	pending := false
	for _, p := range pruned {
		n, needScan := selectionPathBytes(
			p, byPath, remote, state, painter,
			descendIntoMountPoints, goduIgnore,
		)
		total += n
		if needScan {
			pending = true
		}
	}

	word := "items"
	if count == 1 {
		word = "item"
	}
	label = fmt.Sprintf("%d %s (%s)", count, word, FormatSelectionByteSize(total))
	if pending && workingSym != "" {
		label += " " + workingSym
	}
	return label, true
}

func selectionPathBytes(
	path string,
	byPath map[string]localfs.Entry,
	remote bool,
	state panel.State,
	painter DiskUsagePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) (bytes int64, needScan bool) {
	entry, found := byPath[path]
	if !found {
		var err error
		entry, err = localfs.EntryFromPath(path)
		if err != nil {
			return 0, false
		}
	}
	if entry.Type != localfs.EntryDirectory {
		return entry.Size, false
	}
	if remote {
		return 0, false
	}
	if painter == nil {
		return 0, true
	}
	if sz, ok := painter.ByteSize(path); ok {
		return sz, false
	}
	if painter.DiskScanExcluded(path, descendIntoMountPoints, state.ListingDevice, state.ListingDeviceValid, goduIgnore) {
		return 0, false
	}
	return 0, true
}

// SelectedPathsSorted returns selected paths in stable sorted order.
func SelectedPathsSorted(state panel.State) []string {
	if len(state.SelectedPaths) == 0 {
		return nil
	}
	out := make([]string, 0, len(state.SelectedPaths))
	for p, on := range state.SelectedPaths {
		if on {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
