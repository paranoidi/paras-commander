package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDirectoryIconGlyphPriority(t *testing.T) {
	th := theme.Default()
	dir := localfs.Entry{Name: "alpha", Path: "/tmp/alpha", Type: localfs.EntryDirectory}

	if got := directoryIconGlyph(dir, th, "/tmp/alpha", false, 0, false, false, false, true); got != th.SymbolFoldersOpen() {
		t.Fatalf("open glyph = %q, want %q", got, th.SymbolFoldersOpen())
	}
	if got := directoryIconGlyph(dir, th, "/tmp/alpha", false, 0, false, true, false, true); got != th.SymbolFoldersScanning() {
		t.Fatalf("scanning wins over open: got %q, want %q", got, th.SymbolFoldersScanning())
	}
	if got := directoryIconGlyph(dir, th, "", false, 0, false, false, false, true); got != th.SymbolFoldersFolder() {
		t.Fatalf("default folder glyph = %q, want %q", got, th.SymbolFoldersFolder())
	}
	if got := directoryIconGlyph(dir, th, "", false, 0, false, false, true, true); got != diskUsageExcludedFolderGlyph {
		t.Fatalf("excluded glyph = %q, want %q", got, diskUsageExcludedFolderGlyph)
	}
}

func TestPanelDeviconForegroundOpenUsesIndicatorColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := panelDeviconForeground(rowStyle, "", th, "", false, false, true, false)
	wantFG, _, _ := th.PanelRowFolderOpen.Decompose()
	if fg != wantFG {
		t.Fatalf("open fg = %v, want panel.row.folder.open %v", fg, wantFG)
	}
}

func TestPanelDeviconForegroundMountUsesIndicatorColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := panelDeviconForeground(rowStyle, "", th, "", false, false, false, true)
	wantFG, _, _ := th.PanelRowFolderMount.Decompose()
	if fg != wantFG {
		t.Fatalf("mount fg = %v, want panel.row.folder.mount %v", fg, wantFG)
	}
}

func TestPanelDeviconForegroundScanningUsesDiskscanColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := panelDeviconForeground(rowStyle, "", th, "", true, false, false, false)
	wantFG, _, _ := th.PanelFolderDiskscan.Decompose()
	if fg != wantFG {
		t.Fatalf("scanning fg = %v, want panel.folder.diskscan %v", fg, wantFG)
	}
}

func TestPaintPanelIconStripOpenDirectory(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	th := theme.Default()
	entry := localfs.Entry{Name: "child", Path: "/tmp/child", Type: localfs.EntryDirectory}
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, "", false, false, false, "/tmp/child", false, 0, false)

	openRune := []rune(th.SymbolFoldersOpen())[0]
	main, style, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	if gotRune != openRune {
		t.Fatalf("icon rune = %U, want open folder %U", gotRune, openRune)
	}
	openFG, _, _ := th.PanelRowFolderOpen.Decompose()
	gotFG, _, _ := style.Decompose()
	if gotFG != openFG {
		t.Fatalf("icon fg = %v, want %v", gotFG, openFG)
	}
}

func TestPaintPanelIconStripDefaultDirectory(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	th := theme.Default()
	entry := localfs.Entry{Name: "child", Path: "/tmp/child", Type: localfs.EntryDirectory}
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, "", false, false, false, "", false, 0, false)

	folderRune := []rune(th.SymbolFoldersFolder())[0]
	main, _, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	if gotRune != folderRune {
		t.Fatalf("icon rune = %U, want folder %U", gotRune, folderRune)
	}
}

func TestPaintPanelIconStripFileStillUsesDevicon(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	th := theme.Default()
	entry := localfs.Entry{Name: "readme.txt", Path: "/tmp/readme.txt", Type: localfs.EntryFile}
	rowStyle := th.PanelListingEntryStyle(localfs.EntryFile, false)
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, "", false, false, false, "", false, 0, false)

	main, _, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	if gotRune == []rune(th.SymbolFoldersFolder())[0] {
		t.Fatal("file row should not use folder theme glyph")
	}
}
