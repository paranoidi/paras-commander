package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDiskUsageFillColumns(t *testing.T) {
	if got := diskUsageFillColumns(500, 1000, 10); got != 5 {
		t.Fatalf("half ratio: got %d want 5", got)
	}
	if got := diskUsageFillColumns(1000, 1000, 10); got != 10 {
		t.Fatalf("full ratio: got %d want 10", got)
	}
	if diskUsageFillColumns(0, 1000, 10) != 0 {
		t.Fatal("zero usage")
	}
	if diskUsageFillColumns(100, 0, 10) != 0 {
		t.Fatal("zero max")
	}
	if got := diskUsageFillColumns(1, 10000, 100); got != 1 {
		t.Fatalf("tiny ratio clamps to single cell: got %d want 1", got)
	}
}

func TestMenuBarSpinnerGlyphDistinctFrames(t *testing.T) {
	if MenuBarSpinnerGlyph(0) == MenuBarSpinnerGlyph(1) {
		t.Fatal("want distinct frames")
	}
	if len(menuBarSpinnerRunes) != 10 {
		t.Fatalf("unexpected frame count %d", len(menuBarSpinnerRunes))
	}
	if MenuBarSpinnerGlyph(12) != MenuBarSpinnerGlyph(2) {
		t.Fatal("want phase modulo frame count")
	}
}

func TestMenuBarPermissionTailRuneCountIncludesSpinnerSlot(t *testing.T) {
	if got := menuBarPermissionTailRuneCount("drwxr-xr-x", false); got != len("drwxr-xr-x") {
		t.Fatalf("no spinner: got %d", got)
	}
	if got := menuBarPermissionTailRuneCount("drwxr-xr-x", true); got != len("drwxr-xr-x")+1+1 {
		t.Fatalf("perm+gap+spinner: got %d", got)
	}
	if got := menuBarPermissionTailRuneCount("", true); got != 1 {
		t.Fatalf("spinner only: got %d", got)
	}
}

func TestMenuBarRightTailJobsAttentionPaddingAndSpinnerGap(t *testing.T) {
	if got := menuBarRightTailRuneCount("! 1", "", false); got != utf8.RuneCountInString(" ! 1 ") {
		t.Fatalf("attention-only tail width: got %d", got)
	}
	wantAttentionSpinner := utf8.RuneCountInString(" ! 1 ") + 1 + menuBarSpinnerCells
	if got := menuBarRightTailRuneCount("! 1", "", true); got != wantAttentionSpinner {
		t.Fatalf("attention + gap + spinner tail width: got %d want %d", got, wantAttentionSpinner)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width = 80
	screen.SetSize(width, 12)

	styles := theme.Default()
	drawMenuBarRightTail(screen, Rect{X: 0, Y: 0, Width: width, Height: 1},
		"! 1", "", styles.MenuBarAlert, styles.MenuDetail, styles.PanelSpinner, true, 0)

	spinnerCol := width - menuBarPermRightMargin - 1
	gapCol := spinnerCol - 1
	grCell, _, _ := screen.Get(gapCol, 0)
	gr, _ := utf8.DecodeRuneInString(grCell)
	if gr != ' ' {
		t.Fatalf("gap before spinner = %q, want space", gr)
	}

	wantLabel := " ! 1 "
	labelStart := gapCol - utf8.RuneCountInString(wantLabel)
	row := textAt(screen, 0, 0, width)
	if gotLabel := row[labelStart:gapCol]; gotLabel != wantLabel {
		t.Fatalf("attention segment = %q, want %q", gotLabel, wantLabel)
	}
	for col := labelStart; col < gapCol; col++ {
		_, st, _ := screen.Get(col, 0)
		if st != styles.MenuBarAlert {
			t.Fatalf("attention cell style at %d = %v, want MenuBarAlert", col, st)
		}
	}
}
