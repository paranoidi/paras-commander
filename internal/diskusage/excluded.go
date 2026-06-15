package diskusage

import "path/filepath"

// ListingVolumeGate skips descending into directories whose device differs from RefDev when Enabled && Valid.
type ListingVolumeGate struct {
	Enabled bool
	RefDev  uint64
	Valid   bool
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

// OnOtherMount reports whether absPath is on a different device than listingDev (typical mount boundary).
func OnOtherMount(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool) bool {
	return ScanExcluded(absPath, descendIntoMountPoints, listingDev, listingDevValid, nil)
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
