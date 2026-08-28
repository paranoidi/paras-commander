package panelcarousel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// primeChildCacheForTest simulates the async child-snapshot apply (internal/app's
// applyCarouselSnapshot) so BuildColumns/SnapshotChild's pure-cache-read behavior can be tested
// without a real async round trip — SnapshotChild never touches the filesystem itself anymore.
func primeChildCacheForTest(t *testing.T, state *panel.State, target string, viewportRows int) {
	t.Helper()
	loc, err := pathloc.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	selectedName, indexFallback, centerRecalled := state.RecalledCursorFor(target)
	entries, listingLoc, _, _, err := panel.FetchListing(context.Background(), state.ListingRefreshSnapshot(loc, 0))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := state.BuildListingSnapshotFromEntries(listingLoc, entries, selectedName, indexFallback, viewportRows, centerRecalled)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselSideCache.Child = snap
	state.CarouselSideCache.ChildOK = true
	state.CarouselSideCache.ChildCursorDir = target
}

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
	primeChildCacheForTest(t, &state, maple, 10)
	if _, ok := state.SnapshotChild(); !ok {
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

	// Once coalesce ends, BuildColumns never re-reads the disk itself — it is a pure cache read.
	// The cursor moved to oak but the fetch for oak hasn't landed yet, so the column keeps showing
	// the last cached (now stale) maple listing rather than flashing blank, until an async fetch
	// (dispatched a layer up, in internal/apphandler/preview) lands.
	state.CarouselChildPreviewCoalesce = false
	_, _, stale, _ := BuildColumns(state, 10, false, false)
	if !stale.Populated || stale.Snapshot.Path.String() != maple {
		t.Fatalf("child = %+v populated=%v, want still-cached maple until the oak fetch lands", stale.Snapshot, stale.Populated)
	}

	primeChildCacheForTest(t, &state, oak, 10)
	_, _, fresh, _ := BuildColumns(state, 10, false, false)
	if !fresh.Populated || fresh.Snapshot.Path.String() != oak {
		t.Fatalf("fresh child = %+v populated=%v, want oak once the async fetch cache lands", fresh.Snapshot, fresh.Populated)
	}
}

// TestBuildColumnsPaintsStaleParentForContinuity: once the center directory changes, the parent
// cache (still tagged for the previous directory) keeps being painted so the column geometry stays
// exactly as the previous frame drew it until the async fetch lands. Dropping it here instead would
// leave MeasureFitColumnWidths unmeasured for the parent, and resolveWidths falls back to the
// configured cap when a fit column is unmeasured — ballooning the parent to its cap and shoving the
// center column across the panel for one frame.
func TestBuildColumnsPaintsStaleParentForContinuity(t *testing.T) {
	root := t.TempDir()
	walnut := filepath.Join(root, "walnut")
	oak := filepath.Join(root, "oak")
	for _, dir := range []string{walnut, oak} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// chdir to oak without the parent cache having caught up yet (async fetch still in flight).
	state, err := panel.New(oak)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselSideCache.Parent = panel.ListingSnapshot{Entries: fitTestWordEntries()}
	state.CarouselSideCache.ParentOK = true
	state.CarouselSideCache.ParentSourceDir = walnut // stale: still tagged for the old directory

	parent, _, _, _ := BuildColumns(state, 10, false, false)
	if !parent.Populated {
		t.Fatal("stale parent should still be populated (painted for continuity)")
	}

	layout := DefaultLayout()
	layout.Splits[0] = ColumnSplitSpec{Kind: SplitFitChars, Value: 64}
	if got := MeasureFitColumnWidths(layout, parent, state, false, true, uiscrollbar.StyleThumb, 10); got[0] == 0 {
		t.Fatal("stale parent must still be measured; an unmeasured fit column resolves to its cap")
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
