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

func TestDrawCompareViewPrimaryFocusHighlightsLeftColumn(t *testing.T) {
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
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	leftX := contentX
	rightX := contentX + pathW + compareStatusCol + 1
	lineY := rect.Y + 2

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{
				Kind:         comparepkg.KindContentDiff,
				PrimaryRel:   "alpha.txt",
				SecondaryRel: "beta.txt",
				HashDone:     true,
			},
		},
	}
	view := CompareViewState{
		Selected:    0,
		Filter:      comparepkg.FilterAll,
		FocusColumn: CompareColumnPrimary,
	}
	rows := []comparepkg.Row{snap.Rows[0]}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: rows}, styles, false, "", SplitHorizontal)

	leftStyle := cellStyleAt(screen, leftX, lineY)
	gapStyle := cellStyleAt(screen, leftX+pathW-comparePathGapCol, lineY)
	rightStyle := cellStyleAt(screen, rightX, lineY)
	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	_, leftBG, _ := leftStyle.Decompose()
	_, gapBG, _ := gapStyle.Decompose()
	_, rightBG, _ := rightStyle.Decompose()

	if leftBG != activeBG {
		t.Fatalf("primary focus: left bg %v, want active cursor bg %v; right bg %v", leftBG, activeBG, rightBG)
	}
	if gapBG == activeBG {
		t.Fatalf("primary focus: gap before status should not use active cursor bg")
	}
	if rightBG == activeBG {
		t.Fatalf("primary focus: right column should not use active cursor bg")
	}
}

func TestDrawCompareViewSecondaryFocusHighlightsRightColumn(t *testing.T) {
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
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	leftX := contentX
	rightX := contentX + pathW + compareStatusCol + 1
	lineY := rect.Y + 2

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{
				Kind:         comparepkg.KindContentDiff,
				PrimaryRel:   "alpha.txt",
				SecondaryRel: "beta.txt",
				HashDone:     true,
			},
		},
	}
	view := CompareViewState{
		Selected:    0,
		Filter:      comparepkg.FilterAll,
		FocusColumn: CompareColumnSecondary,
	}
	rows := []comparepkg.Row{snap.Rows[0]}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: rows}, styles, false, "", SplitHorizontal)

	leftStyle := cellStyleAt(screen, leftX, lineY)
	rightStyle := cellStyleAt(screen, rightX, lineY)
	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	_, leftBG, _ := leftStyle.Decompose()
	_, rightBG, _ := rightStyle.Decompose()

	if rightBG != activeBG {
		t.Fatalf("secondary focus: right bg %v, want active cursor bg %v; left bg %v", rightBG, activeBG, leftBG)
	}
	if leftBG == activeBG {
		t.Fatalf("secondary focus: left column should not use active cursor bg")
	}
}

func TestDrawCompareViewPrimaryFocusOnEmptyLeftCell(t *testing.T) {
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
	contentX := rect.X + 2
	leftX := contentX
	lineY := rect.Y + 2

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{Kind: comparepkg.KindSecondaryOnly, SecondaryRel: "solo.txt", HashDone: true},
		},
	}
	view := CompareViewState{
		Selected:    0,
		Filter:      comparepkg.FilterAll,
		FocusColumn: CompareColumnPrimary,
	}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: snap.Rows}, styles, false, "", SplitHorizontal)

	_, leftStyle, _ := screen.Get(leftX, lineY)
	_, leftBG, _ := leftStyle.Decompose()
	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	if leftBG != activeBG {
		t.Fatalf("primary focus on empty left cell: bg %v, want active cursor bg %v", leftBG, activeBG)
	}
}

func TestDrawCompareViewUnfocusedColumnNotRowSelectedOnSelectedRow(t *testing.T) {
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
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	rightX := contentX + pathW + compareStatusCol + 1
	lineY := rect.Y + 2

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{Kind: comparepkg.KindContentDiff, PrimaryRel: "alpha.txt", SecondaryRel: "beta.txt", HashDone: true},
		},
	}
	view := CompareViewState{Selected: 0, Filter: comparepkg.FilterAll, FocusColumn: CompareColumnPrimary}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: snap.Rows}, styles, false, "", SplitHorizontal)

	selectedFG, _, _ := styles.PanelRowSelected.Decompose()
	rightFG, _, _ := cellStyleAt(screen, rightX, lineY).Decompose()
	if rightFG == selectedFG {
		t.Fatalf("unfocused right column should not use row-selected fg %v on selected row", selectedFG)
	}
}

