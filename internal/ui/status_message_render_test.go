package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawStatusMessageTruncatesCenteredRunesWithoutTilde(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 12)

	styles := theme.Default()
	msg := "abcdefghijklmnopqrstuvwxyz"
	drawStatusMessageOverlay(screen, Rect{X: 0, Y: 0, Width: 40, Height: 1}, msg, MessageUrgencyInfo, styles)

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
	drawStatusMessageOverlay(screen, Rect{X: 0, Y: 5, Width: 80, Height: 1}, "Hi", MessageUrgencyInfo, styles)

	wantCol := (80-len([]rune(FormatToastDisplay("Hi"))))/2 + 1
	str, st, _ := screen.Get(wantCol, 5)
	r, _ := utf8.DecodeRuneInString(str)
	if r != 'H' || st != styles.MessageInfo {
		t.Fatalf("message at col %d = %q style %v, want H with MessageInfo", wantCol, r, st)
	}
}
