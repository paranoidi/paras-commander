package dialog

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestRowMarksWidth(t *testing.T) {
	cases := []struct {
		name string
		m    RowMarks
		want int
	}{
		{"none", RowMarks{}, 0},
		{"pinned only", RowMarks{Pinned: true}, 2},
		{"job only", RowMarks{HasJob: true}, 2},
		{"both", RowMarks{Pinned: true, HasJob: true}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RowMarksWidth(tc.m); got != tc.want {
				t.Fatalf("RowMarksWidth(%+v) = %d, want %d", tc.m, got, tc.want)
			}
		})
	}
}

func newSimScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	return screen
}

func cellRune(screen tcell.SimulationScreen, x, y int) rune {
	str, _, _ := screen.Get(x, y)
	r, _ := utf8.DecodeRuneInString(str)
	return r
}

func TestDrawRowMarksSuffixBlankFillWhenNoMarks(t *testing.T) {
	screen := newSimScreen(t, 20, 3)
	styles := theme.Default()
	bg := tcell.ColorBlack

	used := DrawRowMarksSuffix(screen, 2, 1, 4, RowMarks{}, bg, styles)
	if used != 0 {
		t.Fatalf("used = %d, want 0", used)
	}
	for x := 2; x < 6; x++ {
		if r := cellRune(screen, x, 1); r != ' ' {
			t.Fatalf("cell (%d,1) = %q, want blank", x, r)
		}
	}
}

func TestDrawRowMarksSuffixPaintsJobBeforePin(t *testing.T) {
	screen := newSimScreen(t, 20, 3)
	styles := theme.Default()
	bg := tcell.ColorBlack

	m := RowMarks{Pinned: true, HasJob: true, JobStatus: "running", JobWrite: true}
	used := DrawRowMarksSuffix(screen, 2, 1, RowMarksWidth(m), m, bg, styles)
	if used != 4 {
		t.Fatalf("used = %d, want 4", used)
	}

	wantJobGlyph := styles.SymbolFilelistJob()
	wantPinGlyph := rowMarksPinRune(styles)

	if r := cellRune(screen, 2, 1); r != ' ' {
		t.Fatalf("cell 2 = %q, want leading space before job glyph", r)
	}
	if r := cellRune(screen, 3, 1); r != wantJobGlyph {
		t.Fatalf("cell 3 = %q, want job glyph %q", r, wantJobGlyph)
	}
	if r := cellRune(screen, 4, 1); r != ' ' {
		t.Fatalf("cell 4 = %q, want leading space before pin glyph", r)
	}
	if r := cellRune(screen, 5, 1); r != wantPinGlyph {
		t.Fatalf("cell 5 = %q, want pin glyph %q", r, wantPinGlyph)
	}

	jobFG, _, _ := styles.PanelJobMarkStyle(m.JobStatus, m.JobWrite).Decompose()
	_, jobStyle, _ := screen.Get(3, 1)
	if fg, _, _ := jobStyle.Decompose(); fg != jobFG {
		t.Fatalf("job glyph fg = %v, want %v", fg, jobFG)
	}

	pinFG, _, _ := styles.PanelRowMarkPinned.Decompose()
	_, pinStyle, _ := screen.Get(5, 1)
	if fg, _, _ := pinStyle.Decompose(); fg != pinFG {
		t.Fatalf("pin glyph fg = %v, want %v", fg, pinFG)
	}
}

func TestDrawRowMarksSuffixTruncatesToMaxWidth(t *testing.T) {
	screen := newSimScreen(t, 20, 3)
	styles := theme.Default()
	bg := tcell.ColorBlack

	m := RowMarks{Pinned: true, HasJob: true}
	// Only 2 cells available: job fits, pin does not.
	used := DrawRowMarksSuffix(screen, 2, 1, 2, m, bg, styles)
	if used != 2 {
		t.Fatalf("used = %d, want 2", used)
	}
	if r := cellRune(screen, 3, 1); r != styles.SymbolFilelistJob() {
		t.Fatalf("cell 3 = %q, want job glyph", r)
	}
}
