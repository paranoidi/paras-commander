package fsvol_test

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsvol"
)

func TestVolumeBytesWritableDir(t *testing.T) {
	dir := t.TempDir()
	avail, total, ok := fsvol.VolumeBytes(dir)
	if !ok {
		t.Skip("volume space unavailable on this platform or permissions")
	}
	if total == 0 {
		t.Fatal("total = 0 with ok true")
	}
	if avail > total {
		t.Fatalf("avail %d > total %d", avail, total)
	}
}
