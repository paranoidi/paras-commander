package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawDedupViewUsesFullListHeight(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	const panelH = 14
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: panelH},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: panelH},
	}
	rect := layout.Primary // finished results render as twin tree panes; main pane = primary rect
	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 {
		t.Fatalf("PanelListRows() = %d, want > 0", visibleRows)
	}
	lastListY := rect.Y + 2 + visibleRows - 1
	if lastListY != rect.Y+rect.Height-2 {
		t.Fatalf("last list row y = %d, want %d (row above bottom border)", lastListY, rect.Y+rect.Height-2)
	}

	// Enough entries to fill every visible row.
	root := pathloc.MustParse("/scan/root")
	snap := comparepkg.DedupSnapshot{Root: root, Phase: comparepkg.DedupDone}
	for i := range visibleRows {
		snap.Groups = append(snap.Groups, dedupTestGroup(byte(i+1), 1024, fmt.Sprintf("file-%02d.bin", i)))
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if len(list) != visibleRows {
		t.Fatalf("rows = %d, want %d", len(list), visibleRows)
	}

	drawDedupView(screen, layout, DedupViewState{}, snap, list, nil, theme.Default(), false, "", SplitHorizontal)

	ch, _, _ := screen.Get(rect.X+2, lastListY)
	if strings.TrimSpace(ch) == "" {
		t.Fatalf("last list row at y=%d is blank; expected full-height list", lastListY)
	}
}

func TestDrawDedupViewSelectedRowUsesActiveCursorStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary // main tree pane
	contentX := rect.X + 2
	firstLineY := rect.Y + 2
	secondLineY := firstLineY + 1

	relA := "alpha/ledger.bin"
	relB := "beta/ledger.bin"
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 1024,
			Files: []comparepkg.DedupFile{
				{Rel: relA, Abs: pathloc.MustParse("/scan/root/" + relA)},
				{Rel: relB, Abs: pathloc.MustParse("/scan/root/" + relB)},
			},
		}},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	view := DedupViewState{Main: DedupPane{Selected: 1}}

	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	_, jobsBG, _ := styles.JobsRow.Decompose()
	innerRight := rect.X + rect.Width - 2
	sizeW, countW := dedupListColumnWidths(list)
	cols := dedupListColumnLayout(contentX, innerRight, sizeW, countW)

	for _, tc := range []struct {
		name       string
		x, y       int
		wantActive bool
		direct     bool // right inner margin: avoid cellStyleAt peeking into the frame border
	}{
		{"left margin", rect.X + 1, secondLineY, true, false},
		{"path column", cols.pathX, secondLineY, true, false},
		{"gap before count", cols.gapBeforeCountX, secondLineY, true, false},
		{"count column", cols.countColX, secondLineY, true, false},
		{"size column", cols.sizeColX, secondLineY, true, false},
		{"right margin", innerRight, secondLineY, true, true},
		{"other row", contentX, firstLineY, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bg tcell.Color
			if tc.direct {
				_, style, _ := screen.Get(tc.x, tc.y)
				_, bg, _ = style.Decompose()
			} else {
				_, bg, _ = cellStyleAt(screen, tc.x, tc.y).Decompose()
			}
			if tc.wantActive {
				if bg != activeBG {
					t.Fatalf("bg %v, want active cursor bg %v", bg, activeBG)
				}
				return
			}
			if bg == activeBG {
				t.Fatalf("bg %v, should not use active cursor bg", bg)
			}
			if bg != jobsBG {
				t.Fatalf("bg %v, want normal jobs row bg %v", bg, jobsBG)
			}
		})
	}
}

func TestDrawDedupViewRootPathHeaderUsesPanelHeaderBackground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary // main tree pane
	headerY := rect.Y + 1
	contentX := rect.X + 2
	innerRight := rect.X + rect.Width - 2

	home := "/home/user"
	root := pathloc.MustParse(home + "/projects")
	snap := comparepkg.DedupSnapshot{
		Root:  root,
		Phase: comparepkg.DedupDone,
	}
	drawDedupView(screen, layout, DedupViewState{}, snap, nil, nil, styles, false, home, SplitHorizontal)

	_, wantHeaderBG, _ := styles.PanelActiveHeader.Decompose()
	_, surfaceBG, _ := styles.PanelActiveSurface.Decompose()
	if wantHeaderBG == surfaceBG {
		t.Fatal("test requires distinct panel.active.header and panel.active.surface backgrounds")
	}

	for _, tc := range []struct {
		name string
		x    int
	}{
		{"left margin", rect.X + 1},
		{"path start", contentX},
		{"path interior", contentX + 4},
		{"right margin", innerRight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, style, _ := screen.Get(tc.x, headerY)
			_, bg, _ := style.Decompose()
			if bg != wantHeaderBG {
				t.Fatalf("bg %v, want panel.active.header bg %v (not surface %v)", bg, wantHeaderBG, surfaceBG)
			}
		})
	}
}

