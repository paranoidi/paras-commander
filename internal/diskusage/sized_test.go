package diskusage

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestCleanKeyKeepsSchemePrefix(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"fileindex:///meadow/":          "fileindex:///meadow",
		"fileindex:///meadow/../clover": "fileindex:///clover",
		"fileindex:///":                 "fileindex:///",
		"/harbor//lantern/":             "/harbor/lantern",
		"/harbor/a://b":                 "/harbor/a:/b",
	} {
		if got := CleanKey(in); got != want {
			t.Errorf("CleanKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSizerTotalsPrefixedRootsAndOrigin: roots under the sizer prefix are sized by the sizer
// (no disk walk, a gone path stays uncached), the origin gets the sizer's own total, and
// ForgetSized drops only sized keys.
func TestSizerTotalsPrefixedRootsAndOrigin(t *testing.T) {
	t.Parallel()
	e := New()
	totals := map[string]int64{"idx:///w": 30, "idx:///w/a": 10, "idx:///w/b": 20}
	e.SetSubtreeSizer(func(p string) bool { return strings.HasPrefix(p, "idx://") }, func(_ context.Context, p string) (int64, int64, error) {
		n, ok := totals[p]
		if !ok {
			return 0, 0, fs.ErrNotExist
		}
		return n, n / 10, nil
	})
	e.SetSize("/local", 1, 5)

	e.StartScanFromDirectory("idx:///w/", []string{"idx:///w/a", "idx:///w/b/", "idx:///w/gone"}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { _, ok := e.ByteSize("idx:///w"); return ok && !e.DiskScanBusy() }, 2*time.Second, "origin sized")
	for p, want := range totals {
		if got, ok := e.ByteSize(p); !ok || got != want {
			t.Fatalf("ByteSize(%s) = %d, %v; want %d", p, got, ok, want)
		}
		if got, _ := e.FileCount(p); got != want/10 {
			t.Fatalf("FileCount(%s) = %d, want %d", p, got, want/10)
		}
	}
	if _, ok := e.ByteSize("idx:///w/gone"); ok {
		t.Fatal("a path the sizer lacks must stay uncached")
	}
	if e.DiskScanExcluded("idx:///w/a", false, 1, true, func(string) bool { return true }) {
		t.Fatal("sized paths are never excluded by disk rules")
	}

	e.ForgetSized()
	if _, ok := e.ByteSize("idx:///w/a"); ok {
		t.Fatal("ForgetSized kept a sized total")
	}
	if _, ok := e.ByteSize("/local"); !ok {
		t.Fatal("ForgetSized dropped a disk total")
	}
}

// TestSizerMatchSeesCleanedFlatPath: a flat tag view path inside a list row is sized although
// CleanKey collapses its "//" row boundary.
func TestSizerMatchSeesCleanedFlatPath(t *testing.T) {
	t.Parallel()
	e := New()
	const key = "fileindex@(clover):///meadow/orchard/lantern"
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
}
