package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestFolderIconForegroundOpenUsesIndicatorColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := th.FolderIconForeground(theme.FolderIconOpen, "", rowStyle)
	wantFG, _, _ := th.PanelRowFolderOpen.Decompose()
	if fg != wantFG {
		t.Fatalf("open fg = %v, want panel.row.folder.open %v", fg, wantFG)
	}
}

func TestFolderIconForegroundMountUsesIndicatorColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := th.FolderIconForeground(theme.FolderIconMount, "", rowStyle)
	wantFG, _, _ := th.PanelRowFolderMount.Decompose()
	if fg != wantFG {
		t.Fatalf("mount fg = %v, want panel.row.folder.mount %v", fg, wantFG)
	}
}

func TestFolderIconForegroundScanningUsesDiskscanColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := th.FolderIconForeground(theme.FolderIconScanning, "", rowStyle)
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
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, PanelIconStripContext{
		Folder: panellist.FolderIconContext{OtherPanelPath: "/tmp/child"},
	})

	openRune := []rune(th.FolderIconGlyph(theme.FolderIconOpen))[0]
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
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, PanelIconStripContext{})

	folderRune := []rune(th.FolderIconGlyph(theme.FolderIconDefault))[0]
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
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, PanelIconStripContext{})

	main, _, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	if gotRune == []rune(th.FolderIconGlyph(theme.FolderIconDefault))[0] {
		t.Fatal("file row should not use folder theme glyph")
	}
}
