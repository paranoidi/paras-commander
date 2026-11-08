package ui

import (
	"io/fs"
	"testing"
	"unicode/utf8"

	devicons "github.com/epilande/go-devicons"
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestFolderIconForegroundOpenUsesIndicatorColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := th.FolderIconForeground(theme.FolderIconOpen, "", rowStyle)
	wantFG, _, _ := th.PanelIconFolderOpen.Decompose()
	if fg != wantFG {
		t.Fatalf("open fg = %v, want panel.icon.folder.open %v", fg, wantFG)
	}
}

func TestFolderIconForegroundMountUsesIndicatorColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := th.FolderIconForeground(theme.FolderIconMount, "", rowStyle)
	wantFG, _, _ := th.PanelIconFolderMount.Decompose()
	if fg != wantFG {
		t.Fatalf("mount fg = %v, want panel.icon.folder.mount %v", fg, wantFG)
	}
}

func TestFolderIconForegroundScanningUsesDiskscanColor(t *testing.T) {
	th := theme.Default()
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, false)
	fg := th.FolderIconForeground(theme.FolderIconScanning, "", rowStyle)
	wantFG, _, _ := th.StatusScanning.Decompose()
	if fg != wantFG {
		t.Fatalf("scanning fg = %v, want status.scanning %v", fg, wantFG)
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

	openRune := []rune(th.FolderIcon(theme.FolderIconOpen))[0]
	main, style, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	if gotRune != openRune {
		t.Fatalf("icon rune = %U, want open folder %U", gotRune, openRune)
	}
	openFG, _, _ := th.PanelIconFolderOpen.Decompose()
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

	folderRune := []rune(th.FolderIcon(theme.FolderIconDefault))[0]
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
	if gotRune == []rune(th.FolderIcon(theme.FolderIconDefault))[0] {
		t.Fatal("file row should not use folder theme icon")
	}
}

func paintBrokenLinkIcon(t *testing.T, th theme.Theme, entry localfs.Entry, ctx PanelIconStripContext) (string, tcell.Color) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)
	paintPanelIconStrip(screen, 0, 1, entry, th.PanelListingEntryStyle(entry.Type, ctx.ChromeBlocked), th, ctx)
	main, style, _ := screen.Get(0, 1)
	fg, _, _ := style.Decompose()
	return main, fg
}

func TestPaintPanelIconStripBrokenLinkUsesBrokenLinkColor(t *testing.T) {
	th := theme.Default()
	wantFG, _, _ := th.PanelIconBrokenLink.Decompose()
	if rowFG, _, _ := th.PanelRowSymlink.Decompose(); wantFG == rowFG {
		t.Fatalf("panel.icon.broken_link %v must differ from panel.row.symlink", wantFG)
	}
	// An image-named link must not take the image preview tint.
	for _, name := range []string{"harbor", "harbor.png"} {
		entry := localfs.Entry{Name: name, Path: "/tmp/" + name, Type: localfs.EntrySymlink, Mode: fs.ModeSymlink | 0o777, BrokenLink: true}
		got, gotFG := paintBrokenLinkIcon(t, th, entry, PanelIconStripContext{})
		if want := string([]rune(devicons.IconForInfo(fileInfoFromEntry(entry)).Icon)[0]); got != want {
			t.Fatalf("%s: icon = %q, want link icon %q", name, got, want)
		}
		if gotFG != wantFG {
			t.Fatalf("%s: icon fg = %v, want panel.icon.broken_link %v", name, gotFG, wantFG)
		}
	}
}

func TestPaintPanelIconStripBrokenLinkBlockedUsesRowForeground(t *testing.T) {
	th := theme.Default()
	entry := localfs.Entry{Name: "harbor", Path: "/tmp/harbor", Type: localfs.EntrySymlink, Mode: fs.ModeSymlink | 0o777, BrokenLink: true}
	_, gotFG := paintBrokenLinkIcon(t, th, entry, PanelIconStripContext{ChromeBlocked: true})
	wantFG, _, _ := th.PanelBlockedRowSymlink.Decompose()
	if gotFG != wantFG {
		t.Fatalf("blocked icon fg = %v, want panel.blocked.row.symlink %v", gotFG, wantFG)
	}
}

func TestPaintPanelIconStripBlockedFileUsesRowForeground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	th := theme.Default()
	entry := localfs.Entry{Name: "main.go", Path: "/tmp/main.go", Type: localfs.EntryFile}
	rowStyle := th.PanelListingEntryStyle(localfs.EntryFile, true)
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, PanelIconStripContext{ChromeBlocked: true})

	_, style, _ := screen.Get(0, 1)
	gotFG, _, _ := style.Decompose()
	wantFG, _, _ := th.PanelBlockedRowFile.Decompose()
	if gotFG != wantFG {
		t.Fatalf("blocked file icon fg = %v, want panel.blocked.row.file %v", gotFG, wantFG)
	}
}

func TestPaintPanelIconStripBlockedOpenDirectoryUsesRowForeground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	th := theme.Default()
	entry := localfs.Entry{Name: "child", Path: "/tmp/child", Type: localfs.EntryDirectory}
	rowStyle := th.PanelListingEntryStyle(localfs.EntryDirectory, true)
	paintPanelIconStrip(screen, 0, 1, entry, rowStyle, th, PanelIconStripContext{
		ChromeBlocked: true,
		Folder:        panellist.FolderIconContext{OtherPanelPath: "/tmp/child"},
	})

	openRune := []rune(th.FolderIcon(theme.FolderIconOpen))[0]
	main, style, _ := screen.Get(0, 1)
	gotRune, _ := utf8.DecodeRuneInString(main)
	if gotRune != openRune {
		t.Fatalf("icon rune = %U, want open folder %U", gotRune, openRune)
	}
	gotFG, _, _ := style.Decompose()
	wantFG, _, _ := th.PanelBlockedRowDirectory.Decompose()
	if gotFG != wantFG {
		t.Fatalf("blocked open folder icon fg = %v, want panel.blocked.row.directory %v", gotFG, wantFG)
	}
}
