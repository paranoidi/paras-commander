package panel

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
)

// ListingPathInDiskUsageScanScope reports whether listingPath is the scan origin or under a scan root.
func ListingPathInDiskUsageScanScope(listingPath, origin string, roots []string) bool {
	if origin == "" || len(roots) == 0 {
		return false
	}
	cur := diskusage.CleanKey(listingPath)
	if cur == diskusage.CleanKey(origin) {
		return true
	}
	for _, root := range roots {
		if pathEqualOrUnder(cur, root) {
			return true
		}
	}
	return false
}

func pathEqualOrUnder(child, root string) bool {
	r := diskusage.CleanKey(root)
	c := diskusage.CleanKey(child)
	if c == r {
		return true
	}
	return strings.HasPrefix(c, r+string(filepath.Separator))
}