func TestDrawDedupViewFullyMarkedGroupUsesRedRowStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary // main tree pane
	contentX := rect.X + 2
	firstLineY := rect.Y + 2
	secondLineY := firstLineY + 1

	relA := "alpha/ledger.bin"
	relB := "beta/ledger.bin"
	absA := pathloc.MustParse("/scan/root/" + relA)
	absB := pathloc.MustParse("/scan/root/" + relB)
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 1024,
			Files: []comparepkg.DedupFile{
				{Rel: relA, Abs: absA},
				{Rel: relB, Abs: absB},
			},
		}},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	view := DedupViewState{
		Marked: map[string]bool{
			absA.String(): true,
			absB.String(): true,
		},
	}

	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	wantFG, _, _ := styles.PanelDedupRowAllMarked.Decompose()
	_, selectedFG, _ := styles.PanelActiveCursorSelected.Decompose()
	if wantFG == selectedFG {
		t.Fatal("test requires distinct dedup all-marked and active cursor selected foregrounds")
	}
	_, panelBG, _ := styles.PanelActiveSurface.Decompose()
	_, wantCursorBG, _ := styles.PanelDedupRowCursorAllMarked.Decompose()

	for _, tc := range []struct {
		name       string
		y          int
		row        int
		wantCursor bool
	}{
		{"first row cursor", firstLineY, 0, true},
		{"second row marked", secondLineY, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := list[tc.row]
			pathTextX := contentX + len([]rune(dedupTreePrefix(styles, row)))
			fg, bg, _ := cellStyleAt(screen, pathTextX, tc.y).Decompose()
			if fg != wantFG {
				t.Fatalf("fg %v, want dedup all-marked fg %v", fg, wantFG)
			}
			wantBG := panelBG
			if tc.wantCursor {
				wantBG = wantCursorBG
			}
			if bg != wantBG {
				t.Fatalf("bg %v, want %v", bg, wantBG)
			}
		})
	}
}

func TestDrawDedupViewTitleBarKeepsFrameDashesAfterTitle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2

	root := pathloc.MustParse("/scan/root")
	snap := comparepkg.DedupSnapshot{Root: root, Phase: comparepkg.DedupDone}
	snap.Groups = append(snap.Groups, dedupTestGroup(1, 1024, "alpha.bin"))
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	view := DedupViewState{}
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	title := dedupViewTitle(snap, 0)
	titleRunes := len([]rune(title))
	_, frameBG, _ := styles.PanelActiveFrame.Decompose()
	_, titleBG, _ := styles.PanelActiveTitle.Decompose()

	for _, tc := range []struct {
		name     string
		x        int
		wantDash bool
		wantBG   tcell.Color
	}{
		{"left margin dash", rect.X + 1, true, frameBG},
		{"title first char", titleX, false, titleBG},
		{"title last char", titleX + titleRunes - 1, false, titleBG},
		{"after title dash", titleX + titleRunes, true, frameBG},
		{"right margin dash", innerRight, true, frameBG},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch, _, _ := screen.Get(tc.x, rect.Y)
			style := cellStyleAt(screen, tc.x, rect.Y)
			if tc.wantDash {
				if ch != "─" {
					t.Fatalf("glyph = %q, want frame dash", ch)
				}
			} else if ch == "─" || ch == "" || ch == " " {
				// title cells should be non-dash glyphs
				if strings.TrimSpace(ch) == "" && ch != " " {
					t.Fatalf("glyph = %q, want title text", ch)
				}
			}
			_, bg, _ := style.Decompose()
			if bg != tc.wantBG {
				t.Fatalf("bg %v, want %v", bg, tc.wantBG)
			}
		})
	}
}

