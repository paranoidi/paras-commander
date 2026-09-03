package diskusage

import "github.com/paranoidi/paras-commander/internal/localfs"

// DirectoriesNeedingScan filters pruned to the directories that are not yet cached in du and
// are not excluded by the mount-point/ignore rules, i.e. the set that still needs a disk-usage
// walk. byPath resolves a path to its listing Entry when already known (avoiding a stat);
// paths missing from byPath fall back to localfs.EntryFromPath. Returns nil when du is nil or
// pruned is empty.
func DirectoriesNeedingScan(
	pruned []string,
	byPath map[string]localfs.Entry,
	listingDev uint64,
	listingDevValid bool,
	du *Engine,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) []string {
	if du == nil || len(pruned) == 0 {
		return nil
	}
	var need []string
	for _, path := range pruned {
		entry, found := byPath[path]
		if !found {
			var err error
			entry, err = localfs.EntryFromPath(path)
			if err != nil || entry.Type != localfs.EntryDirectory {
				continue
			}
		} else if entry.Type != localfs.EntryDirectory {
			continue
		}
		if _, ok := du.ByteSize(path); ok {
			continue
		}
		if du.DiskScanExcluded(path, descendIntoMountPoints, listingDev, listingDevValid, goduIgnore) {
			du.MarkExcluded(path)
			continue
		}
		need = append(need, path)
	}
	return need
}
