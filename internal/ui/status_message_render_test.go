package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawStatusMessageTruncatesLeadingRunesWithoutTilde(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 12)

	styles := theme.Default()
	// maxStart = reserveExclusiveEnd + leftGap = 20 + 1 => message starts at column 21; width 19 runes.
	drawStatusMessageOverlay(screen, Rect{X: 0, Y: 0, Width: 40, Height: 1}, 20, 1, 0, "abcdefghijklmnopqrstuvwxyz", MessageUrgencyInfo, styles)

	firstStr, _, _ := screen.Get(21, 0)
	firstR, _ := utf8.DecodeRuneInString(firstStr)
	if firstR != 'a' {
		t.Fatalf("first visible rune = %q, want 'a'", firstR)
	}
	lastStr, _, _ := screen.Get(39, 0)
	lastR, _ := utf8.DecodeRuneInString(lastStr)
	if lastR != 's' {
		t.Fatalf("last visible rune = %q, want 's' (first 19 letters)", lastR)
	}
}
