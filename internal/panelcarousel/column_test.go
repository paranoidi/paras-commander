package panelcarousel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestBuildColumnsSkipsChildDiskReadDuringCoalesce(t *testing.T) {
	root := t.TempDir()
	maple := filepath.Join(root, "maple")
	oak := filepath.Join(root, "oak")
	for _, dir := range []string{maple, oak} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	if _, ok := state.SnapshotChild(10); !ok {
		t.Fatal("initial SnapshotChild = false, want maple child")
	}
	cached := state.CarouselSideCache.Child
	if !state.CarouselSideCache.ChildOK || cached.Path.String() != maple {
		t.Fatalf("cache = %+v ok=%v, want maple", cached, state.CarouselSideCache.ChildOK)
	}

	if !state.SelectVisibleEntry("oak") {
		t.Fatal("oak not found")
	}
	state.CarouselChildPreviewCoalesce = true
	_, _, child := BuildColumns(state, 10, false)
	if !child.Populated || child.Snapshot.Path.String() != maple {
		t.Fatalf("coalesced child = %+v populated=%v, want cached maple until flush", child.Snapshot, child.Populated)
	}

	state.CarouselChildPreviewCoalesce = false
	_, _, fresh := BuildColumns(state, 10, false)
	if !fresh.Populated || fresh.Snapshot.Path.String() != oak {
		t.Fatalf("fresh child = %+v populated=%v, want oak from disk", fresh.Snapshot, fresh.Populated)
	}
}
