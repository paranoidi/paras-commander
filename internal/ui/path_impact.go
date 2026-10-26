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

// SourceSizeEstimates estimates the byte size of each transfer source from what is already
// known without touching the filesystem (safe on the UI thread): listing entries via lookup and
// the disk-usage cache, by the same rules as the panel's selection-size label (pathImpact). A path
// lookup misses (a find or compare source outside the listing) still uses a cached disk-usage size.
// per[i] is -1 when source i's size is unknown (still scanning, never scanned, or a remote
// directory); total sums the known ones; complete is false when any source is unknown.
func SourceSizeEstimates(
	paths []string,
	lookup func(string) (localfs.Entry, bool),
	remote bool,
	painter DiskUsagePainter,
) (per []int64, total int64, complete bool) {
	per = make([]int64, len(paths))
	complete = true
	for i, p := range paths {
		entry, found := localfs.Entry{}, false
		if lookup != nil {
			entry, found = lookup(p)
		}
		var b int64
		known := false
		switch {
		case found:
			var pending bool
			_, b, pending = pathImpact(p, lookup, false, remote, painter)
			known = !pending && (!remote || entry.Type != localfs.EntryDirectory)
		case painter != nil && !remote:
			b, known = painter.ByteSize(p)
		}
		if !known {
			per[i] = -1
			complete = false
			continue
		}
		per[i] = b
		total += b
	}
	return per, total, complete
}
