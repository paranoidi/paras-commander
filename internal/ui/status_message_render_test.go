package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestMessageUrgencyListStyleUsesBannerBackgroundForInfo(t *testing.T) {
	styles := theme.Default()
	_, panelBG, _ := styles.PanelActiveSurface.Decompose()
	st := messageUrgencyListStyle(styles, MessageUrgencyInfo, panelBG)
	fg, bg, _ := st.Decompose()
	_, bannerBG, _ := styles.MessageInfo.Decompose()
	if fg != bannerBG {
		t.Fatalf("info list fg = %v, want banner bg %v", fg, bannerBG)
	}
	if bg != panelBG {
		t.Fatalf("info list bg = %v, want panel bg %v", bg, panelBG)
	}
}

func TestDrawStatusMessageTruncatesCenteredRunesWithoutTilde(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 12)

	styles := theme.Default()
	msg := "abcdefghijklmnopqrstuvwxyz"
	drawStatusMessageOverlay(screen, Rect{X: 0, Y: 0, Width: 40, Height: 1}, msg, MessageUrgencyInfo, styles, 1)

	msgRunes := []rune(FormatToastDisplay(msg))
	wantStart := (40 - len(msgRunes)) / 2
	firstStr, _, _ := screen.Get(wantStart+1, 0)
	firstR, _ := utf8.DecodeRuneInString(firstStr)
	if firstR != 'a' {
		t.Fatalf("first content rune = %q at col %d, want 'a'", firstR, wantStart+1)
	}
	lastStr, _, _ := screen.Get(wantStart+len(msgRunes)-2, 0)
	lastR, _ := utf8.DecodeRuneInString(lastStr)
	if lastR != 'z' {
		t.Fatalf("last visible rune = %q, want 'z'", lastR)
	}
}

func TestDrawStatusMessageCentersShortText(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	styles := theme.Default()
	drawStatusMessageOverlay(screen, Rect{X: 0, Y: 5, Width: 80, Height: 1}, "Hi", MessageUrgencyInfo, styles, 1)

	wantCol := (80-len([]rune(FormatToastDisplay("Hi"))))/2 + 1
	str, st, _ := screen.Get(wantCol, 5)
	r, _ := utf8.DecodeRuneInString(str)
	if r != 'H' || st != styles.MessageInfo {
		t.Fatalf("message at col %d = %q style %v, want H with MessageInfo", wantCol, r, st)
	}
}

func TestDrawStatusMessageSweepRevealsLeadingCellsWithEdge(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(20, 1)

	styles := theme.Default()
	// " lantern " is 9 cells centered at col 5; half the sweep shows 5 cells, the 5th is the edge.
	drawStatusMessageOverlay(screen, Rect{Width: 20, Height: 1}, "lantern", MessageUrgencyInfo, styles, 0.5)

	if _, st, _ := screen.Get(5+3, 0); st != styles.MessageInfo {
		t.Fatalf("revealed cell style = %v, want MessageInfo", st)
	}
	if str, st, _ := screen.Get(5+4, 0); str != "t" || st != styles.MessageSweep {
		t.Fatalf("edge cell = %q style %v, want t with MessageSweep", str, st)
	}
	if str, _, _ := screen.Get(5+5, 0); str != " " {
		t.Fatalf("unrevealed cell = %q, want untouched blank", str)
	}
}

func TestDrawStatusMessageSuccessPrefixesIconAndUsesSuccessStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(20, 1)

	styles := theme.Default()
	styles.UseNerdfontIcons = true
	// "  harbor " is 10 cells centered at col 5; the icon sits after the leading pad.
	drawStatusMessageOverlay(screen, Rect{Width: 20, Height: 1}, "harbor", MessageUrgencySuccess, styles, 1)

	if str, st, _ := screen.Get(5+1, 0); str != "" || st != styles.MessageSuccess {
		t.Fatalf("icon cell = %q style %v, want success icon with MessageSuccess", str, st)
	}
	if str, _, _ := screen.Get(5+3, 0); str != "h" {
		t.Fatalf("text cell = %q, want h", str)
	}
}
