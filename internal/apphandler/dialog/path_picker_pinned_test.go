package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// fakePathPickerHost implements Host with only ActivePanel overridden; the embedded nil Host
// panics if any other method is called, which is fine since PathPickerItemsPinned only needs
// ActivePanel.
type fakePathPickerHost struct {
	Host
	panelPath string
}

func (f fakePathPickerHost) ActivePanel() *panel.State {
	return &panel.State{Path: pathloc.FileMust(f.panelPath)}
}

func TestPathPickerItemsPinnedExcludesFiles(t *testing.T) {
	dir := t.TempDir()
	h := &Handler{
		host: fakePathPickerHost{panelPath: dir},
		model: &ui.Model{
			PinnedItems: []ui.PinnedItem{
				{Path: "/pinned/dir-one", IsDir: true},
				{Path: "/pinned/file.txt", IsDir: false},
				{Path: "/pinned/dir-two", IsDir: true},
			},
		},
	}
	items, err := h.PathPickerItemsPinned()
	if err != nil {
		t.Fatalf("PathPickerItemsPinned: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (files excluded): %+v", len(items), items)
	}
	for _, it := range items {
		if it.Source != "" {
			t.Errorf("item %+v: Source = %q, want empty (redundant with the picker's own Pinned mode)", it, it.Source)
		}
	}
	if items[0].Path != "/pinned/dir-one" || items[1].Path != "/pinned/dir-two" {
		t.Fatalf("got paths %q, %q", items[0].Path, items[1].Path)
	}
}

func TestPathPickerPinnedFooterEligibleTransferOnly(t *testing.T) {
	h := &Handler{model: &ui.Model{}}

	if h.PathPickerPinnedFooterEligible() {
		t.Fatal("no dialog open: want false")
	}

	h.model.FlattenDialog.Open = true
	h.model.FlattenDialog.FocusField = 0
	if h.PathPickerPinnedFooterEligible() {
		t.Fatal("flatten destination focused: want false (transfer-only)")
	}
	h.model.FlattenDialog.Open = false

	h.model.TransferDialog.Open = true
	h.model.TransferDialog.Phase = dialog.TransferPhaseDestination
	h.model.TransferDialog.FocusField = 0
	if !h.PathPickerPinnedFooterEligible() {
		t.Fatal("transfer destination focused: want true")
	}
}