func TestDrawDedupViewDirectoryFolderIconUsesDirectoryBlue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	lineY := rect.Y + 2

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: pathloc.MustParse("/scan/root/meadow/lantern.txt")},
				{Rel: "lantern.txt", Abs: pathloc.MustParse("/scan/root/lantern.txt")},
			},
		}},
	}
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Main: DedupPane{
			Selected:  1,
			Collapsed: map[string]bool{"d:meadow": true},
		},
	}
	list, _ := DedupRowsFromSnapshot(snap, view)
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	gutterX := contentX
	fg, _, _ := cellStyleAt(screen, gutterX, lineY).Decompose()
	wantFG, _, _ := styles.PanelRowDirectory.Decompose()
	_, jobsFG, _ := styles.JobsRow.Decompose()
	if wantFG == jobsFG {
		t.Fatal("test requires distinct panel.row.directory and jobs.row foregrounds")
	}
	if fg != wantFG {
		t.Fatalf("closed folder icon fg %v, want directory fg %v (not jobs.row %v)", fg, wantFG, jobsFG)
	}

	openFG, _, _ := styles.PanelIconFolderOpen.Decompose()
	if openFG == wantFG {
		t.Fatal("test requires distinct panel.icon.folder.open and panel.row.directory foregrounds")
	}
}

func TestDrawDedupViewOpenFolderIconUsesDirectoryBlue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	lineY := rect.Y + 2

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: pathloc.MustParse("/scan/root/meadow/lantern.txt")},
				{Rel: "lantern.txt", Abs: pathloc.MustParse("/scan/root/lantern.txt")},
			},
		}},
	}
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Main:        DedupPane{Selected: 1},
	}
	list, _ := DedupRowsFromSnapshot(snap, view)
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	gutterX := contentX
	fg, _, _ := cellStyleAt(screen, gutterX, lineY).Decompose()
	wantFG, _, _ := styles.PanelRowDirectory.Decompose()
	openFG, _, _ := styles.PanelIconFolderOpen.Decompose()
	if openFG == wantFG {
		t.Fatal("test requires distinct panel.icon.folder.open and panel.row.directory foregrounds")
	}
	if fg == openFG {
		t.Fatalf("open folder icon fg %v, want directory fg %v (not panel.icon.folder.open)", fg, wantFG)
	}
	if fg != wantFG {
		t.Fatalf("open folder icon fg %v, want directory fg %v", fg, wantFG)
	}
}

func TestDrawDedupViewCursorSelectedDirFolderIconUsesDirectoryBlue(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	lineY := rect.Y + 2

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: pathloc.MustParse("/scan/root/meadow/lantern.txt")},
				{Rel: "lantern.txt", Abs: pathloc.MustParse("/scan/root/lantern.txt")},
			},
		}},
	}
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Main:        DedupPane{Selected: 0}, // cursor on meadow dir row
	}
	list, _ := DedupRowsFromSnapshot(snap, view)
	if list[0].Value.Kind != DedupRowDir {
		t.Fatalf("first row kind = %v, want DedupRowDir", list[0].Value.Kind)
	}
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	wantDirFG, _, _ := styles.PanelRowDirectory.Decompose()
	_, cursorFG, _ := styles.PanelCursorActive.Decompose()
	if wantDirFG == cursorFG {
		t.Fatal("test requires distinct panel.row.directory and panel.active.row.cursor foregrounds")
	}

	gutterX := contentX
	iconFG, _, _ := cellStyleAt(screen, gutterX, lineY).Decompose()
	if iconFG != wantDirFG {
		t.Fatalf("cursor-selected dir folder icon fg %v, want directory fg %v", iconFG, wantDirFG)
	}
	if iconFG == cursorFG {
		t.Fatalf("cursor-selected dir folder icon fg %v, should not use cursor fg", iconFG)
	}

	nameX := contentX + len([]rune(dedupTreePrefix(styles, list[0]))) + 1
	nameFG, _, _ := cellStyleAt(screen, nameX, lineY).Decompose()
	if nameFG == wantDirFG {
		t.Fatal("test requires dir name to use cursor styling, not plain directory fg")
	}
}

