package fsbackend

import (
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestHasDotfileNames(t *testing.T) {
	t.Parallel()
	if HasDotfileNames(nil) {
		t.Fatal("nil entries: want false")
	}
	if HasDotfileNames([]Entry{{Name: "visible.txt"}}) {
		t.Fatal("no dotfiles: want false")
	}
	if !HasDotfileNames([]Entry{{Name: ".hidden"}}) {
		t.Fatal("dotfile present: want true")
	}
}

func TestPanelEntryConversionKeepsFileColumnFacts(t *testing.T) {
	stamp := time.Unix(1700000000, 0)
	in := localfs.Entry{
		Name: "orchard.txt", Path: "/tmp/orchard.txt", Owner: "wren", Group: "heron", Links: 3,
		ChangedAt: stamp, AccessedAt: stamp.Add(time.Minute), CreatedAt: stamp.Add(time.Hour),
	}
	out := ToPanelEntry(FromPanelEntry(in))
	if out.Owner != in.Owner || out.Group != in.Group || out.Links != in.Links ||
		!out.ChangedAt.Equal(in.ChangedAt) || !out.AccessedAt.Equal(in.AccessedAt) || !out.CreatedAt.Equal(in.CreatedAt) {
		t.Fatalf("round trip = %+v", out)
	}
}
