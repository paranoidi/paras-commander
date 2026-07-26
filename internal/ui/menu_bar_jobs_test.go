package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestMenuBarJobsGroupsWidth(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	runningGlyphW := runewidth.RuneWidth(styles.SymbolMenuJob("running"))
	queuedGlyphW := runewidth.RuneWidth(styles.SymbolMenuJob("queued"))
	// "<glyph> <count>" per group, one-space separators between groups.
	one := MenuBarJobsGroupsWidth([]MenuBarJobGroup{{Status: "running", Count: 3}}, styles)
	if want := runningGlyphW + 1 + 1; one != want {
		t.Fatalf("single group width = %d, want %d", one, want)
	}
	two := MenuBarJobsGroupsWidth([]MenuBarJobGroup{
		{Status: "running", Count: 3},
		{Status: "queued", Count: 12},
	}, styles)
	if want := (runningGlyphW + 1 + 1) + 1 + (queuedGlyphW + 1 + 2); two != want {
		t.Fatalf("two group width = %d, want %d", two, want)
	}
	if w := MenuBarJobsGroupsWidth(nil, styles); w != 0 {
		t.Fatalf("empty groups width = %d, want 0", w)
	}
}

func TestLayoutMenuBarJobsStrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                  string
		total                 int
		stripW                int
		wantProgress          bool
		wantQueueW, wantProgW int
	}{
		{"zero width", 0, 3, true, 0, 0},
		{"both full", 20, 5, true, 5, 14},
		{"progress min width", 9, 5, true, 5, 3},
		{"drop progress narrow", 8, 5, true, 5, 0},
		{"strip too long progress only", 8, 10, true, 0, 8},
		{"strip too long no progress want", 8, 10, false, 0, 0},
		{"strip fits no progress", 8, 5, false, 5, 0},
		{"progress only", 10, 0, true, 0, 10},
		{"progress too narrow alone", 2, 0, true, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, p := LayoutMenuBarJobsStrip(tc.total, tc.stripW, tc.wantProgress)
			if q != tc.wantQueueW || p != tc.wantProgW {
				t.Fatalf("LayoutMenuBarJobsStrip(%d,%d,%v) = (%d,%d), want (%d,%d)",
					tc.total, tc.stripW, tc.wantProgress, q, p, tc.wantQueueW, tc.wantProgW)
			}
		})
	}
}

func TestMenuBarProgressFilledCells(t *testing.T) {
	t.Parallel()
	if !menuBarProgressFilledCells(0.5, 10, 4) {
		t.Fatal("50% of 10 => 5 filled, index 4 should be filled")
	}
	if menuBarProgressFilledCells(0.5, 10, 5) {
		t.Fatal("50% of 10 => 5 filled, index 5 should be empty")
	}
	if !menuBarProgressFilledCells(1, 3, 2) {
		t.Fatal("100% => all filled")
	}
	if menuBarProgressFilledCells(0, 8, 0) {
		t.Fatal("0% => none filled")
	}
}

func TestDrawMenuBarJobsGapHidesZeroCountGroups(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 3)

	strip := MenuBarJobsStrip{Groups: []MenuBarJobGroup{{Status: "running", Count: 2}}}
	DrawMenuBarJobsGap(screen, 0, 0, 20, strip, styles)
	wantW := MenuBarJobsGroupsWidth(strip.Groups, styles)
	for x := wantW; x < 20; x++ {
		str, _, _ := screen.Get(x, 0)
		if str != " " {
			t.Fatalf("cell %d = %q, want blank past the drawn strip (width %d)", x, str, wantW)
		}
	}
}
