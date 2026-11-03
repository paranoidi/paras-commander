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

// The painted frame is the placed rect of the spec, so the partial repaint and the full render agree.
func TestFileDialogFrameIsThePlacedSpecRect(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	states := []FileDialogState{
		{Open: true, DialogType: FileDialogRename, RenamePhase: RenamePhaseMain, Fields: []FileDialogField{{Label: "Name", Value: "alpha"}}},
		{Open: true, DialogType: FileDialogMkdir, MkdirShowActions: true, Fields: []FileDialogField{{Label: "Directory name"}}},
		{Open: true, DialogType: FileDialogChown, Fields: []FileDialogField{{Label: "User"}, {Label: "Group"}}},
		{Open: true, DialogType: FileDialogDelete, DeleteSummary: "1 file", DeleteEntries: transferGoldenEntries(3)},
	}
	for _, st := range states {
		got, ok := fileDialogFrame(layout, st, 0)
		p, placed := dlg.Place(layout.Width, layout.Height, FileDialogView(st, FileDialogViewContext{Layout: layout}))
		if !ok || !placed || got != p.Rect {
			t.Errorf("type %d: painted frame = %+v ok=%v, placed %+v", st.DialogType, got, ok, p.Rect)
		}
	}
}

func TestFileDialogDeleteIsAltOnly(t *testing.T) {
	ctx := FileDialogViewContext{Layout: Layout{Width: 80, Height: 24}}
	del := FileDialogState{Open: true, DialogType: FileDialogDelete, DeleteSummary: "1 file", DeleteEntries: transferGoldenEntries(1)}
	if !FileDialogView(del, ctx).AltOnly {
		t.Error("delete dialog: AltOnly not set")
	}
	mk := FileDialogState{Open: true, DialogType: FileDialogMkdir, Fields: []FileDialogField{{Label: "Directory name"}}}
	if FileDialogView(mk, ctx).AltOnly {
		t.Error("mkdir dialog: AltOnly set")
	}
}
