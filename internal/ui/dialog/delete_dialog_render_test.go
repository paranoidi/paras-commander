package dialog

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestDeleteDialogListViewportRowsSmallListUncapped(t *testing.T) {
	state := FileDialogState{
		DeleteSummary: "2 files (300 B)",
		DeleteEntries: []DeleteListEntry{
			{Name: "a.txt", Type: localfs.EntryFile},
			{Name: "b.txt", Type: localfs.EntryFile},
		},
	}
	got := DeleteDialogListViewportRows(24, state)
	if got != 2 {
		t.Fatalf("viewport = %d, want 2", got)
	}
}

func TestDeleteDialogListViewportRowsCapsAtMaxListRows(t *testing.T) {
	entries := make([]DeleteListEntry, 100)
	for i := range entries {
		entries[i] = DeleteListEntry{Name: "file.txt", Type: localfs.EntryFile}
	}
	state := FileDialogState{
		DeleteSummary: "100 files (1 KiB)",
		DeleteEntries: entries,
	}
	const layoutH = 100
	got := DeleteDialogListViewportRows(layoutH, state)
	if got != DeleteDialogMaxListRows {
		t.Fatalf("viewport = %d, want max list cap %d", got, DeleteDialogMaxListRows)
	}
}

func TestDeleteDialogListViewportRowsCapsAt80Percent(t *testing.T) {
	entries := make([]DeleteListEntry, 30)
	for i := range entries {
		entries[i] = DeleteListEntry{Name: "file-" + strings.Repeat("x", 4) + ".txt", Type: localfs.EntryFile}
	}
	state := FileDialogState{
		DeleteSummary: "30 files (1.2 GiB)",
		DeleteEntries: entries,
	}
	const layoutH = 20
	maxH := layoutH * 80 / 100 // 16
	got := DeleteDialogListViewportRows(layoutH, state)
	want := maxH - 6 // footer summary block + sep/buttons chrome
	if got != want {
		t.Fatalf("viewport = %d, want %d", got, want)
	}
	height := fileDeleteDialogHeight(layoutH, state)
	if height > maxH {
		t.Fatalf("height = %d, exceeds 80%% max %d", height, maxH)
	}
	if height != 6+got {
		t.Fatalf("height = %d, want %d", height, 6+got)
	}
}

func TestDeleteDialogListViewportRowsWithWarning(t *testing.T) {
	entries := make([]DeleteListEntry, 20)
	for i := range entries {
		entries[i] = DeleteListEntry{Name: "dir", Type: localfs.EntryDirectory}
	}
	state := FileDialogState{
		DeleteSummary: "842 files (1.2 GiB)",
		DeleteWarning: "legacy warning line still reserves a row",
		DeleteEntries: entries,
	}
	const layoutH = 20
	maxH := layoutH * 80 / 100
	got := DeleteDialogListViewportRows(layoutH, state)
	want := maxH - 6 - 1
	if got != want {
		t.Fatalf("viewport = %d, want %d", got, want)
	}
}

func TestDeleteEnsureListScroll(t *testing.T) {
	st := FileDialogState{DeleteListScroll: 100}
	DeleteEnsureListScroll(&st, 5, 10)
	if st.DeleteListScroll != 5 {
		t.Fatalf("scroll = %d, want 5", st.DeleteListScroll)
	}
}
