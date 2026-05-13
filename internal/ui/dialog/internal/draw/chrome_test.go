package draw

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawDialogFrameCentersTitleInTopBorder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	rect := Rect{X: 10, Y: 3, Width: 50, Height: 8}
	title := "Create hardlink"
	DrawDialogFrame(screen, rect, title, theme.Default())

	gotUL, _, _ := screen.Get(rect.X, rect.Y)
	gotUR, _, _ := screen.Get(rect.X+rect.Width-1, rect.Y)
	if gotUL != "┌" || gotUR != "┐" {
		t.Fatalf("corners = %q %q, want ┌ and ┐", gotUL, gotUR)
	}

	inner := tcelltest.TextAt(screen, rect.X+1, rect.Y, rect.Width-2)
	innerW := rect.Width - 2
	padded := " " + strings.TrimSpace(title) + " "
	tlen := utf8.RuneCountInString(padded)
	if tlen > innerW {
		t.Fatalf("title block wider than inner row")
	}
	leftPad := (innerW - tlen) / 2
	rightPad := innerW - leftPad - tlen
	want := strings.Repeat("─", leftPad) + padded + strings.Repeat("─", rightPad)
	if inner != want {
		t.Fatalf("top inner row = %q\nwant          %q", inner, want)
	}
}

func TestEnsureScrollInputVisible(t *testing.T) {
	tests := []struct {
		name                   string
		length, cursor, scroll int
		width                  int
		wantCursor, wantScroll int
	}{
		{name: "empty/short text", length: 0, cursor: 0, scroll: 0, width: 10, wantCursor: 0, wantScroll: 0},
		{name: "cursor in window keeps scroll", length: 20, cursor: 5, scroll: 0, width: 10, wantCursor: 5, wantScroll: 0},
		{name: "cursor past right scrolls right", length: 20, cursor: 12, scroll: 0, width: 10, wantCursor: 12, wantScroll: 3},
		{name: "cursor before left scrolls left", length: 20, cursor: 2, scroll: 10, width: 10, wantCursor: 2, wantScroll: 2},
		{name: "cursor at end reserves trailing cell", length: 20, cursor: 20, scroll: 0, width: 10, wantCursor: 20, wantScroll: 11},
		{name: "cursor clamped to length", length: 5, cursor: 99, scroll: 0, width: 10, wantCursor: 5, wantScroll: 0},
		{name: "negative cursor clamped to 0", length: 5, cursor: -3, scroll: 0, width: 10, wantCursor: 0, wantScroll: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, s := EnsureScrollInputVisible(tc.length, tc.cursor, tc.scroll, tc.width)
			if c != tc.wantCursor || s != tc.wantScroll {
				t.Fatalf("EnsureScrollInputVisible(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
					tc.length, tc.cursor, tc.scroll, tc.width, c, s, tc.wantCursor, tc.wantScroll)
			}
		})
	}
}

func TestDrawScrollingDialogInputShowsTailWhenCursorAtEnd(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 10)

	value := "0123456789ABCDEFGHIJ" // 20 chars
	width := 10
	cursor := utf8.RuneCountInString(value)
	DrawScrollingDialogInput(screen, 2, 2, width, value, cursor, 0, true, false, theme.Default())

	got := tcelltest.TextAt(screen, 2, 2, width)
	want := "◀CDEFGHIJ " // scroll=11 hides 0..A; overflow marker on first cell, cursor blank tail
	if got != want {
		t.Fatalf("visible row = %q want %q", got, want)
	}
}

func TestDrawDialogFrameCentersShortTitle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)

	rect := Rect{X: 5, Y: 2, Width: 40, Height: 6}
	DrawDialogFrame(screen, rect, "Copy", theme.Default())

	inner := tcelltest.TextAt(screen, rect.X+1, rect.Y, rect.Width-2)
	innerW := rect.Width - 2
	padded := " Copy "
	tlen := utf8.RuneCountInString(padded)
	leftPad := (innerW - tlen) / 2
	rightPad := innerW - leftPad - tlen
	want := strings.Repeat("─", leftPad) + padded + strings.Repeat("─", rightPad)
	if inner != want {
		t.Fatalf("top inner row = %q\nwant          %q", inner, want)
	}
}