func TestDrawDedupViewDetailsUseListingColumns(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	lineY := rect.Y + 2

	const fileSize = int64(741683)
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: fileSize,
			Files: []comparepkg.DedupFile{
				{Rel: "alpha.bin", Abs: pathloc.MustParse("/scan/root/alpha.bin")},
				{Rel: "beta.bin", Abs: pathloc.MustParse("/scan/root/beta.bin")},
				{Rel: "gamma.bin", Abs: pathloc.MustParse("/scan/root/gamma.bin")},
			},
		}},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if len(list) == 0 || !list[0].Value.ShowSize {
		t.Fatal("expected group header row with ShowSize")
	}

	drawDedupView(screen, layout, DedupViewState{}, snap, list, nil, styles, false, "", SplitHorizontal)

	innerRight := rect.X + rect.Width - 2
	sizeW, countW := dedupListColumnWidths(list)
	cols := dedupListColumnLayout(contentX, innerRight, sizeW, countW)

	wantSize := formatByteSizeListed(fileSize)
	wantCount := "3"

	countField := rowTextAt(screen, cols.countColX, lineY, cols.countColW)
	if !strings.HasSuffix(strings.TrimRight(countField, " "), wantCount) {
		t.Fatalf("count column = %q, want right-aligned %q", countField, wantCount)
	}
	if cols.sizeColX != cols.countColX+cols.countColW+1 {
		t.Fatalf("size column x = %d, want one space after count ending at %d", cols.sizeColX, cols.countColX+cols.countColW)
	}

	sizeField := rowTextAt(screen, cols.sizeColX, lineY, cols.sizeColW)
	if !strings.HasSuffix(sizeField, wantSize) {
		t.Fatalf("size column = %q, want right-aligned %q", sizeField, wantSize)
	}
	if strings.TrimSpace(sizeField) != wantSize {
		t.Fatalf("size column = %q, want only padding plus %q", sizeField, wantSize)
	}
	if cols.sizeColX+cols.sizeColW-1 != cols.sizeColRight {
		t.Fatalf("size column right edge = %d, want %d", cols.sizeColX+cols.sizeColW-1, cols.sizeColRight)
	}
	if cols.sizeColW != len([]rune(wantSize)) {
		t.Fatalf("size column width = %d, want compact width %d for %q", cols.sizeColW, len([]rune(wantSize)), wantSize)
	}
}

func TestDrawDedupViewHeaderShowsListingColumnTitles(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	innerRight := rect.X + rect.Width - 2
	headerY := rect.Y + 1

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 1024,
			Files: []comparepkg.DedupFile{
				{Rel: "alpha.bin", Abs: pathloc.MustParse("/scan/root/alpha.bin")},
				{Rel: "beta.bin", Abs: pathloc.MustParse("/scan/root/beta.bin")},
			},
		}},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	drawDedupView(screen, layout, DedupViewState{}, snap, list, nil, styles, false, "", SplitHorizontal)

	sizeW, countW := dedupListColumnWidths(list)
	cols := dedupListColumnLayout(contentX, innerRight, sizeW, countW)
	countField := rowTextAt(screen, cols.countColX, headerY, cols.countColW)
	if !strings.HasSuffix(strings.TrimRight(countField, " "), dedupListCountTitle) {
		t.Fatalf("header count column = %q, want right-aligned %q", countField, dedupListCountTitle)
	}
	sizeField := rowTextAt(screen, cols.sizeColX, headerY, cols.sizeColW)
	if !strings.HasSuffix(sizeField, dedupListSizeTitle) {
		t.Fatalf("header size column = %q, want right-aligned %q", sizeField, dedupListSizeTitle)
	}
	if cols.sizeColX != cols.countColX+cols.countColW+1 {
		t.Fatalf("header size column x = %d, want one space after count", cols.sizeColX)
	}
	if cols.sizeColRight != innerRight-1 {
		t.Fatalf("sizeColRight = %d, want inner margin at %d", cols.sizeColRight, innerRight-1)
	}
}

func TestDedupListHeaderMatchesCompactColumnSpacing(t *testing.T) {
	pathW := 20
	sizeW, countW := 4, 5
	got := dedupListHeader(pathW, sizeW, countW, "/scan/root")
	want := fmt.Sprintf("%-*s %*s %*s", pathW, "/scan/root", countW, dedupListCountTitle, sizeW, dedupListSizeTitle)
	if got != want {
		t.Fatalf("dedupListHeader() = %q, want %q", got, want)
	}
}

func TestDedupListColumnWidthsUsesCompactMinimums(t *testing.T) {
	rows := []DedupRow{{
		Value: DedupRowData{
			Kind:     DedupRowFile,
			Size:     741683,
			Copies:   3,
			ShowSize: true,
		},
	}}
	sizeW, countW := dedupListColumnWidths(rows)
	if sizeW != len([]rune(formatByteSizeListed(741683))) {
		t.Fatalf("sizeW = %d, want %d", sizeW, len([]rune(formatByteSizeListed(741683))))
	}
	if countW != len([]rune(dedupListCountTitle)) {
		t.Fatalf("countW = %d, want header title width %d", countW, len([]rune(dedupListCountTitle)))
	}
}

