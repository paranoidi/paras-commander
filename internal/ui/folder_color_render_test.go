package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestFolderColorTintsNameAndIconButNotCursorRow(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 10
	screen.SetSize(width, height)

	root := "/vol"
	tint := tcell.NewRGBColor(0x12, 0x34, 0x56)
	state := panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "meadow", Path: root + "/meadow", Type: localfs.EntryDirectory},
			{Name: "harbor", Path: root + "/harbor", Type: localfs.EntryDirectory},
			{Name: "plain", Path: root + "/plain", Type: localfs.EntryDirectory},
		},
		Cursor: 1,
	}
	ctx := PanelContext{
		PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true, ActivePanel: PrimaryPanel,
		SyncDriverPanelID: -1, QuickViewDriverPanelID: -1,
		FolderColor: func(p string) (tcell.Color, bool) { return tint, p != root+"/plain" },
	}
	drawPanel(screen, Rect{Width: width, Height: height}, state,
		PanelStyleConfig{Styles: theme.Default()}, ctx,
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	tinted := func(y int) (n int) {
		for x := 1; x < width-1; x++ {
			if _, st, _ := screen.Get(x, y); st != tcell.StyleDefault {
				if fg, _, _ := st.Decompose(); fg == tint {
					n++
				}
			}
		}
		return n
	}
	// Rows start at y=2 (see the quick-view indicator test): meadow, harbor (cursor), plain.
	if n := tinted(2); n < 2 { // icon + at least part of the name
		t.Fatalf("coloured row has %d tinted cells, want name and icon", n)
	}
	if n := tinted(3); n != 0 {
		t.Fatalf("cursor row kept the folder colour on %d cells", n)
	}
	if n := tinted(4); n != 0 {
		t.Fatalf("uncoloured row has %d tinted cells", n)
	}
	ctx.ChromeBlocked = true
	drawPanel(screen, Rect{Width: width, Height: height}, state,
		PanelStyleConfig{Styles: theme.Default()}, ctx,
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})
	if n := tinted(2); n != 0 {
		t.Fatalf("blocked chrome still tinted %d cells", n)
	}
}
