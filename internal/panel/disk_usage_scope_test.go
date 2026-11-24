package panel

import (
	"path/filepath"
	"testing"
)

func TestListingPathInDiskUsageScanScope(t *testing.T) {
	t.Parallel()
	root := filepath.Clean("/tmp/project")
	scanned := filepath.Clean("/tmp/project/src")
	other := filepath.Clean("/tmp/project/docs")

	if !ListingPathInDiskUsageScanScope(root, root, []string{scanned}) {
		t.Fatal("scan origin listing should be in scope")
	}
	if !ListingPathInDiskUsageScanScope(scanned, root, []string{scanned}) {
		t.Fatal("path under scan root should be in scope")
	}
	if !ListingPathInDiskUsageScanScope(filepath.Join(scanned, "nested"), root, []string{scanned}) {
		t.Fatal("nested path under scan root should be in scope")
	}
	if ListingPathInDiskUsageScanScope(other, root, []string{scanned}) {
		t.Fatal("sibling path outside scan roots should be out of scope")
	}
	if ListingPathInDiskUsageScanScope("/tmp/other", root, []string{scanned}) {
		t.Fatal("unrelated path should be out of scope")
	}
	if ListingPathInDiskUsageScanScope(root, root, nil) {
		t.Fatal("empty scan roots should be out of scope")
	}
}

// TestListingPathInDiskUsageScanScopeInFlatRow: inside a flat tag view row the scan scope is told
// apart by the "//" row boundary, and an index path is not mistaken for a host path.
func TestListingPathInDiskUsageScanScopeInFlatRow(t *testing.T) {
	t.Parallel()
	const origin = "fileindex@(clover):///meadow//orchard"
	scanned := "fileindex@(clover):///meadow//orchard/lantern"
	if !ListingPathInDiskUsageScanScope(scanned+"/harbor", origin, []string{scanned}) {
		t.Fatal("a path inside a scanned flat-row folder should be in scope")
	}
	if !ListingPathInDiskUsageScanScope(origin+"/", origin, []string{scanned}) {
		t.Fatal("the flat-row origin should be in scope")
	}
	if ListingPathInDiskUsageScanScope("fileindex@(clover):///meadow//orchard/willow", origin, []string{scanned}) {
		t.Fatal("a sibling folder in the flat row should be out of scope")
	}
	if ListingPathInDiskUsageScanScope("fileindex:///meadow/orchard/lantern", origin, []string{scanned}) {
		t.Fatal("the plain index path of the same folder is another location")
	}
}
