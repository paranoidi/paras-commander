package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestSourceSizeEstimates(t *testing.T) {
	t.Parallel()
	listing := map[string]localfs.Entry{
		"/fern/quarry.txt":    {Path: "/fern/quarry.txt", Type: localfs.EntryFile, Size: 300},
		"/fern/lantern":       {Path: "/fern/lantern", Type: localfs.EntryDirectory},
		"/fern/meadow-stones": {Path: "/fern/meadow-stones", Type: localfs.EntryDirectory},
	}
	lookup := func(p string) (localfs.Entry, bool) {
		e, ok := listing[p]
		return e, ok
	}
	painter := partialDiskUsagePainter{sizes: map[string]int64{
		"/fern/lantern":        2000, // scanned directory in the listing
		"/elsewhere/harvest":   700,  // off-listing source (find/compare) with a cached size
		"/fern/unused-cobbler": 999,
	}}

	tests := []struct {
		name     string
		paths    []string
		remote   bool
		per      []int64
		total    int64
		complete bool
	}{
		{"listed file and scanned dir", []string{"/fern/quarry.txt", "/fern/lantern"}, false, []int64{300, 2000}, 2300, true},
		{"dir still pending", []string{"/fern/quarry.txt", "/fern/meadow-stones"}, false, []int64{300, -1}, 300, false},
		{"off-listing path from the cache", []string{"/elsewhere/harvest"}, false, []int64{700}, 700, true},
		{"off-listing path never scanned", []string{"/elsewhere/thimble"}, false, []int64{-1}, 0, false},
		{"remote file known, remote dir unknown", []string{"/fern/quarry.txt", "/fern/lantern"}, true, []int64{300, -1}, 300, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			per, total, complete := SourceSizeEstimates(tc.paths, lookup, tc.remote, painter)
			if total != tc.total || complete != tc.complete {
				t.Fatalf("total=%d complete=%v, want %d %v", total, complete, tc.total, tc.complete)
			}
			for i := range per {
				if per[i] != tc.per[i] {
					t.Fatalf("per = %v, want %v", per, tc.per)
				}
			}
		})
	}
}
