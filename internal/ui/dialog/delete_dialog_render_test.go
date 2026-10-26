package dialog

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestFileDialogOuterTitleDanglingDirsDelete(t *testing.T) {
	single := FileDialogState{
		DialogType:         FileDialogDelete,
		DeleteDanglingDirs: true,
		DeleteEntries:      []DeleteListEntry{{Name: "empty"}},
	}
	if title := fileDialogOuterTitle(single); title != "Delete empty leftover dir?" {
		t.Fatalf("single title = %q, want Delete empty leftover dir?", title)
	}
	multi := FileDialogState{
		DialogType:         FileDialogDelete,
		DeleteDanglingDirs: true,
		DeleteEntries: []DeleteListEntry{
			{Name: "a"},
			{Name: "b"},
		},
	}
	if title := fileDialogOuterTitle(multi); title != "Delete empty leftover dirs?" {
		t.Fatalf("multi title = %q, want Delete empty leftover dirs?", title)
	}
}

// deleteListRows is the delete list's height in the dialog fitted to an 80 x layoutH screen.
func deleteListRows(t *testing.T, layoutH int, state FileDialogState) int {
	t.Helper()
	state.DialogType = FileDialogDelete
	list, ok := FileDialogView(state, FileDialogViewContext{Layout: Layout{Width: 80, Height: layoutH}}).Item(fileDeleteList)
	if !ok {
		t.Fatal("no delete list")
	}
	return list.Height
}

func TestDeleteListHeightSmallListUncapped(t *testing.T) {
	state := FileDialogState{
		DeleteSummary: "2 files (300 B)",
		DeleteEntries: []DeleteListEntry{
			{Name: "a.txt", Type: localfs.EntryFile},
			{Name: "b.txt", Type: localfs.EntryFile},
		},
	}
	if got := deleteListRows(t, 24, state); got != 2 {
		t.Fatalf("list height = %d, want 2", got)
	}
}

func TestDeleteListHeightCapsAtMaxListRows(t *testing.T) {
	entries := make([]DeleteListEntry, 100)
	for i := range entries {
		entries[i] = DeleteListEntry{Name: "file.txt", Type: localfs.EntryFile}
	}
	state := FileDialogState{
		DeleteSummary: "100 files (1 KiB)",
		DeleteEntries: entries,
	}
	if got := deleteListRows(t, 100, state); got != DeleteDialogMaxListRows {
		t.Fatalf("list height = %d, want max list cap %d", got, DeleteDialogMaxListRows)
	}
}

// The list fills the dialog up to 80% of the screen: the dialog is exactly that tall, the list
// what is left after the summary (and warning), the button strip and the borders.
func TestDeleteListHeightCapsAt80Percent(t *testing.T) {
	for _, warning := range []string{"", "legacy warning line still reserves a row"} {
		entries := make([]DeleteListEntry, 30)
		for i := range entries {
			entries[i] = DeleteListEntry{Name: "file-" + strings.Repeat("x", 4) + ".txt", Type: localfs.EntryFile}
		}
		state := FileDialogState{
			DialogType:    FileDialogDelete,
			DeleteSummary: "30 files (1.2 GiB)",
			DeleteWarning: warning,
			DeleteEntries: entries,
		}
		const layoutH = 20
		maxH := layoutH * 80 / 100 // 16
		spec := FileDialogView(state, FileDialogViewContext{Layout: Layout{Width: 80, Height: layoutH}})
		p, ok := dlg.Place(80, layoutH, spec)
		if !ok {
			t.Fatal("delete dialog does not fit")
		}
		if p.Rect.Height != maxH {
			t.Fatalf("warning %q: height = %d, want the 80%% max %d", warning, p.Rect.Height, maxH)
		}
		list, _ := spec.Item(fileDeleteList)
		placed, _ := p.Item(fileDeleteList)
		if placed.Rect.Height != list.Height {
			t.Fatalf("warning %q: drawn list height %d, spec says %d", warning, placed.Rect.Height, list.Height)
		}
		if got := deleteListRows(t, layoutH, state); placed.Rect.Y+got != p.SepY-2-min(len(warning), 1) {
			t.Fatalf("warning %q: list of %d rows does not end above the summary", warning, got)
		}
	}
}