func TestDrawDedupViewDirRowShowsSubtreeMarkIndicator(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	dirRowY := rect.Y + 2

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: pathloc.MustParse("/scan/root/meadow/lantern.txt")},
				{Rel: "lantern.txt", Abs: pathloc.MustParse("/scan/root/lantern.txt")},
			},
		}},
	}
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Marked:      map[string]bool{"/scan/root/meadow/lantern.txt": true},
		MarkedCount: 1,
		Main:        DedupPane{Selected: 2}, // keep cursor off the dir row
	}
	list, _ := DedupRowsFromSnapshot(snap, view)
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	var meadowRow DedupRow
	for _, row := range list {
		if row.Value.Kind == DedupRowDir && row.Value.DirRel == "meadow" {
			meadowRow = row
			break
		}
	}
	markX := contentX + len([]rune(dedupTreePrefix(styles, meadowRow))) + 1 + len("meadow")
	r, style, _ := screen.Get(markX, dirRowY)
	if r != string(styles.SymbolFilelistSelectionSubtree()) {
		t.Fatalf("rune at (%d,%d) = %q, want subtree indicator", markX, dirRowY, r)
	}
	fg, _, _ := style.Decompose()
	wantFG, _, _ := styles.PanelRowIndicatorSelectionSubtree.Decompose()
	if fg != wantFG {
		t.Fatalf("indicator fg %v, want panel.row.indicator.selection_subtree %v", fg, wantFG)
	}
}

func TestDrawDedupViewDirRowShowsRedSubtreeMarkWhenGroupFullyMarked(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2
	dirRowY := rect.Y + 2

	absMeadow := pathloc.MustParse("/scan/root/meadow/lantern.txt")
	absRoot := pathloc.MustParse("/scan/root/lantern.txt")
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: absMeadow},
				{Rel: "lantern.txt", Abs: absRoot},
			},
		}},
	}
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Marked: map[string]bool{
			absMeadow.String(): true,
			absRoot.String():   true,
		},
		MarkedCount: 2,
		Main:        DedupPane{Selected: 3}, // keep cursor off the dir row
	}
	list, _ := DedupRowsFromSnapshot(snap, view)
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	var meadowRow DedupRow
	for _, row := range list {
		if row.Value.Kind == DedupRowDir && row.Value.DirRel == "meadow" {
			meadowRow = row
			break
		}
	}
	markX := contentX + len([]rune(dedupTreePrefix(styles, meadowRow))) + 1 + len("meadow")
	r, style, _ := screen.Get(markX, dirRowY)
	if r != string(styles.SymbolFilelistSelectionSubtree()) {
		t.Fatalf("rune at (%d,%d) = %q, want subtree indicator", markX, dirRowY, r)
	}
	fg, _, _ := style.Decompose()
	wantFG, _, _ := styles.PanelDedupRowAllMarked.Decompose()
	if fg != wantFG {
		t.Fatalf("indicator fg %v, want panel.dedup.row.all_marked %v", fg, wantFG)
	}
	yellowFG, _, _ := styles.PanelRowIndicatorSelectionSubtree.Decompose()
	if fg == yellowFG {
		t.Fatalf("indicator fg should be red (all-marked), not yellow subtree color")
	}
}

func TestDrawDedupViewNestedRowShowsTreeConnectors(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Primary
	contentX := rect.X + 2

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "meadow/lantern.txt", Abs: pathloc.MustParse("/scan/root/meadow/lantern.txt")},
				{Rel: "meadow/copy.txt", Abs: pathloc.MustParse("/scan/root/meadow/copy.txt")},
				{Rel: "lantern.txt", Abs: pathloc.MustParse("/scan/root/lantern.txt")},
			},
		}},
	}
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Main:        DedupPane{},
	}
	list, _ := DedupRowsFromSnapshot(snap, view)
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	wantConnectorFG, _, _ := styles.PanelRowTreeConnector.Decompose()
	branch := styles.SymbolTreeBranch()
	endGlyph := styles.SymbolTreeEnd()

	var nestedRow DedupRow
	var nestedY int
	for i, row := range list {
		if row.Depth == 1 {
			nestedRow = row
			nestedY = rect.Y + 2 + i
			break
		}
	}
	if nestedRow.ID == "" {
		t.Fatal("expected a depth-1 nested row")
	}

	connectorX := contentX
	got, style, _ := screen.Get(connectorX, nestedY)
	wantFirst := []rune(dedupTreeConnectorPrefix(styles, nestedRow))[0]
	if got != string(wantFirst) {
		t.Fatalf("connector at (%d,%d) = %q, want %q", connectorX, nestedY, got, string(wantFirst))
	}
	fg, _, _ := style.Decompose()
	if fg != wantConnectorFG {
		t.Fatalf("connector fg %v, want panel.row.tree.connector %v", fg, wantConnectorFG)
	}

	prefix := dedupTreeConnectorPrefix(styles, nestedRow)
	if !strings.Contains(prefix, branch) && !strings.Contains(prefix, endGlyph) {
		t.Fatalf("connector prefix %q, want branch %q or end %q", prefix, branch, endGlyph)
	}
}

