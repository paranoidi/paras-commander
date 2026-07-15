package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestAmbiguousListViewportRowsSmallListUncapped(t *testing.T) {
	got := AmbiguousListViewportRows(Layout{Height: 24}, 2)
	if got != 2 {
		t.Fatalf("viewport = %d, want 2", got)
	}
}

func TestAmbiguousListViewportRowsCapsAtMaxListRows(t *testing.T) {
	got := AmbiguousListViewportRows(Layout{Height: 100}, 100)
	if got != AmbiguousListMaxRows {
		t.Fatalf("viewport = %d, want max list cap %d", got, AmbiguousListMaxRows)
	}
}

func TestAmbiguousListViewportRowsShrinksForSmallLayout(t *testing.T) {
	got := AmbiguousListViewportRows(Layout{Height: 10}, 20)
	if got < 1 {
		t.Fatalf("viewport = %d, want >= 1", got)
	}
	if got >= AmbiguousListMaxRows {
		t.Fatalf("viewport = %d, want shrunk below max cap %d for small layout", got, AmbiguousListMaxRows)
	}
}

func TestAmbiguousTransferDialogTitle(t *testing.T) {
	if got := ambiguousTransferDialogTitle(TransferKindCopy); got != "Confirm ambiguous copy?" {
		t.Fatalf("copy title = %q, want %q", got, "Confirm ambiguous copy?")
	}
	if got := ambiguousTransferDialogTitle(TransferKindMove); got != "Confirm ambiguous move?" {
		t.Fatalf("move title = %q, want %q", got, "Confirm ambiguous move?")
	}
}

func TestAmbiguousDialogWidthUsesPreferredMinimum(t *testing.T) {
	state := AmbiguousTransferState{Kind: TransferKindCopy, CommonRoot: "/tmp"}
	got := ambiguousDialogWidth(Layout{Width: 80}, state, "", 0)
	if got < PreferredFormDialogWidth {
		t.Fatalf("width = %d, want at least PreferredFormDialogWidth %d", got, PreferredFormDialogWidth)
	}
}

func TestAmbiguousEntryLabelAddsSlashForDirectories(t *testing.T) {
	got := ambiguousEntryLabel(DeleteListEntry{Name: "alpha", Type: localfs.EntryDirectory})
	if got != "alpha/" {
		t.Fatalf("label = %q, want %q", got, "alpha/")
	}
	got = ambiguousEntryLabel(DeleteListEntry{Name: "river.txt", Type: localfs.EntryFile})
	if got != "river.txt" {
		t.Fatalf("label = %q, want %q", got, "river.txt")
	}
}
