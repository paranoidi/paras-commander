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

func TestMenuBarProgressCutoff(t *testing.T) {
	t.Parallel()
	if got := menuBarProgressCutoff(0.5, 10); got != 5 {
		t.Fatalf("50%% of 10 => 5 filled, got %d", got)
	}
	if got := menuBarProgressCutoff(1, 3); got != 3 {
		t.Fatalf("100%% => all filled, got %d", got)
	}
	if got := menuBarProgressCutoff(0, 8); got != 0 {
		t.Fatalf("0%% => none filled, got %d", got)
	}
}

func TestDrawMenuBarJobsGapLightbar(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	gfxHead := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	gfxTrail := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	styles.MenuProgressDoneGfx = []tcell.Style{gfxHead, gfxTrail}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	strip := MenuBarJobsStrip{HasProgress: true, ProgressFrac: 1, LightbarHead: 1}
	if exited := DrawMenuBarJobsGap(screen, 0, 0, 5, strip, styles); exited {
		t.Fatal("head 1 of 5 done cells must not report exited")
	}
	// Exited once the whole gradient has slid past the 5-cell done span.
	strip.LightbarHead = 5 + len(styles.MenuProgressDoneGfx) - 1
	if exited := DrawMenuBarJobsGap(screen, 1, 0, 5, strip, styles); !exited {
		t.Fatal("head past done edge + gradient must report exited")
	}

	_, gotHeadStyle, _ := screen.Get(1, 0)
	if gotHeadStyle != gfxHead {
		t.Fatalf("cell 1 style = %v, want light-bar head %v", gotHeadStyle, gfxHead)
	}
	_, gotTrailStyle, _ := screen.Get(0, 0)
	if gotTrailStyle != gfxTrail {
		t.Fatalf("cell 0 style = %v, want light-bar trail %v", gotTrailStyle, gfxTrail)
	}
	for x := 2; x < 5; x++ {
		_, gotStyle, _ := screen.Get(x, 0)
		if gotStyle != styles.MenuProgressDone {
			t.Fatalf("cell %d style = %v, want plain done style %v", x, gotStyle, styles.MenuProgressDone)
		}
	}
	// The exited frame paints no gradient cell; the caller resets the head and repaints.
	for x := 0; x < 5; x++ {
		if _, gotStyle, _ := screen.Get(x, 1); gotStyle != styles.MenuProgressDone {
			t.Fatalf("exited frame: cell %d style = %v, want plain done style", x, gotStyle)
		}
	}
}

func TestDrawMenuBarJobsGapDeleteLightbar(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	doneGfxHead := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	doneGfxTrail := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	delGfxHead := tcell.StyleDefault.Foreground(tcell.ColorRed)
	delGfxTrail := tcell.StyleDefault.Foreground(tcell.ColorYellow)
	styles.MenuProgressDoneGfx = []tcell.Style{doneGfxHead, doneGfxTrail}
	styles.MenuProgressDeleteGfx = []tcell.Style{delGfxHead, delGfxTrail}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(10, 2)

	strip := MenuBarJobsStrip{HasProgress: true, ProgressFrac: 1, Deleting: true, LightbarHead: 1}
	if exited := DrawMenuBarJobsGap(screen, 0, 0, 5, strip, styles); exited {
		t.Fatal("head 1 of 5 done cells must not report exited")
	}

	_, gotHeadStyle, _ := screen.Get(1, 0)
	if gotHeadStyle != delGfxHead {
		t.Fatalf("cell 1 style = %v, want delete light-bar head %v", gotHeadStyle, delGfxHead)
	}
	_, gotTrailStyle, _ := screen.Get(0, 0)
	if gotTrailStyle != delGfxTrail {
		t.Fatalf("cell 0 style = %v, want delete light-bar trail %v", gotTrailStyle, delGfxTrail)
	}
}

func TestDrawMenuBarJobsGapIndeterminatePingPong(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	gfx0 := tcell.StyleDefault.Foreground(tcell.ColorPurple)
	gfx1 := tcell.StyleDefault.Foreground(tcell.ColorOrange)
	styles.MenuProgressDoneGfx = []tcell.Style{gfx0, gfx1}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(10, 3)

	// width 5 => progW 5, period 8; LightbarHead 6 => p=6 => x=2, moving left.
	strip := MenuBarJobsStrip{HasProgress: true, ProgressIndeterminate: true, LightbarHead: 6}
	exited := DrawMenuBarJobsGap(screen, 0, 0, 5, strip, styles)
	if exited {
		t.Fatal("exited = true at LightbarHead 6, want false")
	}
	wantStyle := map[int]tcell.Style{
		0: styles.MenuProgressRemaining,
		1: styles.MenuProgressRemaining,
		2: gfx0,
		3: gfx1,
		4: styles.MenuProgressDone,
	}
	for x, want := range wantStyle {
		_, got, _ := screen.Get(x, 0)
		if got != want {
			t.Fatalf("cell %d style = %v, want %v", x, got, want)
		}
	}

	strip.LightbarHead = 7
	if exited = DrawMenuBarJobsGap(screen, 1, 0, 5, strip, styles); exited {
		t.Fatal("exited = true at LightbarHead 7 (last leftward frame), want false")
	}
	strip.LightbarHead = 8 // back at cell 0 after a full bounce
	if exited = DrawMenuBarJobsGap(screen, 1, 0, 5, strip, styles); !exited {
		t.Fatal("exited = false at LightbarHead 8, want true")
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