func TestDedupTreeConnectorLastChildUsesEnd(t *testing.T) {
	styles := theme.Default()
	row := DedupRow{
		Depth:       1,
		HasChildren: true,
		Expanded:    true,
		LastChild:   true,
	}
	prefix := dedupTreeConnectorPrefix(styles, row)
	endGlyph := styles.SymbolTreeEnd()
	if !strings.HasPrefix(prefix, endGlyph) {
		t.Fatalf("prefix %q, want end %q when last child even if expanded", prefix, endGlyph)
	}

	child := DedupRow{
		Depth:           2,
		LastChild:       true,
		AncestorHasNext: []bool{false},
	}
	childPrefix := dedupTreeConnectorPrefix(styles, child)
	want := "   " + styles.SymbolTreeEnd() + " "
	if childPrefix != want {
		t.Fatalf("child prefix %q, want %q", childPrefix, want)
	}
}

func TestDedupTreeConnectorNonLastChildUsesBranch(t *testing.T) {
	styles := theme.Default()
	row := DedupRow{
		Depth:       1,
		HasChildren: true,
		Expanded:    true,
		LastChild:   false,
	}
	prefix := dedupTreeConnectorPrefix(styles, row)
	branch := styles.SymbolTreeBranch()
	if !strings.HasPrefix(prefix, branch) {
		t.Fatalf("prefix %q, want branch %q when not last child", prefix, branch)
	}

	child := DedupRow{
		Depth:           2,
		LastChild:       true,
		AncestorHasNext: []bool{true},
	}
	childPrefix := dedupTreeConnectorPrefix(styles, child)
	want := styles.SymbolTreeContinue() + "  " + styles.SymbolTreeEnd() + " "
	if childPrefix != want {
		t.Fatalf("child prefix %q, want %q", childPrefix, want)
	}
}

func TestDedupTreeConnectorTwoRootsOnlyChildDir(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "test-relocate-new/basic/IMG_4005.jpg", Abs: pathloc.MustParse("/scan/root/test-relocate-new/basic/IMG_4005.jpg")},
				{Rel: "test-relocate-orig/IMG_4005.jpg", Abs: pathloc.MustParse("/scan/root/test-relocate-orig/IMG_4005.jpg")},
			},
		}},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	styles := theme.Default()

	var basicRow, imgRow DedupRow
	for _, row := range list {
		switch {
		case row.Value.Kind == DedupRowDir && row.Value.DirRel == "test-relocate-new/basic":
			basicRow = row
		case row.Value.Kind == DedupRowFile && row.Value.Display == "IMG_4005.jpg" && row.Depth == 2:
			imgRow = row
		}
	}
	if basicRow.ID == "" || imgRow.ID == "" {
		t.Fatalf("rows = %+v, want basic dir and nested file", list)
	}
	if !basicRow.LastChild {
		t.Fatalf("basic LastChild = false, want true (only child under test-relocate-new)")
	}
	basicPrefix := dedupTreeConnectorPrefix(styles, basicRow)
	if !strings.HasPrefix(basicPrefix, styles.SymbolTreeEnd()) {
		t.Fatalf("basic prefix %q, want end connector", basicPrefix)
	}
	imgPrefix := dedupTreeConnectorPrefix(styles, imgRow)
	wantImg := "   " + styles.SymbolTreeEnd() + " "
	if imgPrefix != wantImg {
		t.Fatalf("img prefix %q, want %q", imgPrefix, wantImg)
	}
	gutter, _ := dedupTreeGutter(styles, imgRow, tcell.StyleDefault, true)
	if gutter != "" {
		t.Fatalf("file row gutter = %q, want empty", gutter)
	}
}

