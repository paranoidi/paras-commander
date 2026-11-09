package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	barIconCursorName = "acorn.txt"
	barIconLongName   = "harbor.txt"
	barIconMidName    = "lantern.txt"
	barIconShortName  = "pebble.txt"
)

// barIconRows is a flat fileindex listing whose rows have a full bar (long), a bar that covers
// the icons but stops before the name's suffix mark (mid), and a bar that stops before the icons
// (short). The cursor row is listed first and has no bar unless a test gives it one.
func barIconRows() (panel.State, map[string]int64) {
	root := "/scan/root"
	entry := func(name string, size int64) localfs.Entry {
		return localfs.Entry{Name: name, Path: root + "/" + name, Type: localfs.EntryFile, Size: size, IndexStatus: localfs.IndexStatusIndexed}
	}
	entries := []localfs.Entry{
		entry(barIconCursorName, 0),
		entry(barIconLongName, 1000),
		entry(barIconMidName, 150),
		entry(barIconShortName, 1),
	}
	sizes := map[string]int64{}
	for _, e := range entries {
		sizes[e.Path] = e.Size
	}
	return panel.State{
		Columns:   panel.ColumnsBrief,
		Path:      pathloc.MustParse(root),
		Entries:   entries,
		WithIndex: true,
	}, sizes
}

func drawBarIconPanel(t *testing.T, state panel.State, sizes map[string]int64, ctx PanelContext, pinned map[string]struct{}) tcell.Screen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const w, h = 80, 14
	screen.SetSize(w, h)
	ctx.SyncDriverPanelID, ctx.QuickViewDriverPanelID = -1, -1
	drawPanel(screen, Rect{X: 0, Y: 0, Width: w, Height: h}, state,
		PanelStyleConfig{Styles: theme.Default()},
		ctx,
		PanelDisplayConfig{
			ShowIcons:     true,
			ShowDiskUsage: true,
			Painter:       partialDiskUsagePainter{sizes: sizes},
			PinnedPaths:   pinned,
		})
	return screen
}

// barIconNameCell locates name's row and the screen column where the name starts.
func barIconNameCell(t *testing.T, screen tcell.Screen, name string) (nameX, y int) {
	t.Helper()
	w, h := screen.Size()
	for y := range h {
		var line strings.Builder
		for x := range w {
			s, _, _ := screen.Get(x, y)
			if s == "" {
				s = " "
			}
			line.WriteString(string([]rune(s)[0]))
		}
		if i := strings.Index(line.String(), name); i >= 0 {
			// Index counts bytes; icons before the name are multi-byte runes.
			return len([]rune(line.String()[:i])), y
		}
	}
	t.Fatalf("row %q not on screen", name)
	return 0, 0
}

// barIconCellStyle returns the style of the cell offset cells from the start of name's row text.
func barIconCellStyle(t *testing.T, screen tcell.Screen, name string, offset int) (fg, bg tcell.Color) {
	t.Helper()
	nameX, y := barIconNameCell(t, screen, name)
	_, st, _ := screen.Get(nameX+offset, y)
	fg, bg, _ = st.Decompose()
	return fg, bg
}

// barIconGlyphLeftOf returns the style of the nth non-blank cell left of name (1 = the file icon,
// 2 = the fileindex state icon).
func barIconGlyphLeftOf(t *testing.T, screen tcell.Screen, name string, nth int) (fg, bg tcell.Color) {
	t.Helper()
	nameX, y := barIconNameCell(t, screen, name)
	for x := nameX - 1; x > 0; x-- {
		s, st, _ := screen.Get(x, y)
		if strings.TrimSpace(s) == "" {
			continue
		}
		if nth--; nth == 0 {
			fg, bg, _ = st.Decompose()
			return fg, bg
		}
	}
	t.Fatalf("row %q: glyph not found left of the name", name)
	return 0, 0
}

func barIconFileIconStyle(t *testing.T, screen tcell.Screen, name string) (fg, bg tcell.Color) {
	t.Helper()
	return barIconGlyphLeftOf(t, screen, name, 1)
}

func barIconIndexStyle(t *testing.T, screen tcell.Screen, name string) (fg, bg tcell.Color) {
	t.Helper()
	return barIconGlyphLeftOf(t, screen, name, 2)
}

func fgOf(st tcell.Style) tcell.Color {
	fg, _, _ := st.Decompose()
	return fg
}