func TestDrawCompareViewFocusedColumnCursorOnlyOnSelectedRow(t *testing.T) {
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
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	leftX := contentX
	rightX := contentX + pathW + compareStatusCol + 1
	firstLineY := rect.Y + 2
	secondLineY := firstLineY + 1

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{Kind: comparepkg.KindEqual, PrimaryRel: "first.txt", SecondaryRel: "first.txt", HashDone: true},
			{Kind: comparepkg.KindContentDiff, PrimaryRel: "second.txt", SecondaryRel: "second.txt", HashDone: true},
			{Kind: comparepkg.KindEqual, PrimaryRel: "third.txt", SecondaryRel: "third.txt", HashDone: true},
		},
	}
	view := CompareViewState{
		Selected:    1,
		Filter:      comparepkg.FilterAll,
		FocusColumn: CompareColumnPrimary,
	}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: snap.Rows}, styles, false, "", SplitHorizontal)

	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	_, jobsBG, _ := styles.JobsRow.Decompose()

	for _, tc := range []struct {
		name       string
		x, y       int
		wantActive bool
	}{
		{"focused row left", leftX, secondLineY, true},
		{"other row left above", leftX, firstLineY, false},
		{"other row left below", leftX, secondLineY + 1, false},
		{"focused row right", rightX, secondLineY, false},
		{"other row right", rightX, firstLineY, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, bg, _ := cellStyleAt(screen, tc.x, tc.y).Decompose()
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

func TestDrawCompareViewStripsCommonPathPrefix(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(120, 14)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 60, Height: 11},
		Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 11},
	}
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	leftX := contentX
	rightX := contentX + pathW + compareStatusCol + 1
	lineY := rect.Y + 2
	home := "/home/alice"

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/home/alice/projects/paras-commander/test-cases/diff-b"),
		SecondaryRoot: pathloc.MustParse("/home/alice/projects/paras-commander/test-cases/diff-a"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{
				Kind:         comparepkg.KindContentDiff,
				PrimaryRel:   "alpha.txt",
				SecondaryRel: "alpha.txt",
				HashDone:     true,
			},
		},
	}
	view := CompareViewState{Selected: 0, Filter: comparepkg.FilterAll, FocusColumn: CompareColumnPrimary}
	rows := []comparepkg.Row{snap.Rows[0]}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: rows}, styles, false, home, SplitHorizontal)

	leftText := rowTextAt(screen, leftX, lineY, pathW-comparePathGapCol)
	rightText := rowTextAt(screen, rightX, lineY, pathW)
	if leftText != "alpha.txt" {
		t.Fatalf("left path = %q, want %q", leftText, "alpha.txt")
	}
	if rightText != "alpha.txt" {
		t.Fatalf("right path = %q, want %q", rightText, "alpha.txt")
	}
}

func TestCompareDisplayPathPairSingleSided(t *testing.T) {
	t.Run("primary only", func(t *testing.T) {
		left, right := compareDisplayPathPair("subdir/solo.txt", "")
		if left != "subdir/solo.txt" {
			t.Fatalf("left = %q, want %q", left, "subdir/solo.txt")
		}
		if right != "" {
			t.Fatalf("right = %q, want empty", right)
		}
	})

	t.Run("secondary only", func(t *testing.T) {
		left, right := compareDisplayPathPair("", "subdir/solo.txt")
		if left != "" {
			t.Fatalf("left = %q, want empty", left)
		}
		if right != "subdir/solo.txt" {
			t.Fatalf("right = %q, want %q", right, "subdir/solo.txt")
		}
	})

	t.Run("both empty unchanged", func(t *testing.T) {
		left, right := compareDisplayPathPair("", "")
		if left != "" || right != "" {
			t.Fatalf("got (%q, %q), want empty pair", left, right)
		}
	})
}