func TestDrawDedupViewCopiesPaneEmptyHeaderOmitsPathDot(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Secondary
	contentX := rect.X + 2
	headerY := rect.Y + 1

	root := pathloc.MustParse("/scan/root")
	snap := comparepkg.DedupSnapshot{Root: root, Phase: comparepkg.DedupDone}
	snap.Groups = append(snap.Groups, dedupTestGroup(1, 1024, "alpha.bin"))
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	drawDedupView(screen, layout, DedupViewState{Main: DedupPane{Selected: -1}}, snap, list, nil, styles, false, "", SplitHorizontal)

	ch, _, _ := screen.Get(contentX, headerY)
	if ch == "." {
		t.Fatalf("empty copies header at x=%d is %q, want no path dot", contentX, ch)
	}
}

func TestDrawDedupViewCopiesPaneEmptyTextStartsAtContentColumn(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	rect := layout.Secondary
	contentX := rect.X + 2
	emptyY := rect.Y + 2
	want := "Select a file to see its copies"

	root := pathloc.MustParse("/scan/root")
	snap := comparepkg.DedupSnapshot{Root: root, Phase: comparepkg.DedupDone}
	snap.Groups = append(snap.Groups, dedupTestGroup(1, 1024, "alpha.bin"))
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	drawDedupView(screen, layout, DedupViewState{Main: DedupPane{Selected: -1}}, snap, list, nil, styles, false, "", SplitHorizontal)

	ch, _, _ := screen.Get(contentX, emptyY)
	if ch != "S" {
		t.Fatalf("first empty-text glyph at x=%d is %q, want %q", contentX, ch, "S")
	}
	for i, wantR := range []rune(want) {
		ch, _, _ := screen.Get(contentX+i, emptyY)
		if ch != string(wantR) {
			t.Fatalf("empty text at x=%d is %q, want %q", contentX+i, ch, string(wantR))
		}
	}
}

func TestDrawDedupViewCopiesPaneDirUsesSelectionStyleWhenFullyMarked(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	copiesRect := layout.Secondary
	contentX := copiesRect.X + 2
	dirRowY := copiesRect.Y + 2

	absMain := pathloc.MustParse("/scan/root/lantern.txt")
	absMeadow := pathloc.MustParse("/scan/root/meadow/lantern.txt")
	absBeacon := pathloc.MustParse("/scan/root/meadow/beacon.txt")
	absOrchard := pathloc.MustParse("/scan/root/orchard/lantern.txt")
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{{
			Size: 4,
			Files: []comparepkg.DedupFile{
				{Rel: "lantern.txt", Abs: absMain},
				{Rel: "meadow/lantern.txt", Abs: absMeadow},
				{Rel: "meadow/beacon.txt", Abs: absBeacon},
				{Rel: "orchard/lantern.txt", Abs: absOrchard},
			},
		}},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	var mainSel DedupRow
	for _, r := range list {
		if r.Value.AbsKey == absMain.String() {
			mainSel = r
			break
		}
	}
	copies := DedupCopyRows(snap, mainSel, nil)
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Marked: map[string]bool{
			absMeadow.String():  true,
			absBeacon.String():  true,
			absOrchard.String(): false,
		},
		Main: DedupPane{Selected: DedupRowIndexByID(list, mainSel.ID)},
	}
	drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal)

	wantSelectedFG, _, _ := styles.PanelRowSelected.Decompose()
	_, normalFG, _ := styles.PanelRowDirectory.Decompose()
	if wantSelectedFG == normalFG {
		t.Fatal("test requires distinct panel.row.selected and panel.row.directory foregrounds")
	}

	// Copies pane: meadow dir has all copy files marked; orchard has one unmarked.
	var meadowRow DedupRow
	for _, row := range copies {
		if row.Value.Kind == DedupRowDir && row.Value.DirRel == "meadow" {
			meadowRow = row
			break
		}
	}
	meadowNameX := contentX + len([]rune(dedupTreePrefix(styles, meadowRow))) + 1
	fg, _, _ := cellStyleAt(screen, meadowNameX, dirRowY).Decompose()
	if fg != wantSelectedFG {
		t.Fatalf("fully marked meadow dir fg %v, want selection fg %v", fg, wantSelectedFG)
	}

	meadowGutterX := contentX + len([]rune(dedupTreeConnectorPrefix(styles, meadowRow)))
	iconFG, _, _ := cellStyleAt(screen, meadowGutterX, dirRowY).Decompose()
	wantIconFG, _, _ := styles.PanelRowDirectory.Decompose()
	if iconFG != wantIconFG {
		t.Fatalf("fully marked meadow folder icon fg %v, want directory fg %v", iconFG, wantIconFG)
	}
	if iconFG == wantSelectedFG {
		t.Fatalf("fully marked meadow folder icon fg %v, should not use selection fg", iconFG)
	}

	orchardRowY := dirRowY + 3 // meadow dir + 2 files + orchard dir
	var orchardRow DedupRow
	for _, row := range copies {
		if row.Value.Kind == DedupRowDir && row.Value.DirRel == "orchard" {
			orchardRow = row
			break
		}
	}
	orchardNameX := contentX + len([]rune(dedupTreePrefix(styles, orchardRow))) + 1
	fg, _, _ = cellStyleAt(screen, orchardNameX, orchardRowY).Decompose()
	if fg == wantSelectedFG {
		t.Fatalf("partially marked orchard dir fg %v, should not use selection fg", fg)
	}
}