func TestDiskUsageBarIconColorStopsAtBarEnd(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	barIcon := th.PanelFileIconFG["panel.usage.normal"]
	_, barBG, _ := th.PanelUsageNormal.Decompose()
	indexedFG := fgOf(th.PanelFileIndexStyle(localfs.IndexStatusIndexed))
	if barIcon == indexedFG {
		t.Fatal("test needs distinct bar icon and index state colors")
	}

	state, sizes := barIconRows()
	screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, nil)

	if fg, bg := barIconIndexStyle(t, screen, barIconLongName); fg != barIcon || bg != barBG {
		t.Errorf("long row index icon fg=%v bg=%v, want %v on bar %v", fg, bg, barIcon, barBG)
	}
	if fg, bg := barIconFileIconStyle(t, screen, barIconLongName); fg != barIcon || bg != barBG {
		t.Errorf("long row file icon fg=%v bg=%v, want %v on bar %v", fg, bg, barIcon, barBG)
	}
	// The short row's bar covers only the leading gutter.
	if fg, bg := barIconIndexStyle(t, screen, barIconShortName); fg != indexedFG || bg == barBG {
		t.Errorf("short row index icon fg=%v bg=%v, want state color %v off the bar", fg, bg, indexedFG)
	}
	if fg, bg := barIconFileIconStyle(t, screen, barIconShortName); fg == barIcon || bg == barBG {
		t.Errorf("short row file icon fg=%v bg=%v, want devicon color off the bar", fg, bg)
	}
}

func TestDiskUsageBarIconColorAppliesToSelectedRows(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	state, sizes := barIconRows()
	state.AddSelection(state.Entries[1].Path)
	screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, nil)

	_, barBG, _ := th.PanelUsageSelected.Decompose()
	want := th.PanelFileIconFG["panel.usage.selected"]
	if fg, bg := barIconFileIconStyle(t, screen, barIconLongName); fg != want || bg != barBG {
		t.Errorf("selected row file icon fg=%v bg=%v, want %v on bar %v", fg, bg, want, barBG)
	}
	if fg, bg := barIconIndexStyle(t, screen, barIconLongName); fg != want || bg != barBG {
		t.Errorf("selected row index icon fg=%v bg=%v, want %v on bar %v", fg, bg, want, barBG)
	}
}

func TestDiskUsageBarIconColorFollowsSuffixMarkCell(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	barIcon := th.PanelFileIconFG["panel.usage.normal"]
	pinnedFG := fgOf(th.PanelRowMarkPinned)
	if barIcon == pinnedFG {
		t.Fatal("test needs distinct bar icon and pin mark colors")
	}
	state, sizes := barIconRows()
	pinned := map[string]struct{}{state.Entries[1].Path: {}, state.Entries[2].Path: {}}
	screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, pinned)

	markOffset := func(name string) int { return len(name) + 1 }
	if fg, _ := barIconCellStyle(t, screen, barIconLongName, markOffset(barIconLongName)); fg != barIcon {
		t.Errorf("mark on the bar fg=%v, want %v", fg, barIcon)
	}
	if fg, _ := barIconFileIconStyle(t, screen, barIconMidName); fg != barIcon {
		t.Errorf("mid row file icon fg=%v, want %v: its bar must reach the icon", fg, barIcon)
	}
	if fg, _ := barIconCellStyle(t, screen, barIconMidName, markOffset(barIconMidName)); fg != pinnedFG {
		t.Errorf("mark past the bar end fg=%v, want %v", fg, pinnedFG)
	}
}

func TestDiskUsageBarCursorRowKeepsItsIconColors(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	const cursorEntry = 1

	t.Run("active", func(t *testing.T) {
		t.Parallel()
		state, sizes := barIconRows()
		state.Cursor = cursorEntry
		screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, nil)
		if fg, _ := barIconFileIconStyle(t, screen, barIconLongName); fg != th.PanelFileIconFG["panel.active.row.cursor"] {
			t.Errorf("file icon fg=%v, want the cursor row icon color", fg)
		}
		if fg, _ := barIconIndexStyle(t, screen, barIconLongName); fg != fgOf(th.PanelCursorActive) {
			t.Errorf("index icon fg=%v, want the cursor row foreground", fg)
		}
	})
	t.Run("active selected", func(t *testing.T) {
		t.Parallel()
		state, sizes := barIconRows()
		state.Cursor = cursorEntry
		state.AddSelection(state.Entries[cursorEntry].Path)
		screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, nil)
		if fg, _ := barIconFileIconStyle(t, screen, barIconLongName); fg != th.PanelFileIconFG["panel.active.row.cursor.selected"] {
			t.Errorf("file icon fg=%v, want the selected cursor row icon color", fg)
		}
		if fg, _ := barIconIndexStyle(t, screen, barIconLongName); fg != fgOf(th.PanelActiveCursorSelected) {
			t.Errorf("index icon fg=%v, want the selected cursor row foreground", fg)
		}
	})
	t.Run("inactive", func(t *testing.T) {
		t.Parallel()
		state, sizes := barIconRows()
		state.Cursor = cursorEntry
		screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel}, nil)
		// panel.inactive.usage.cursor has a black background and sets no icon color.
		if fg, _ := barIconFileIconStyle(t, screen, barIconLongName); fg == th.PanelFileIconFG["panel.usage.normal"] {
			t.Errorf("file icon fg=%v must not take the normal bar's icon color", fg)
		}
	})
}

