package diskusage

import "path/filepath"

// ListingVolumeGate skips descending into directories whose device differs from RefDev when Enabled && Valid.
// FromRoot resolves RefDev from the scan root itself (off the UI goroutine) so an explicitly
// requested scan of a mount point counts that mount while still not crossing nested mounts.
type ListingVolumeGate struct {
	Enabled  bool
	RefDev   uint64
	Valid    bool
	FromRoot bool
}

// ComposeListingVolumeIgnore wraps base (e.g. ~/.goduignore) with listing-volume skipping for WalkFolder.
func ComposeListingVolumeIgnore(base ShouldIgnoreFolder, gate ListingVolumeGate) ShouldIgnoreFolder {
	return func(abs string) bool {
		if base != nil && base(abs) {
			return true
		}
		if !gate.Enabled || !gate.Valid {
			return false
		}
		dev, ok := pathStatDevice(filepath.Clean(abs))
		if !ok {
			return true
		}
		return dev != gate.RefDev
	}
}

// ScanExcluded reports whether a directory would not be descended into by disk-usage rules used for the listing row (godu + optional mount gate).
func ScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore ShouldIgnoreFolder) bool {
	clean := filepath.Clean(absPath)
	if goduIgnore != nil && goduIgnore(clean) {
		return true
	}
	if descendIntoMountPoints || !listingDevValid {
		return false
	}
	dev, ok := pathStatDevice(clean)
	if !ok {
		return false
	}
	return dev != listingDev
}
