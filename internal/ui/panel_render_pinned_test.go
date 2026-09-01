package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawPanelRowShowsPinGlyphForPinnedEntry(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 10
	screen.SetSize(width, height)

	styles := theme.Default()

	root := "/vol"
	pinnedPath := root + "/pinned.txt"
	otherPath := root + "/other.txt"
	state := panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "pinned.txt", Path: pinnedPath, Type: localfs.EntryFile},
			{Name: "other.txt", Path: otherPath, Type: localfs.EntryFile},
		},
	}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{
			ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout(),
			PinnedPaths: map[string]struct{}{pinnedPath: {}},
		})

	wantGlyph := []rune(styles.SymbolPin())[0]
	rowHasGlyph := func(rowY int) bool {
		for col := rect.X + 1; col < rect.X+rect.Width-1; col++ {
			ch, _, _ := screen.Get(col, rowY)
			r, _ := utf8.DecodeRuneInString(ch)
			if r == wantGlyph {
				return true
			}
		}
		return false
	}

	pinnedRowY := rect.Y + 2
	otherRowY := rect.Y + 3
	if !rowHasGlyph(pinnedRowY) {
		t.Fatal("pin glyph not found on pinned entry's row")
	}
	if rowHasGlyph(otherRowY) {
		t.Fatal("pin glyph unexpectedly found on non-pinned entry's row")
	}
}

func TestPinnedPathSet(t *testing.T) {
	if got := PinnedPathSet(nil); got != nil {
		t.Fatalf("PinnedPathSet(nil) = %v, want nil", got)
	}
	if got := PinnedPathSet([]PinnedItem{}); got != nil {
		t.Fatalf("PinnedPathSet(empty) = %v, want nil", got)
	}
	items := []PinnedItem{
		{Path: "/a/b", IsDir: false},
		{Path: "/a/c", IsDir: true},
	}
	got := PinnedPathSet(items)
	if len(got) != 2 {
		t.Fatalf("PinnedPathSet len = %d, want 2", len(got))
	}
	for _, it := range items {
		if _, ok := got[it.Path]; !ok {
			t.Fatalf("PinnedPathSet missing %q", it.Path)
		}
	}
}