func TestDiskUsageBarIconColorIgnoredWhenChromeBlocked(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	_, barBG, _ := th.PanelUsageNormal.Decompose()
	state, sizes := barIconRows()
	screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, ChromeBlocked: true}, nil)
	fg, bg := barIconIndexStyle(t, screen, barIconLongName)
	if fg != fgOf(th.PanelBlockedText) {
		t.Errorf("index icon fg=%v, want the blocked text color", fg)
	}
	if bg == barBG {
		t.Errorf("index icon bg=%v: blocked chrome has no bar", bg)
	}
}

func TestDiskUsageBarIconColorOnTreeRows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sizes := map[string]int64{}
	for name, size := range map[string]int{barIconCursorName: 0, barIconLongName: 1000, barIconShortName: 1} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		sizes[path] = int64(size)
	}
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	state.SetListLayout(panel.ListLayoutTree, 10)
	th := theme.Default()
	barIcon := th.PanelFileIconFG["panel.usage.normal"]
	_, barBG, _ := th.PanelUsageNormal.Decompose()

	screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, nil)
	if fg, bg := barIconFileIconStyle(t, screen, barIconLongName); fg != barIcon || bg != barBG {
		t.Errorf("long tree row icon fg=%v bg=%v, want %v on bar %v", fg, bg, barIcon, barBG)
	}
	if fg, bg := barIconFileIconStyle(t, screen, barIconShortName); fg == barIcon || bg == barBG {
		t.Errorf("short tree row icon fg=%v bg=%v, want devicon color off the bar", fg, bg)
	}
}

func TestDiskUsageBarIconColorOnGitColumn(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	barIcon := th.PanelFileIconFG["panel.usage.normal"]
	_, barBG, _ := th.PanelUsageNormal.Decompose()
	modifiedFG := fgOf(th.PanelGitStyle(gitstatus.Modified.ThemeKey()))
	newFG := fgOf(th.PanelGitStyle(gitstatus.New.ThemeKey()))
	if barIcon == modifiedFG || barIcon == newFG {
		t.Fatal("test needs distinct bar icon and git status colors")
	}
	const unstagedCell, stagedCell = 2, 3 // non-blank cells left of the name, after the file icon

	state, sizes := barIconRows()
	state.WithIndex = false
	state.GitColumnActive = true
	state.GitByPath = map[string]gitstatus.Cell{}
	for _, e := range state.Entries {
		state.GitByPath[e.Path] = gitstatus.Cell{Staged: gitstatus.Modified, Unstaged: gitstatus.New}
	}
	state.Cursor = 0
	screen := drawBarIconPanel(t, state, sizes, PanelContext{PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true}, nil)

	for _, cell := range []int{unstagedCell, stagedCell} {
		if fg, bg := barIconGlyphLeftOf(t, screen, barIconLongName, cell); fg != barIcon || bg != barBG {
			t.Errorf("long row cell %d fg=%v bg=%v, want %v on bar %v", cell, fg, bg, barIcon, barBG)
		}
	}
	// The short row's bar covers only the leading gutter.
	if fg, bg := barIconGlyphLeftOf(t, screen, barIconShortName, stagedCell); fg != modifiedFG || bg == barBG {
		t.Errorf("short row staged fg=%v bg=%v, want %v off the bar", fg, bg, modifiedFG)
	}
	if fg, bg := barIconGlyphLeftOf(t, screen, barIconShortName, unstagedCell); fg != newFG || bg == barBG {
		t.Errorf("short row unstaged fg=%v bg=%v, want %v off the bar", fg, bg, newFG)
	}
	// The cursor row has no bar here and keeps its own foreground.
	if fg, _ := barIconGlyphLeftOf(t, screen, barIconCursorName, stagedCell); fg != fgOf(th.PanelCursorActive) {
		t.Errorf("cursor row staged fg=%v, want the cursor row foreground", fg)
	}
}

// A folder row is bold; its icon on the bar must not be, or bold-as-bright terminals paint the
// black bar icon in the bar's own grey.
func TestDiskUsageBarFolderIconNotBold(t *testing.T) {
	t.Parallel()
	root := "/scan/root"
	dir := func(name string) localfs.Entry {
		return localfs.Entry{Name: name, Path: root + "/" + name, Type: localfs.EntryDirectory}
	}
	state := panel.State{Columns: panel.ColumnsBrief, Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{dir("acorn"), dir("harbor")}}
	sizes := map[string]int64{root + "/acorn": 1, root + "/harbor": 1000}
	screen := drawBarIconPanel(t, state, sizes, PanelContext{FileListActive: true, CursorRowActive: true}, nil)
	nameX, y := barIconNameCell(t, screen, "harbor")
	for x := nameX - 1; x > 0; x-- {
		s, st, _ := screen.Get(x, y)
		if strings.TrimSpace(s) == "" {
			continue
		}
		fg, _, attrs := st.Decompose()
		if fg != tcell.ColorBlack || attrs&tcell.AttrBold != 0 {
			t.Fatalf("folder icon fg=%v bold=%v, want black not bold", fg, attrs&tcell.AttrBold != 0)
		}
		return
	}
	t.Fatal("folder icon not found")
}
