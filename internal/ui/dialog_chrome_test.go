package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
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
	drawDialogFrame(screen, rect, title, theme.Default())

	gotUL, _, _ := screen.Get(rect.X, rect.Y)
	gotUR, _, _ := screen.Get(rect.X+rect.Width-1, rect.Y)
	if gotUL != "┌" || gotUR != "┐" {
		t.Fatalf("corners = %q %q, want ┌ and ┐", gotUL, gotUR)
	}

	inner := textAt(screen, rect.X+1, rect.Y, rect.Width-2)
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

func TestDrawDialogFrameCentersShortTitle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)

	rect := Rect{X: 5, Y: 2, Width: 40, Height: 6}
	drawDialogFrame(screen, rect, "Copy", theme.Default())

	inner := textAt(screen, rect.X+1, rect.Y, rect.Width-2)
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
