package diskusage

import (
	"path/filepath"
	"testing"
)

func TestComposeListingVolumeIgnoreDelegatesToBaseWhenGateDisabled(t *testing.T) {
	t.Parallel()
	base := func(abs string) bool {
		return filepath.Base(abs) == "skip"
	}
	combined := ComposeListingVolumeIgnore(base, ListingVolumeGate{})
	if !combined("/tmp/skip") {
		t.Fatal("want godu layer active")
	}
	if combined("/tmp/keep") {
		t.Fatal("want not ignored")
	}
}

func TestScanExcludedGoduIgnore(t *testing.T) {
	t.Parallel()
	g := func(abs string) bool { return filepath.Base(abs) == "node_modules" }
	if !ScanExcluded("/proj/node_modules", false, 0, false, g) {
		t.Fatal("basename rule should exclude")
	}
	if ScanExcluded("/proj/src", false, 0, false, g) {
		t.Fatal("unrelated path should not exclude via godu")
	}
}

func TestScanExcludedNilGodu(t *testing.T) {
	t.Parallel()
	if ScanExcluded("/any", false, 0, false, nil) {
		t.Fatal("no godu and invalid listing dev -> no exclusion")
	}
}

func TestScanExcludedDescendIntoMountPointsSkipsVolumeGate(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	refDev, ok := PathDevice(tmp)
	if !ok {
		t.Skip("no st_dev on this platform")
	}
	if ScanExcluded(tmp, true, refDev, true, nil) {
		t.Fatal("when descendIntoMountPoints, volume gate must not exclude same-volume path")
	}
}
