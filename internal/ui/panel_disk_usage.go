package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

var menuBarSpinnerRunes = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// MenuBarSpinnerGlyph picks the braille-pattern spinner frame from spinPhase.
func MenuBarSpinnerGlyph(spinPhase uint8) rune {
	if len(menuBarSpinnerRunes) == 0 {
		return '?'
	}
	return menuBarSpinnerRunes[int(spinPhase)%len(menuBarSpinnerRunes)]
}

// DiskUsagePainter surfaces size cache + scan state for proportional row painting.
type DiskUsagePainter interface {
	ByteSize(absPath string) (n int64, ok bool)
	// FileCount returns cached recursive file count for a directory or 1 for a cached file path.
	FileCount(absPath string) (n int64, ok bool)
	// PendingForPanel is true when absPath should use the disk-scan folder tint for this panel
	// (PrimaryPanel/SecondaryPanel): queued, walking, or under such a root for that panel only.
	PendingForPanel(absPath string, panelID int) bool
	// DiskScanBusy is true while a disk usage scan is queued or walking the filesystem.
	DiskScanBusy() bool
	// DiskScanExcluded is true when a directory would not be descended into by disk-usage traversal for this listing (godu + listing-volume gate). Stat's absPath — avoid calling this per path in a loop over a large selection; use IsKnownExcluded there instead.
	DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
	// IsKnownExcluded reports whether a background pass already determined absPath is excluded
	// (via MarkExcluded), with no filesystem access. False just means "not known yet", not "not excluded".
	IsKnownExcluded(absPath string) bool
}

func entryDiskUsageBytes(entry localfs.Entry, show bool, painter DiskUsagePainter) int64 {
	if !show || painter == nil {
		if entry.Type != localfs.EntryDirectory {
			return entry.Size
		}
		return 0
	}
	if sz, ok := painter.ByteSize(entry.Path); ok {
		return sz
	}
	if entry.Type != localfs.EntryDirectory {
		return entry.Size
	}
	return 0
}

func panelDiskUsageDenom(show bool, painter DiskUsagePainter, entries []localfs.Entry) int64 {
	if !show || painter == nil || len(entries) == 0 {
		return 0
	}
	var max int64
	for _, e := range entries {
		sz := entryDiskUsageBytes(e, true, painter)
		if sz > max {
			max = sz
		}
	}
	return max
}

func diskUsageFillColumns(usageBytes, maxBytes int64, rowWidth int) int {
	if rowWidth <= 0 || maxBytes <= 0 || usageBytes <= 0 {
		return 0
	}
	if usageBytes >= maxBytes {
		return rowWidth
	}
	fill := int(uint64(usageBytes) * uint64(rowWidth) / uint64(maxBytes))
	if fill < 1 {
		return 1
	}
	if fill > rowWidth {
		return rowWidth
	}
	return fill
}

func mergeDiskUsageBackground(rowStyle tcell.Style, usageAccent tcell.Style) tcell.Style {
	_, accentBG, _ := usageAccent.Decompose()
	return rowStyle.Background(accentBG)
}
