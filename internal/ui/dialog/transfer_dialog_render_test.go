package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func transferEntriesForTest(n int) []DeleteListEntry {
	entries := make([]DeleteListEntry, n)
	for i := range entries {
		entries[i] = DeleteListEntry{Name: "entry", Path: "/tmp/entry", Type: localfs.EntryFile}
	}
	return entries
}

func TestTransferListViewportRowsSmallListUncapped(t *testing.T) {
	st := TransferDialogState{Kind: TransferKindCopy, Entries: transferEntriesForTest(2)}
	got := TransferListViewportRows(Layout{Height: 40}, st)
	if got != 2 {
		t.Fatalf("viewport = %d, want 2", got)
	}
}

func TestTransferListViewportRowsCapsAtMaxListRows(t *testing.T) {
	st := TransferDialogState{Kind: TransferKindCopy, Entries: transferEntriesForTest(100)}
	got := TransferListViewportRows(Layout{Height: 100}, st)
	if got != TransferListMaxRows {
		t.Fatalf("viewport = %d, want max list cap %d", got, TransferListMaxRows)
	}
}

func TestTransferListViewportRowsShrinksForSmallLayout(t *testing.T) {
	st := TransferDialogState{Kind: TransferKindCopy, Entries: transferEntriesForTest(20)}
	got := TransferListViewportRows(Layout{Height: 10}, st)
	if got < 1 {
		t.Fatalf("viewport = %d, want >= 1", got)
	}
	if got >= TransferListMaxRows {
		t.Fatalf("viewport = %d, want shrunk below max cap %d for small layout", got, TransferListMaxRows)
	}
}

func TestTransferMultiDialogWidthUsesPreferredMinimum(t *testing.T) {
	state := TransferDialogState{Kind: TransferKindCopy, CommonRoot: "/tmp"}
	got := transferMultiDialogWidth(Layout{Width: 80}, state, "", 0)
	if got < PreferredFormDialogWidth {
		t.Fatalf("width = %d, want at least PreferredFormDialogWidth %d", got, PreferredFormDialogWidth)
	}
}

func TestTransferEntryLabelAddsSlashForDirectories(t *testing.T) {
	got := transferEntryLabel(DeleteListEntry{Name: "alpha", Path: "/root/alpha", Type: localfs.EntryDirectory}, false)
	if got != "alpha/" {
		t.Fatalf("label = %q, want %q", got, "alpha/")
	}
	got = transferEntryLabel(DeleteListEntry{Name: "river.txt", Path: "/root/river.txt", Type: localfs.EntryFile}, false)
	if got != "river.txt" {
		t.Fatalf("label = %q, want %q", got, "river.txt")
	}
}

func TestTransferEntryLabelFlattenUsesBasename(t *testing.T) {
	got := transferEntryLabel(DeleteListEntry{Name: "bravo/alpha", Path: "/root/bravo/alpha", Type: localfs.EntryDirectory}, true)
	if got != "alpha/" {
		t.Fatalf("flatten dir label = %q, want %q", got, "alpha/")
	}
	got = transferEntryLabel(DeleteListEntry{Name: "bravo/river.txt", Path: "/root/bravo/river.txt", Type: localfs.EntryFile}, true)
	if got != "river.txt" {
		t.Fatalf("flatten file label = %q, want %q", got, "river.txt")
	}
}
