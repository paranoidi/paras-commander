package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// primeChildCacheForTest simulates the async child-snapshot apply (internal/app's
// applyCarouselSnapshot) so carousel rendering can be tested against a warm cache without a real
// async round trip — SnapshotChild never touches the filesystem itself anymore.
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

// Regression: ui.Render clears the full screen each frame, so carousel child preview must be
// repainted from cache during coalesce — skipping the child column leaves it blank.
func TestCarouselCoalesceRepaintsCachedChildAfterFullScreenClear(t *testing.T) {
	root := t.TempDir()
	childDir := filepath.Join(root, "walnut")
	if err := os.Mkdir(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childDir, "acorn.log"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("walnut") {
		t.Fatal("walnut not found")
	}
	primeChildCacheForTest(t, &state, childDir, 10)
	if _, ok := state.SnapshotChild(); !ok {
		t.Fatal("SnapshotChild = false, want child preview cached")
	}
	state.CarouselMode = true
	state.CarouselChildPreviewCoalesce = true

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 100, 20
	screen.SetSize(width, height)

	// Same full-frame clear as ui.Render before panels are drawn.
	primitive.Fill(screen, primitive.Rect{Width: width, Height: height}, ' ', tcell.StyleDefault)

	rect := Rect{X: 0, Y: 1, Width: width, Height: height - 3}
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	cols := panelcarousel.SplitColumns(rect, true, panelcarousel.DefaultLayout(), [3]int{})
	childCol := cols[2]
	headerY := rect.Y + 1
	foundHeader := false
	for x := childCol.X; x < childCol.X+childCol.Width; x++ {
		ch, _, _ := screen.Get(x, headerY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == 'N' || r == 'n' { // "Name" column header
			foundHeader = true
			break
		}
	}
	if !foundHeader {
		t.Fatal("child column header not painted after full-screen clear")
	}

	listY := childCol.Y
	foundEntry := false
	for x := childCol.X; x < childCol.X+childCol.Width; x++ {
		ch, _, _ := screen.Get(x, listY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == 'a' { // acorn.log
			foundEntry = true
			break
		}
	}
	if !foundEntry {
		t.Fatal("cached child listing not repainted after full-screen clear during coalesce")
	}
}

func TestDrawCarouselFilePreviewDuringQuickFilter(t *testing.T) {
	root := t.TempDir()
	scroll := filepath.Join(root, "scroll.txt")
	if err := os.WriteFile(scroll, []byte("river delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselMode = true
	if !state.SelectVisibleEntry("scroll.txt") {
		t.Fatal("scroll.txt not found")
	}
	visibleRows := PanelListRows(Rect{X: 0, Y: 1, Width: 100, Height: 20})
	state.OpenFilter(visibleRows)
	state.AppendFilterRune('n', visibleRows)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 100, 20
	screen.SetSize(width, height)

	rect := Rect{X: 0, Y: 1, Width: width, Height: height - 3}
	styles := theme.Default()
	preview := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		TitleBase:    "scroll.txt",
		Path:         scroll,
		CombinedText: "river delta\n",
	}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1, SelectionsBottomHint: true},
		PanelDisplayConfig{ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout(), CarouselFilePreview: preview})

	cols := panelcarousel.SplitColumns(rect, true, panelcarousel.DefaultLayout(), [3]int{})
	childCol := cols[2]
	found := false
	for y := childCol.Y; y < childCol.Y+childCol.Height; y++ {
		for x := childCol.X; x < childCol.X+childCol.Width; x++ {
			ch, _, _ := screen.Get(x, y)
			r, _ := utf8.DecodeRuneInString(ch)
			if r == 'r' {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("carousel child preview body not painted while quick filter is active")
	}
}
