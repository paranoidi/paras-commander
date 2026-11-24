package panel

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ListingPathInDiskUsageScanScope reports whether listingPath is the scan origin or under a scan root.
func ListingPathInDiskUsageScanScope(listingPath, origin string, roots []string) bool {
	if origin == "" || len(roots) == 0 {
		return false
	}
	cur := pathloc.Clean(listingPath)
	if cur == pathloc.Clean(origin) {
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
	r := pathloc.Clean(root)
	c := pathloc.Clean(child)
	if c == r {
		return true
	}
	return strings.HasPrefix(c, r+string(filepath.Separator))
}
