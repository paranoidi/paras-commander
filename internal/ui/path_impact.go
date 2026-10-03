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
	painter DiskUsagePainter,
) (files, bytes int64, pending bool) {
	for _, path := range pruned {
		f, b, pend := pathImpact(path, func(p string) (localfs.Entry, bool) {
			e, ok := byPath[p]
			return e, ok
		}, true, remote, painter)
		files += f
		bytes += b
		if pend {
			pending = true
		}
	}
	return files, bytes, pending
}

// FormatDeleteImpactSummary formats "1 file (512 B)" / "1,234 files (1.2 GiB)" with optional working icon.
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
	lookup func(string) (localfs.Entry, bool),
	statOnMiss bool,
	remote bool,
	painter DiskUsagePainter,
) (files, bytes int64, pending bool) {
	var entry localfs.Entry
	found := false
	if lookup != nil {
		entry, found = lookup(path)
	}
	if !found {
		if !statOnMiss {
			// Render path: never stat. A background Lstat (reconcileSelectionSizeScans) fills the
			// off-listing cache; remote panels have no such pass, so a miss there is just unknown.
			return 0, 0, !remote
		}
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
	// IsKnownExcluded only, not DiskScanExcluded: this runs once per selected/pruned root here,
	// so a live Stat fallback would reintroduce a synchronous per-path filesystem call on every
	// render for a large selection. A background reconcile pass (see diskusage.MarkExcluded)
	// populates the excluded cache; until it does, an excluded path just stays "pending".
	if painter.IsKnownExcluded(path) {
		return 0, 0, false
	}
	return 0, 0, true
}
