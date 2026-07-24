package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawPanelInfoColumnUsesPanelRowInfoFG(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	const width, height = 48, 10
	screen.SetSize(width, height)

	styles := theme.Default()
	wantInfoFG, _, _ := styles.PanelRowInfo.Decompose()
	wantFileFG, _, _ := styles.PanelRowFile.Decompose()
	if wantInfoFG == wantFileFG {
		t.Fatal("test requires distinct panel.row.info and panel.row.file foregrounds")
	}

	state := panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "harbor", Path: "/tmp/harbor", Type: localfs.EntryFile, Size: 5000},
		},
		ListFormat: panel.ListFormatBrief,
		Cursor:     1, // non-cursor row keeps panel.row.file on the name
	}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{CarouselLayout: panelcarousel.DefaultLayout()})

	listTextWidth := width - 2
	nameWidth := panelListNameWidth(listTextWidth, panel.ListFormatBrief, false, false)
	rowY := rect.Y + 2
	// EntryDisplayRunes prefixes files with a space when icons are off.
	nameX := rect.X + 2
	sizeX := rect.X + 1 + nameWidth + 1
	// Size is right-aligned in panelListSizeCells; skip leading pad spaces.
	sizeGlyphX := sizeX
	for x := sizeX; x < sizeX+panelListSizeCells; x++ {
		ch, _, _ := screen.Get(x, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r != ' ' && r != 0 {
			sizeGlyphX = x
			break
		}
	}

	nameCh, nameStyle, _ := screen.Get(nameX, rowY)
	nr, _ := utf8.DecodeRuneInString(nameCh)
	if nr != 'h' {
		t.Fatalf("name cell = %q, want 'h'", nameCh)
	}
	nameFG, _, _ := nameStyle.Decompose()
	if nameFG != wantFileFG {
		t.Fatalf("name FG = %v, want panel.row.file %v", nameFG, wantFileFG)
	}

	sizeCh, sizeStyle, _ := screen.Get(sizeGlyphX, rowY)
	sr, _ := utf8.DecodeRuneInString(sizeCh)
	if !strings.ContainsRune("4.9K", sr) {
		t.Fatalf("size cell = %q, want a size glyph", sizeCh)
	}
	sizeFG, _, _ := sizeStyle.Decompose()
	if sizeFG != wantInfoFG {
		t.Fatalf("size FG = %v, want panel.row.info %v (cell %q)", sizeFG, wantInfoFG, sizeCh)
	}
}
