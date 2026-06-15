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
	_, _, child, _ := BuildColumns(state, 10, false, false)
	if !child.Populated || child.Snapshot.Path.String() != maple {
		t.Fatalf("coalesced child = %+v populated=%v, want cached maple until flush", child.Snapshot, child.Populated)
	}

	state.CarouselChildPreviewCoalesce = false
	_, _, fresh, _ := BuildColumns(state, 10, false, false)
	if !fresh.Populated || fresh.Snapshot.Path.String() != oak {
		t.Fatalf("fresh child = %+v populated=%v, want oak from disk", fresh.Snapshot, fresh.Populated)
	}
}

func TestShowChildPreviewColumnForFileWhenEligible(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "ledger.txt")
	if err := os.WriteFile(ledger, []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.CarouselCenterHasSubdirectories() {
		t.Fatal("fixture should be files only")
	}
	if !state.SelectVisibleEntry("ledger.txt") {
		t.Fatal("ledger.txt not found")
	}
	if ShowChildPreviewColumn(state, false, false) {
		t.Fatal("want no child column when file preview not eligible")
	}
	if !ShowChildPreviewColumn(state, false, true) {
		t.Fatal("want child column for file cursor when file preview eligible")
	}
	if ChildPreviewKindFor(state, false, true) != ChildPreviewFile {
		t.Fatal("want file preview kind")
	}
}
