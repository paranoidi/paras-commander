package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
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

func multiLocationWideState(dest string) TransferDialogState {
	root := "/home/user/" + strings.Repeat("meadow-", 10) + "thicket"
	entry := strings.Repeat("harborlantern", 8) + ".txt"
	return TransferDialogState{
		Open:        true,
		Kind:        TransferKindCopy,
		Phase:       TransferPhaseDestination,
		CommonRoot:  root,
		Destination: FileDialogField{Value: dest, Cursor: len([]rune(dest))},
		Entries: []DeleteListEntry{
			{Name: entry, Path: root + "/" + entry, Type: localfs.EntryFile},
		},
	}
}

func TestTransferDestinationTextWidthPlainUsesPreferredWidth(t *testing.T) {
	layout := Layout{Width: 120, Height: 24}
	state := TransferDialogState{Kind: TransferKindCopy, Phase: TransferPhaseDestination}
	got := TransferDestinationTextWidth(layout, state, "", 0)
	want := PreferredFormDialogWidth - 4 - 2
	if got != want {
		t.Fatalf("plain transfer text width = %d, want %d", got, want)
	}
}

func TestTransferDestinationTextWidthUsesMultiLocationPaintedWidth(t *testing.T) {
	layout := Layout{Width: 120, Height: 40}
	preferredText := PreferredFormDialogWidth - 4 - 2
	// Longer than the preferred-width text row, shorter than the expanded multi-location row.
	dest := "/home/user/" + strings.Repeat("x", preferredText)
	state := multiLocationWideState(dest)
	painted := TransferDestinationTextWidth(layout, state, "", 0)
	if painted <= preferredText {
		t.Fatalf("painted width %d, want greater than preferred %d", painted, preferredText)
	}
	destLen := len([]rune(dest))
	if destLen <= preferredText {
		t.Fatalf("dest len %d should exceed preferred %d", destLen, preferredText)
	}
	if destLen > painted {
		t.Fatalf("dest len %d should fit painted %d", destLen, painted)
	}
	_, scroll := EnsurePathInputScroll(destLen, destLen, 0, painted, 0)
	if scroll != 0 {
		t.Fatalf("scroll = %d, want 0 when dest fits painted multi-location width", scroll)
	}
	_, prefScroll := EnsurePathInputScroll(destLen, destLen, 0, preferredText, 0)
	if prefScroll == 0 {
		t.Fatal("preferred width unexpectedly fits dest (test setup)")
	}
}

func TestTransferMultiLocationDestinationRendersWithoutScrollWhenExpandedFits(t *testing.T) {
	const w, h = 120, 40
	layout := Layout{Width: w, Height: h}
	preferredText := PreferredFormDialogWidth - 4 - 2
	dest := "/home/user/" + strings.Repeat("x", preferredText)
	state := multiLocationWideState(dest)
	_, scroll := EnsurePathInputScroll(len([]rune(dest)), len([]rune(dest)), 0, TransferDestinationTextWidth(layout, state, "", 0), 0)
	state.Destination.Scroll = scroll

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)
	DrawTransferDialog(screen, layout, state, DialogRenderContext{Styles: theme.Default()}, nil)

	found := false
	for y := 0; y < h; y++ {
		row := tcelltest.TextAt(screen, 0, y, w)
		if !strings.Contains(row, dest) {
			continue
		}
		found = true
		if strings.Contains(row, string(draw.ScrollOverflowLeft)) || strings.Contains(row, string(draw.ScrollOverflowRight)) {
			t.Fatalf("destination row scrolled or overflow-marked despite fitting expanded width: %q", strings.TrimSpace(row))
		}
	}
	if !found {
		t.Fatal("destination path not found in multi-location dialog")
	}
}
