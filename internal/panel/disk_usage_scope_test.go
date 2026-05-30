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
