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
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
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
	var list []DedupEntry
	for i := range visibleRows {
		rel := fmt.Sprintf("file-%02d.bin", i)
		abs := pathloc.MustParse("/scan/root/" + rel)
		list = append(list, DedupEntry{
			File:       comparepkg.DedupFile{Rel: rel, Abs: abs},
			AbsKey:     abs.String(),
			GroupFirst: true,
			Size:       1024,
			Copies:     2,
		})
	}

	drawDedupView(screen, layout, DedupViewState{}, snap, list, theme.Default(), false, "", SplitHorizontal)

	ch, _, _ := screen.Get(rect.X+2, lastListY)
	if strings.TrimSpace(ch) == "" {
		t.Fatalf("last list row at y=%d is blank; expected full-height list", lastListY)
	}
}

func TestDrawDedupHashProgressBarAndLabel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 14)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 11},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 11},
	}
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
	progressX := rect.X + 1
	progressW := rect.Width - 2
	lineY := rect.Y + 2

	snap := comparepkg.DedupSnapshot{
		Root:      pathloc.MustParse("/scan/root"),
		Phase:     comparepkg.DedupHashing,
		Hashed:    1,
		HashTotal: 4,
		Current:   "nested",
	}
	view := DedupViewState{}
	layoutChrome := drawAuxPanelChrome(screen, rect, dedupViewTitle(snap), "", true, false, styles)
	drawDedupView(screen, layout, view, snap, nil, styles, false, "", SplitHorizontal)

	_, wantUsageBG, _ := styles.PanelUsageNormal.Decompose()
	_, rowBG, _ := styles.JobsRow.Background(layoutChrome.ContentBG).Decompose()

	fillCols := 1
	if snap.HashTotal > 0 {
		fillCols = int(float64(snap.Hashed) / float64(snap.HashTotal) * float64(progressW))
	}
	if fillCols < 1 {
		fillCols = 1
	}
	for col := progressX; col < progressX+fillCols; col++ {
		_, gotBG, _ := cellStyleAt(screen, col, lineY).Decompose()
		if gotBG != wantUsageBG {
			t.Fatalf("filled col %d bg %v, want disk-usage accent %v (row bg %v)", col, gotBG, wantUsageBG, rowBG)
		}
	}
	for col := progressX + fillCols; col < progressX+progressW; col++ {
		_, gotBG, _ := cellStyleAt(screen, col, lineY).Decompose()
		if gotBG != rowBG {
			t.Fatalf("unfilled col %d bg %v, want row bg %v", col, gotBG, rowBG)
		}
	}

	var line strings.Builder
	for col := progressX; col < progressX+progressW; col++ {
		ch, _, _ := screen.Get(col, lineY)
		line.WriteString(ch)
	}
	got := strings.TrimSpace(line.String())
	want := "Hashing nested…"
	if !strings.Contains(got, want) {
		t.Fatalf("progress label = %q, want substring %q", got, want)
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
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
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
	list := DedupEntriesFromSnapshot(snap)
	view := DedupViewState{Selected: 1}

	drawDedupView(screen, layout, view, snap, list, styles, false, "", SplitHorizontal)

	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	_, jobsBG, _ := styles.JobsRow.Decompose()
	pathW := max((rect.Width-4)-1-dedupSizeCol, 4)
	pathX := contentX
	gapBeforeSizeX := pathX + pathW
	sizeX := gapBeforeSizeX + 1
	innerRight := rect.X + rect.Width - 2

	for _, tc := range []struct {
		name       string
		x, y       int
		wantActive bool
		direct     bool // right inner margin: avoid cellStyleAt peeking into the frame border
	}{
		{"left margin", rect.X + 1, secondLineY, true, false},
		{"path column", pathX, secondLineY, true, false},
		{"gap before size", gapBeforeSizeX, secondLineY, true, false},
		{"size column", sizeX, secondLineY, true, false},
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
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
	headerY := rect.Y + 1
	contentX := rect.X + 2
	innerRight := rect.X + rect.Width - 2

	home := "/home/user"
	root := pathloc.MustParse(home + "/projects")
	snap := comparepkg.DedupSnapshot{
		Root:  root,
		Phase: comparepkg.DedupDone,
	}
	drawDedupView(screen, layout, DedupViewState{}, snap, nil, styles, false, home, SplitHorizontal)

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
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
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
	list := DedupEntriesFromSnapshot(snap)
	view := DedupViewState{
		Selected: 0,
		Marked: map[string]bool{
			absA.String(): true,
			absB.String(): true,
		},
	}

	drawDedupView(screen, layout, view, snap, list, styles, false, "", SplitHorizontal)

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
		wantCursor bool
	}{
		{"first row cursor", firstLineY, true},
		{"second row marked", secondLineY, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fg, bg, _ := cellStyleAt(screen, contentX, tc.y).Decompose()
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
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2

	snap := comparepkg.DedupSnapshot{
		Root:  pathloc.MustParse("/scan/root"),
		Phase: comparepkg.DedupAwaitConfirm,
	}
	view := DedupViewState{}
	drawDedupView(screen, layout, view, snap, nil, styles, false, "", SplitHorizontal)

	title := dedupViewTitle(snap)
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