func TestDrawDedupViewFileTreePaneDirUsesSelectionStyleWhenFullyMarked(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 13},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 13},
	}
	treeRect := layout.Primary
	contentX := treeRect.X + 2
	dirRowY := treeRect.Y + 2

	absMain := pathloc.MustParse("/scan/root/lantern.txt")
	absMeadow := pathloc.MustParse("/scan/root/meadow/lantern.txt")
	absBeacon := pathloc.MustParse("/scan/root/meadow/beacon.txt")
	absOrchard := pathloc.MustParse("/scan/root/orchard/lantern.txt")
	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			{
				Size: 4,
				Files: []comparepkg.DedupFile{
					{Rel: "lantern.txt", Abs: absMain},
					{Rel: "meadow/lantern.txt", Abs: absMeadow},
					{Rel: "orchard/lantern.txt", Abs: absOrchard},
				},
			},
			{
				Size: 5,
				Files: []comparepkg.DedupFile{
					{Rel: "meadow/beacon.txt", Abs: absBeacon},
				},
			},
		},
	}
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true, TreeDirs: true})
	view := DedupViewState{
		IgnoreEmpty: true,
		TreeDirs:    true,
		Marked: map[string]bool{
			absMeadow.String(): true,
			absBeacon.String(): true,
		},
		Main: DedupPane{Selected: 0},
	}
	drawDedupView(screen, layout, view, snap, list, nil, styles, false, "", SplitHorizontal)

	wantSelectedFG, _, _ := styles.PanelRowSelected.Decompose()
	_, normalFG, _ := styles.PanelRowDirectory.Decompose()
	if wantSelectedFG == normalFG {
		t.Fatal("test requires distinct panel.row.selected and panel.row.directory foregrounds")
	}

	var meadowRow DedupRow
	for _, row := range list {
		if row.Value.Kind == DedupRowDir && row.Value.DirRel == "meadow" {
			meadowRow = row
			break
		}
	}
	meadowNameX := contentX + len([]rune(dedupTreePrefix(styles, meadowRow))) + 1
	fg, _, _ := cellStyleAt(screen, meadowNameX, dirRowY).Decompose()
	if fg != wantSelectedFG {
		t.Fatalf("fully marked meadow dir fg %v, want selection fg %v", fg, wantSelectedFG)
	}

	meadowGutterX := contentX + len([]rune(dedupTreeConnectorPrefix(styles, meadowRow)))
	iconFG, _, _ := cellStyleAt(screen, meadowGutterX, dirRowY).Decompose()
	wantIconFG, _, _ := styles.PanelRowDirectory.Decompose()
	if iconFG != wantIconFG {
		t.Fatalf("fully marked meadow folder icon fg %v, want directory fg %v", iconFG, wantIconFG)
	}

	var orchardRow DedupRow
	for _, row := range list {
		if row.Value.Kind == DedupRowDir && row.Value.DirRel == "orchard" {
			orchardRow = row
			break
		}
	}
	orchardRowY := dirRowY + 1
	orchardNameX := contentX + len([]rune(dedupTreePrefix(styles, orchardRow))) + 1
	fg, _, _ = cellStyleAt(screen, orchardNameX, orchardRowY).Decompose()
	if fg == wantSelectedFG {
		t.Fatalf("partially marked orchard dir fg %v, should not use selection fg", fg)
	}
}
