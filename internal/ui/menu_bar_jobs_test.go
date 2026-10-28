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
	runningIconW := runewidth.RuneWidth(styles.IconMenuJob("running"))
	queuedIconW := runewidth.RuneWidth(styles.IconMenuJob("queued"))
	// "<icon> <count>" per group, one-space separators between groups.
	one := MenuBarJobsGroupsWidth([]MenuBarJobGroup{{Status: "running", Count: 3}}, styles)
	if want := runningIconW + 1 + 1; one != want {
		t.Fatalf("single group width = %d, want %d", one, want)
	}
	two := MenuBarJobsGroupsWidth([]MenuBarJobGroup{
		{Status: "running", Count: 3},
		{Status: "queued", Count: 12},
	}, styles)
	if want := (runningIconW + 1 + 1) + 1 + (queuedIconW + 1 + 2); two != want {
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

// TestDrawMenuBarJobsGapRemovingUsesRemoveGradient checks a move's source removal draws
// MenuProgressRemoveGfx, and falls back to the delete gradient when the theme defines none.
func TestDrawMenuBarJobsGapRemovingUsesRemoveGradient(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	delGfx := []tcell.Style{tcell.StyleDefault.Foreground(tcell.ColorPurple), tcell.StyleDefault.Foreground(tcell.ColorOrange)}
	remGfx := []tcell.Style{tcell.StyleDefault.Foreground(tcell.ColorRed), tcell.StyleDefault.Foreground(tcell.ColorYellow)}
	styles.MenuProgressDeleteGfx = delGfx
	styles.MenuProgressRemoveGfx = nil // as a theme without remove.gfx keys

	headAndTrail := func(styles theme.Theme) (tcell.Style, tcell.Style) {
		t.Helper()
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init() error = %v", err)
		}
		defer screen.Fini()
		screen.SetSize(10, 2)
		strip := MenuBarJobsStrip{HasProgress: true, ProgressFrac: 1, Removing: true, LightbarHead: 1}
		DrawMenuBarJobsGap(screen, 0, 0, 5, strip, styles)
		_, head, _ := screen.Get(1, 0)
		_, trail, _ := screen.Get(0, 0)
		return head, trail
	}

	if head, trail := headAndTrail(styles); head != delGfx[0] || trail != delGfx[1] {
		t.Fatalf("no remove gradient: head/trail = %v/%v, want delete gradient %v/%v", head, trail, delGfx[0], delGfx[1])
	}
	styles.MenuProgressRemoveGfx = remGfx
	if head, trail := headAndTrail(styles); head != remGfx[0] || trail != remGfx[1] {
		t.Fatalf("head/trail = %v/%v, want remove gradient %v/%v", head, trail, remGfx[0], remGfx[1])
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

func TestDrawMenuBarJobsGapSpeedPill(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	doneSym := string(styles.IconMenuProgressDone())
	remSym := string(styles.IconMenuProgressRemaining())

	newScreen := func(t *testing.T) tcell.SimulationScreen {
		t.Helper()
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init() error = %v", err)
		}
		t.Cleanup(screen.Fini)
		screen.SetSize(32, 3)
		return screen
	}

	barStartX := func(t *testing.T, screen tcell.SimulationScreen, totalWidth int) int {
		t.Helper()
		for x := 0; x < totalWidth; x++ {
			str, _, _ := screen.Get(x, 0)
			if str == doneSym || str == remSym {
				return x
			}
		}
		t.Fatal("no progress bar cell found")
		return -1
	}

	const totalWidth = 20 // >= 11 (slot) + 1 (margin) + 3 (min bar) + queueW(0), room to spare

	for _, speed := range []string{"120MB/s", "2KB/s", "99MB/s", "100MB/s", ""} {
		t.Run("speed_"+speed, func(t *testing.T) {
			t.Parallel()
			screen := newScreen(t)
			strip := MenuBarJobsStrip{HasProgress: true, ProgressFrac: 0.5, Speed: speed, SpeedSlot: true}
			DrawMenuBarJobsGap(screen, 0, 0, totalWidth, strip, styles)

			gotBarX := barStartX(t, screen, totalWidth)
			if wantBarX := menuBarSpeedSlotWidth + 1; gotBarX != wantBarX {
				t.Fatalf("Speed %q: bar starts at x=%d, want %d (slot stays fixed width)", speed, gotBarX, wantBarX)
			}

			// Margin cell right before the bar is untouched menu-bar background.
			if str, style, _ := screen.Get(gotBarX-1, 0); str != " " || style != styles.MenuBarInactive {
				t.Fatalf("Speed %q: margin cell = %q/%v, want blank MenuBarInactive", speed, str, style)
			}

			if speed == "" {
				// No pill: every slot cell stays menu-bar background.
				for x := 0; x < menuBarSpeedSlotWidth; x++ {
					if str, style, _ := screen.Get(x, 0); str != " " || style != styles.MenuBarInactive {
						t.Fatalf("empty speed: slot cell %d = %q/%v, want blank MenuBarInactive", x, str, style)
					}
				}
				return
			}

			// The pill always fills the whole slot: caps at both slot edges regardless of text
			// length, so the pill never changes size between e.g. 99MB/s and 100MB/s.
			if str, style, _ := screen.Get(0, 0); str != string(styles.IconMenuSpeedLeft()) || style != styles.MenuSpeedCap {
				t.Fatalf("Speed %q: left cap at 0 = %q/%v, want %q/%v", speed, str, style, styles.IconMenuSpeedLeft(), styles.MenuSpeedCap)
			}
			rightCapX := menuBarSpeedSlotWidth - 1
			if str, style, _ := screen.Get(rightCapX, 0); str != string(styles.IconMenuSpeedRight()) || style != styles.MenuSpeedCap {
				t.Fatalf("Speed %q: right cap at %d = %q/%v, want %q/%v", speed, rightCapX, str, style, styles.IconMenuSpeedRight(), styles.MenuSpeedCap)
			}
			// Text is right-aligned inside the pill; the padding to its left is pill fill.
			textX := rightCapX - 1 - len([]rune(speed))
			if str, style, _ := screen.Get(textX, 0); str != speed[:1] || style != styles.MenuSpeedText {
				t.Fatalf("Speed %q: text cell at %d = %q/%v, want %q/%v", speed, textX, str, style, speed[:1], styles.MenuSpeedText)
			}
			for x := 1; x < textX; x++ {
				if str, style, _ := screen.Get(x, 0); str != " " || style != styles.MenuSpeedText {
					t.Fatalf("Speed %q: pad cell %d = %q/%v, want blank MenuSpeedText", speed, x, str, style)
				}
			}
		})
	}

	t.Run("no speed slot for a delete job", func(t *testing.T) {
		t.Parallel()
		screen := newScreen(t)
		strip := MenuBarJobsStrip{HasProgress: true, ProgressFrac: 0.5, Deleting: true}
		DrawMenuBarJobsGap(screen, 0, 0, totalWidth, strip, styles)
		if gotBarX := barStartX(t, screen, totalWidth); gotBarX != 0 {
			t.Fatalf("delete job: bar starts at x=%d, want 0 (no pill slot)", gotBarX)
		}
	})

	t.Run("move removing sources keeps the slot and fill", func(t *testing.T) {
		t.Parallel()
		transfer, removing := newScreen(t), newScreen(t)
		strip := MenuBarJobsStrip{Groups: []MenuBarJobGroup{{Status: "running", Count: 1}}, HasProgress: true, ProgressFrac: 0.5, Speed: "120MB/s", SpeedSlot: true}
		DrawMenuBarJobsGap(transfer, 0, 0, totalWidth+6, strip, styles)
		strip.Removing = true
		DrawMenuBarJobsGap(removing, 0, 0, totalWidth+6, strip, styles)
		for x := 0; x < totalWidth+6; x++ {
			a, _, _ := transfer.Get(x, 0)
			b, _, _ := removing.Get(x, 0)
			if a != b {
				t.Fatalf("cell %d = %q while removing, %q while copying; want the same layout and fill", x, b, a)
			}
		}
	})

	t.Run("narrow gap drops the pill, keeps the bar", func(t *testing.T) {
		t.Parallel()
		screen := newScreen(t)
		const narrowWidth = 10 // < 12 (slot) + 1 (margin) + 3 (min bar)
		strip := MenuBarJobsStrip{HasProgress: true, ProgressFrac: 0.5, Speed: "120MB/s", SpeedSlot: true}
		DrawMenuBarJobsGap(screen, 0, 0, narrowWidth, strip, styles)
		if gotBarX := barStartX(t, screen, narrowWidth); gotBarX != 0 {
			t.Fatalf("narrow gap: bar starts at x=%d, want 0 (bar keeps full span)", gotBarX)
		}
	})
}
