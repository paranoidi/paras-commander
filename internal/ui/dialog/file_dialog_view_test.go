package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestFileFieldIDRoundTrip(t *testing.T) {
	for i := range 3 {
		if got, ok := FileFieldIndex(FileFieldID(i)); !ok || got != i {
			t.Fatalf("FileFieldIndex(FileFieldID(%d)) = %d,%v", i, got, ok)
		}
	}
	if _, ok := FileFieldIndex(dlg.OK); ok {
		t.Fatal("a button is not a field")
	}
}

func TestFileDialogApply(t *testing.T) {
	var st FileDialogState
	st.Apply(dlg.Event{ID: MkdirActionID(MkdirActionCreateMoveSelect)})
	st.Apply(dlg.Event{ID: FileRenameFocusAfter})
	st.Apply(dlg.Event{ID: FileSanitizeDots})
	st.Apply(dlg.Event{ID: FileSlugifyUnderscore})
	st.Apply(dlg.Event{ID: renameEncodingID(2)})
	if st.MkdirAction != MkdirActionCreateMoveSelect || !st.RenameFocusAfter || !st.RenameSanitizeDots ||
		st.RenameSlugifySep != RenameSlugifyUnderscore || st.RenameEncodingSelected != 2 {
		t.Fatalf("Apply did not fold the events: %+v", st)
	}
}

// FileDialogRect is the placed rect of the spec, so the overlay repaint and the full render agree.
func TestFileDialogRectIsThePlacedSpecRect(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	states := []FileDialogState{
		{Open: true, DialogType: FileDialogRename, RenamePhase: RenamePhaseMain, Fields: []FileDialogField{{Label: "Name", Value: "alpha"}}},
		{Open: true, DialogType: FileDialogMkdir, MkdirShowActions: true, Fields: []FileDialogField{{Label: "Directory name"}}},
		{Open: true, DialogType: FileDialogChown, Fields: []FileDialogField{{Label: "User"}, {Label: "Group"}}},
		{Open: true, DialogType: FileDialogDelete, DeleteSummary: "1 file", DeleteEntries: transferGoldenEntries(3)},
	}
	for _, st := range states {
		got, ok := FileDialogRect(layout, st, 0)
		p, placed := dlg.Place(layout.Width, layout.Height, FileDialogView(st, FileDialogViewContext{Layout: layout}))
		if !ok || !placed || got != p.Rect {
			t.Errorf("type %d: FileDialogRect = %+v ok=%v, placed %+v", st.DialogType, got, ok, p.Rect)
		}
	}
}
