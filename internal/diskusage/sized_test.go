package diskusage

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// TestSizerTotalsPrefixedRootsAndOrigin: roots under the sizer prefix are sized by the sizer
// (no disk walk, a gone path stays uncached), the origin gets the sizer's own total, and
// ForgetSized drops only sized keys.
func TestSizerTotalsPrefixedRootsAndOrigin(t *testing.T) {
	t.Parallel()
	e := New()
	totals := map[string]int64{"fileindex:///w": 30, "fileindex:///w/a": 10, "fileindex:///w/b": 20}
	e.SetSubtreeSizer(func(p string) bool { return strings.HasPrefix(p, "fileindex://") }, func(_ context.Context, p string) (int64, int64, error) {
		n, ok := totals[p]
		if !ok {
			return 0, 0, fs.ErrNotExist
		}
		return n, n / 10, nil
	})
	e.SetSize("/local", 1, 5)

	e.StartScanFromDirectory("fileindex:///w/", []string{"fileindex:///w/a", "fileindex:///w/b/", "fileindex:///w/gone"}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { _, ok := e.ByteSize("fileindex:///w"); return ok && !e.DiskScanBusy() }, 2*time.Second, "origin sized")
	for p, want := range totals {
		if got, ok := e.ByteSize(p); !ok || got != want {
			t.Fatalf("ByteSize(%s) = %d, %v; want %d", p, got, ok, want)
		}
		if got, _ := e.FileCount(p); got != want/10 {
			t.Fatalf("FileCount(%s) = %d, want %d", p, got, want/10)
		}
	}
	if _, ok := e.ByteSize("fileindex:///w/gone"); ok {
		t.Fatal("a path the sizer lacks must stay uncached")
	}
	if e.DiskScanExcluded("fileindex:///w/a", false, 1, true, func(string) bool { return true }) {
		t.Fatal("sized paths are never excluded by disk rules")
	}

	e.ForgetSized()
	if _, ok := e.ByteSize("fileindex:///w/a"); ok {
		t.Fatal("ForgetSized kept a sized total")
	}
	if _, ok := e.ByteSize("/local"); !ok {
		t.Fatal("ForgetSized dropped a disk total")
	}
}

// TestSizerKeepsFlatRowBoundary: a flat tag view path inside a list row reaches the matcher and
// the sizer with its "//" row boundary, and is cached under it.
func TestSizerKeepsFlatRowBoundary(t *testing.T) {
	t.Parallel()
	e := New()
	const key = "fileindex@(clover):///meadow//orchard/lantern"
	e.SetSubtreeSizer(func(p string) bool { return strings.HasPrefix(p, "fileindex") }, func(_ context.Context, p string) (int64, int64, error) {
		if p != key {
			return 0, 0, fs.ErrNotExist
		}
		return 7, 1, nil
	})
	e.StartScanFromDirectory("fileindex@(clover):///meadow//orchard/", []string{"fileindex@(clover):///meadow//orchard/lantern"}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { _, ok := e.ByteSize(key); return ok && !e.DiskScanBusy() }, 2*time.Second, "flat path sized")
	if got, _ := e.ByteSize(key); got != 7 {
		t.Fatalf("ByteSize = %d, want 7", got)
	}
	if _, ok := e.ByteSize("fileindex@(clover):///meadow/orchard/lantern"); ok {
		t.Fatal("the row-less path is another location and must not share the cached total")
	}
}

// TestNearestAncestorWalksURIKeys: the ancestors of an index key are index keys, up to the root
// (path/filepath would turn "fileindex:///a/b" into "fileindex:/a"), also inside a flat row.
func TestNearestAncestorWalksURIKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ key, have, want string }{
		{"fileindex:///meadow/orchard/lantern", "fileindex:///meadow", "fileindex:///meadow"},
		{"fileindex:///meadow/orchard", "fileindex:///", "fileindex:///"},
		{"fileindex@(clover):///meadow//orchard/lantern", "fileindex@(clover):///meadow", "fileindex@(clover):///meadow"},
		{"fileindex@(clover):///meadow//orchard/lantern", "fileindex@(clover):///meadow//orchard", "fileindex@(clover):///meadow//orchard"},
	} {
		got, ok := nearestAncestor(tc.key, map[string]int{tc.have: 1})
		if !ok || got != tc.want {
			t.Errorf("nearestAncestor(%q) = %q, %v; want %q", tc.key, got, ok, tc.want)
		}
	}
	if nestedInOther("fileindex:///meadow/orchard", map[string]int{"fileindex:///meadow": 1}) != true {
		t.Error("nestedInOther missed the index ancestor")
	}
}
