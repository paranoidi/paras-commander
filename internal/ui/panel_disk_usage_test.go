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
	// Verify padded attention width.
	padded, ok := menuBarAttentionPadded("󰋗 1 job waiting")
	if !ok || padded == "" {
		t.Fatal("menuBarAttentionPadded returned empty for non-empty attention")
	}
	paddedRunes := []rune(padded)
	if len(paddedRunes) != 17 {
		t.Fatalf("padded attention has %d runes, want 17", len(paddedRunes))
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width = 120
	screen.SetSize(width, 12)

	styles := theme.Default()
	drawMenuBarRightTail(screen, Rect{X: 0, Y: 0, Width: width, Height: 1},
		"󰋗 1 job waiting", "", styles.MenuBarAlert, styles.MenuDetail, styles.MenuSpinner, true, 0)

	spinnerCol := width - menuBarPermRightMargin - 1
	gapCol := spinnerCol - 1
	grCell, _, _ := screen.Get(gapCol, 0)
	gr, _ := utf8.DecodeRuneInString(grCell)
	if gr != ' ' {
		t.Fatalf("gap before spinner = %q, want space", gr)
	}

	// For no-permission case the attention segment starts at cell 100 (see drawMenuBarRightTail).
	attStart := 100
	for col := attStart; col < attStart+len(paddedRunes); col++ {
		_, st, _ := screen.Get(col, 0)
		if st != styles.MenuBarAlert {
			t.Fatalf("attention cell style at %d = %v, want MenuBarAlert", col, st)
		}
	}

	// Verify attention glyphs (handling double-width Nerd Font glyph).
	var attentionText []rune
	for col := attStart; col < attStart+len(paddedRunes); {
		ch, _, cw := screen.Get(col, 0)
		if cw < 1 {
			cw = 1
		}
		r, _ := utf8.DecodeRuneInString(ch)
		attentionText = append(attentionText, r)
		col += cw
	}
	got := string(attentionText)
	if got != padded {
		t.Fatalf("attention text = %q, want %q", got, padded)
	}

	// Spinner cell
	_, st, _ := screen.Get(spinnerCol, 0)
	if st != styles.MenuSpinner {
		t.Fatalf("spinner cell style at %d = %v, want MenuSpinner", spinnerCol, st)
	}
}
