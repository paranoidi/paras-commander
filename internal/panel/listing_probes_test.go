package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Supplied probes must be applied verbatim — no synchronous statfs/stat/.git lookups on apply.
func TestApplyListingWithProbesUsesSuppliedProbes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lantern.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := NewDeferred(dir, localfs.DefaultListOptions(), nil)
	if err != nil {
		t.Fatal(err)
	}
	loc := pathloc.MustParse(dir)
	entries, _, _, _, err := FetchListing(t.Context(), st.ListingRefreshSnapshot(loc, 0))
	if err != nil {
		t.Fatal(err)
	}
	probes := &PathProbes{VolumeAvail: 7, VolumeTotal: 11, VolumeOK: true, Device: 42, DeviceOK: true}
	if err := st.ApplyListingWithProbes(loc, entries, "", 10, NoIndexCursorFallback, false, probes); err != nil {
		t.Fatal(err)
	}
	if !st.VolumeSpaceOK || st.VolumeAvailBytes != 7 || st.VolumeTotalBytes != 11 {
		t.Fatalf("volume = ok:%v %d/%d, want ok 7/11", st.VolumeSpaceOK, st.VolumeAvailBytes, st.VolumeTotalBytes)
	}
	if !st.ListingDeviceValid || st.ListingDevice != 42 {
		t.Fatalf("device = valid:%v %d, want valid 42", st.ListingDeviceValid, st.ListingDevice)
	}
	if st.GitColumnActive {
		t.Fatal("empty WorkRoot in probes must leave the git column off")
	}
}
