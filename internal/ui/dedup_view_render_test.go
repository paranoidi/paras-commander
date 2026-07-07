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

func TestDrawDedupViewDirectoryFolderIconUsesListingColor(t *testing.T) {
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
	if fg != wantFG {
		t.Fatalf("closed folder icon fg %v, want panel.row.directory %v", fg, wantFG)
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