func TestDrawCompareViewStripsCommonPathPrefixSingleSided(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(120, 14)

	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 60, Height: 11},
		Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 11},
	}
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, SplitHorizontal)
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	leftX := contentX
	rightX := contentX + pathW + compareStatusCol + 1
	lineY := rect.Y + 2
	home := "/home/alice"

	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/home/alice/projects/paras-commander/test-cases/diff-b"),
		SecondaryRoot: pathloc.MustParse("/home/alice/projects/paras-commander/test-cases/diff-a"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{Kind: comparepkg.KindPrimaryOnly, PrimaryRel: "solo.txt", HashDone: true},
		},
	}
	view := CompareViewState{Selected: 0, Filter: comparepkg.FilterAll, FocusColumn: CompareColumnPrimary}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: snap.Rows}, styles, false, home, SplitHorizontal)

	leftText := rowTextAt(screen, leftX, lineY, pathW-comparePathGapCol)
	rightText := rowTextAt(screen, rightX, lineY, pathW)
	if leftText != "solo.txt" {
		t.Fatalf("left path = %q, want %q", leftText, "solo.txt")
	}
	if rightText != "-" {
		t.Fatalf("right path = %q, want %q (absent side indicator)", rightText, "-")
	}
}

func TestDrawCompareViewUsesFullListHeight(t *testing.T) {
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

	rows := make([]comparepkg.Row, visibleRows)
	for i := range rows {
		name := fmt.Sprintf("file-%02d.txt", i)
		rows[i] = comparepkg.Row{
			Kind:         comparepkg.KindContentDiff,
			PrimaryRel:   name,
			SecondaryRel: name,
			HashDone:     true,
		}
	}
	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows:          rows,
	}
	view := CompareViewState{Selected: visibleRows - 1, Filter: comparepkg.FilterAll}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: rows}, theme.Default(), false, "", SplitHorizontal)

	want := fmt.Sprintf("file-%02d.txt", visibleRows-1)
	got := rowTextAt(screen, rect.X+2, lastListY, 20)
	if got != want {
		t.Fatalf("last list row text = %q, want %q (empty/missing last item)", got, want)
	}
}

func rowTextAt(screen tcell.SimulationScreen, x, y, width int) string {
	var b strings.Builder
	for dx := 0; dx < width; dx++ {
		ch, _, _ := screen.Get(x+dx, y)
		if ch == "" || ch == " " {
			continue
		}
		b.WriteString(ch)
	}
	return strings.TrimSpace(b.String())
}

func cellStyleAt(screen tcell.SimulationScreen, x, y int) tcell.Style {
	for dx := 0; dx < 3; dx++ {
		ch, st, _ := screen.Get(x+dx, y)
		if ch != "" && ch != " " {
			return st
		}
	}
	_, style, _ := screen.Get(x, y)
	return style
}

func TestDrawCompareViewActiveHashingGlyphIsGreen(t *testing.T) {
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
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - compareStatusCol - 1) / 2
	statusX := contentX + pathW + 1
	lineY := rect.Y + 2

	rows := []comparepkg.Row{
		{
			Kind:         comparepkg.KindEqual,
			PrimaryRel:   "active.bin",
			SecondaryRel: "active.bin",
			Hashing:      true,
		},
		{
			Kind:         comparepkg.KindEqual,
			PrimaryRel:   "queued.bin",
			SecondaryRel: "queued.bin",
		},
	}
	snap := comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseHashing,
		Rows:          rows,
		HashTotal:     2,
	}
	view := CompareViewState{Selected: 0, Filter: comparepkg.FilterAll, FocusColumn: CompareColumnPrimary}

	drawCompareView(screen, layout, view, compareViewData{Snap: snap, Rows: rows}, styles, false, "", SplitHorizontal)

	wantFG, _, _ := styles.CompareHashing.Decompose()
	// Pending disk glyph is a Nerd Font private-use rune; the simulation screen often
	// reports it as a space cell — read the status column origin directly.
	_, activeStyle, _ := screen.Get(statusX, lineY)
	activeFG, _, _ := activeStyle.Decompose()
	if activeFG != wantFG {
		t.Fatalf("active hashing glyph fg = %v, want compare.hashing %v", activeFG, wantFG)
	}

	_, queuedStyle, _ := screen.Get(statusX, lineY+1)
	queuedFG, _, _ := queuedStyle.Decompose()
	if queuedFG == wantFG {
		t.Fatalf("queued pending glyph should not use compare.hashing green")
	}
}
