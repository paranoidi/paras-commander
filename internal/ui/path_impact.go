package ui

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// PathsDeleteImpact sums recursive file count and bytes for pruned absolute paths.
func PathsDeleteImpact(
	pruned []string,
	byPath map[string]localfs.Entry,
	remote bool,
	listingDevice uint64,
	listingDeviceValid bool,
	painter DiskUsagePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) (files, bytes int64, pending bool) {
	for _, path := range pruned {
		f, b, pend := pathImpact(
			path, byPath, remote,
			listingDevice, listingDeviceValid,
			painter, descendIntoMountPoints, goduIgnore,
		)
		files += f
		bytes += b
		if pend {
			pending = true
		}
	}
	return files, bytes, pending
}

// FormatDeleteImpactSummary formats "1 file (512 B)" / "1,234 files (1.2 GiB)" with optional working glyph.
func FormatDeleteImpactSummary(files, bytes int64, pending bool, workingSym string) string {
	word := "files"
	if files == 1 {
		word = "file"
	}
	label := fmt.Sprintf("%d %s (%s)", files, word, FormatSelectionByteSize(bytes))
	if pending && workingSym != "" {
		label += " " + workingSym
	}
	return label
}

func pathImpact(
	path string,
	byPath map[string]localfs.Entry,
	remote bool,
	listingDevice uint64,
	listingDeviceValid bool,
	painter DiskUsagePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) (files, bytes int64, pending bool) {
	entry, found := byPath[path]
	if !found {
		var err error
		entry, err = localfs.EntryFromPath(path)
		if err != nil {
			return 0, 0, false
		}
	}
	if entry.Type != localfs.EntryDirectory {
		return 1, entry.Size, false
	}
	if remote {
		return 0, 0, false
	}
	if painter == nil {
		return 0, 0, true
	}
	if sz, ok := painter.ByteSize(path); ok {
		fc := int64(0)
		if n, ok := painter.FileCount(path); ok {
			fc = n
		}
		return fc, sz, false
	}
	if painter.DiskScanExcluded(path, descendIntoMountPoints, listingDevice, listingDeviceValid, goduIgnore) {
		return 0, 0, false
	}
	return 0, 0, true
}
