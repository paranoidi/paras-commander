package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
)

var menuBarSpinnerRunes = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// MenuBarSpinnerIcon picks the braille-pattern spinner frame from spinPhase.
func MenuBarSpinnerIcon(spinPhase uint8) rune {
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
	// SizePendingDue is PendingForPanel held back until the scan has been pending for
	// [panels].pending_marker_delay_ms; only the size column's pending marker uses it.
	SizePendingDue(absPath string, panelID int) bool
	// DiskScanBusy is true while a disk usage scan is queued or walking the filesystem.
	DiskScanBusy() bool
	// DiskScanExcluded is true when a directory would not be descended into by disk-usage traversal for this listing (godu + listing-volume gate). Stat's absPath — background classification only; paint and UI-goroutine classification must use IsKnownExcluded / diskUsagePaintExcluded.
	DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
	// IsKnownExcluded reports whether a background pass already determined absPath is excluded
	// (via MarkExcluded), with no filesystem access. False just means "not known yet", not "not excluded".
	IsKnownExcluded(absPath string) bool
}

// diskUsagePaintExcluded reports cached exclusion for row paint and UI classification.
// It never calls DiskScanExcluded, so Stat stays off the UI goroutine.
func diskUsagePaintExcluded(painter DiskUsagePainter, absPath string) bool {
	if painter == nil || absPath == "" {
		return false
	}
	return painter.IsKnownExcluded(absPath)
}

func entryDiskUsageBytes(entry localfs.Entry, show bool, painter DiskUsagePainter) int64 {
	if !show {
		painter = nil
	}
	return panellist.EntryUsageBytes(entry, painter)
}

func panelDiskUsageDenom(show bool, painter DiskUsagePainter, state panel.State) int64 {
	if !show || painter == nil {
		return 0
	}
	var max int64
	for i := range state.VisibleEntryCount() {
		e, _, ok := state.VisibleEntry(i)
		if !ok {
			continue
		}
		sz := entryDiskUsageBytes(e, true, painter)
		if sz > max {
			max = sz
		}
	}
	return max
}

// diskUsageFillColumns returns the tinted cell count; sliver is true when usage is non-zero but
// its share is under one cell (no cell is tinted then).
func diskUsageFillColumns(usageBytes, maxBytes int64, rowWidth int) (cols int, sliver bool) {
	if rowWidth <= 0 || maxBytes <= 0 || usageBytes <= 0 {
		return 0, false
	}
	if usageBytes >= maxBytes {
		return rowWidth, false
	}
	fill := int(uint64(usageBytes) * uint64(rowWidth) / uint64(maxBytes))
	return min(fill, rowWidth), fill < 1
}

func mergeDiskUsageBackground(rowStyle tcell.Style, usageAccent tcell.Style) tcell.Style {
	_, accentBG, _ := usageAccent.Decompose()
	return rowStyle.Background(accentBG)
}
