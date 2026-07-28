package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPaintPanelIconStripPreviewLoadingUsesScanningColor(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	th := theme.Default()
	entry := localfs.Entry{Name: "clip.mp4", Path: "/tmp/clip.mp4", Type: localfs.EntryFile}
	rowStyle := th.PanelListingEntryStyle(localfs.EntryFile, false)
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, PanelIconStripContext{PreviewLoading: true})

	main, style, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	wantGlyph := th.SymbolFilelistPreviewLoading()
	if gotRune != wantGlyph {
		t.Fatalf("icon rune = %q, want preview_loading %q", string(gotRune), string(wantGlyph))
	}
	gotFG, _, _ := style.Decompose()
	wantFG, _, _ := th.PanelIconFolderScanning.Decompose()
	if gotFG != wantFG {
		t.Fatalf("icon fg = %v, want scanning magenta %v", gotFG, wantFG)
	}
}
