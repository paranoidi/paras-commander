package dialog

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"

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

func TestPathPickerHostFooterEligibleHosts(t *testing.T) {
	h := &Handler{model: &ui.Model{}}
	if h.PathPickerHostFooterEligible() {
		t.Fatal("no dialog open: want false")
	}
	h.model.FlattenDialog.Open = true
	if !h.PathPickerHostFooterEligible() {
		t.Fatal("flatten destination focused: want true")
	}
	h.model.FlattenDialog.Open = false
	h.model.TransferDialog.Open = true
	h.model.TransferDialog.Phase = dialog.TransferPhaseDestination
	if !h.PathPickerHostFooterEligible() {
		t.Fatal("transfer destination focused: want true")
	}
}

func TestPinDialogKeyOpensPinnedPickerInFlattenAndFileField(t *testing.T) {
	setup := func(t *testing.T) (*Handler, *tcell.EventKey) {
		h, host := newBookmarkTestHandler(t)
		h.host = allPickerHost{bookmarkTestHost: host, inactive: &panel.State{Path: pathloc.FileMust(host.active.PathString())}}
		bundle, err := keymap.DefaultBundle()
		if err != nil {
			t.Fatal(err)
		}
		h.keysGlobal = bundle.Global
		h.keysDialogInput = bundle.DialogInput
		h.model.PinnedItems = []ui.PinnedItem{{Path: filepath.Join(host.active.PathString(), "orchard"), IsDir: true}}
		ev, ok := bundle.Global.FirstEventKeyForAction(keymap.ActionPanelPinDialog)
		if !ok {
			t.Fatal("no pin-dialog binding")
		}
		return h, ev
	}

	t.Run("flatten", func(t *testing.T) {
		h, ev := setup(t)
		h.model.FlattenDialog.Open = true
		if !h.TryPathPickerHostShortcut(ev) {
			t.Fatal("shortcut not handled")
		}
		st := h.model.PathPicker
		if !st.Open || st.Title != "Pinned" || st.Purpose != dialog.PathPickerPurposeApplyFlattenDestination {
			t.Fatalf("picker = %+v", st)
		}
	})
	t.Run("symlink field", func(t *testing.T) {
		h, ev := setup(t)
		h.model.FileDialog = dialog.FileDialogState{
			Open:       true,
			DialogType: dialog.FileDialogSymlink,
			Fields:     []dialog.FileDialogField{{Label: "Target", PathPicker: true}, {Label: "Link path", PathPicker: true}},
		}
		h.model.FileDialog.FocusedField = 1
		if !h.TryPathPickerHostShortcut(ev) {
			t.Fatal("shortcut not handled")
		}
		st := h.model.PathPicker
		if !st.Open || st.Title != "Pinned" || st.Purpose != dialog.PathPickerPurposeApplyFileDialogField || st.FileFieldIndex != 1 {
			t.Fatalf("picker = %+v", st)
		}
	})
}
