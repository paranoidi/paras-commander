package panelcarousel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// DiskUsageSource surfaces size cache and scan state for carousel row painting.
type DiskUsageSource interface {
	ByteSize(absPath string) (n int64, ok bool)
	PendingForPanel(absPath string, panelID int) bool
	DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
}

// DiskUsage configures proportional disk-usage bars in carousel columns.
type DiskUsage struct {
	Active                 bool
	PanelID                int
	ListingDevice          uint64
	ListingDeviceValid     bool
	DescendIntoMountPoints bool
	GoduIgnore             func(string) bool
	Source                 DiskUsageSource
}

func entryDiskUsageBytes(entry localfs.Entry, src DiskUsageSource) int64 {
	if src == nil {
		if entry.Type != localfs.EntryDirectory {
			return entry.Size
		}
		return 0
	}
	if sz, ok := src.ByteSize(entry.Path); ok {
		return sz
	}
	if entry.Type != localfs.EntryDirectory {
		return entry.Size
	}
	return 0
}

func diskUsageDenom(src DiskUsageSource, entries []localfs.Entry) int64 {
	if src == nil || len(entries) == 0 {
		return 0
	}
	var max int64
	for _, e := range entries {
		sz := entryDiskUsageBytes(e, src)
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
