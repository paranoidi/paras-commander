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
		{name: "cursor past right scrolls right", length: 20, cursor: 12, scroll: 0, width: 10, wantCursor: 12, wantScroll: 4},
		{name: "cursor before left scrolls left", length: 20, cursor: 2, scroll: 10, width: 10, wantCursor: 2, wantScroll: 2},
		{name: "cursor at end reserves trailing cell", length: 20, cursor: 20, scroll: 0, width: 10, wantCursor: 20, wantScroll: 12},
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

func TestAdjustScrollForCompletionShiftsLeftForSuffix(t *testing.T) {
	const width = 8
	valueLen := 10
	cursor := 10
	suffixLen := 4
	_, scroll := AdjustScrollForCompletion(valueLen, cursor, 0, width, suffixLen)
	// suffixEnd=14 must be visible: need scroll > 14-width=6
	if scroll < 7 {
		t.Fatalf("scroll = %d, want >= 7 to reveal suffix", scroll)
	}
}

func TestAdjustScrollRevealOnEraseFitsToZero(t *testing.T) {
	value := "~/synthetic/workspace/catalog"
	runes := []rune(value)
	width := len(runes) + 2
	_, scroll := AdjustScrollRevealOnErase(value, len(runes), len(runes)-1, width, 0)
	if scroll != 0 {
		t.Fatalf("scroll = %d want 0 when text fits", scroll)
	}
}

func TestAdjustScrollRevealOnEraseFitsToZeroDespiteLongGhostSuffix(t *testing.T) {
	value := "/synthetic/volume/catalog/branches/widget/12"
	width := 72
	_, scroll := AdjustScrollRevealOnErase(value, len([]rune(value)), len([]rune(value)), width, 40)
	if scroll != 0 {
		t.Fatalf("scroll = %d want 0 when committed value fits width", scroll)
	}
}

func TestEnsurePathInputScrollShowsTailAtEnd(t *testing.T) {
	valueLen := 120
	width := 40
	cursor := valueLen
	_, scroll := EnsurePathInputScroll(valueLen, cursor, 0, width, 0)
	_, want := EnsureScrollInputVisible(valueLen, cursor, 0, width)
	if scroll != want {
		t.Fatalf("scroll = %d want %d for cursor at EOT", scroll, want)
	}
}

func TestAdjustScrollForCompletionKeepsZeroWhenValueFits(t *testing.T) {
	valueLen := len([]rune("/synthetic/volume/catalog/branches/widget/12"))
	_, scroll := AdjustScrollForCompletion(valueLen, valueLen, 40, 72, 30)
	if scroll != 0 {
		t.Fatalf("scroll = %d want 0 when value fits despite ghost suffix", scroll)
	}
}

func TestAdjustScrollRevealOnEraseLongPathScroll35(t *testing.T) {
	value := "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/~/synthetic/workspace/catalog/"
	cursor := len([]rune(value))
	_, scroll := AdjustScrollRevealOnErase(value, cursor, 35, 72, 0)
	if scroll >= 35 {
		t.Fatalf("scroll = %d want < 35", scroll)
	}
}

func TestAdjustScrollRevealOnEraseStepsWord(t *testing.T) {
	value := "/foo/bar/baz"
	runes := []rune(value)
	scroll := len(runes) - 1
	width := 4
	_, got := AdjustScrollRevealOnErase(value, scroll, scroll, width, 0)
	want := 9 // BackwardWordIndex from scroll at trailing "baz"
	if got != want {
		t.Fatalf("scroll = %d want %d", got, want)
	}
}

func TestShouldPreemptiveScrollRevealOnEraseLastVisible(t *testing.T) {
	valueLen := 20
	width := 8
	scroll := 12
	cursor := 20
	if !ShouldPreemptiveScrollRevealOnErase(valueLen, cursor, scroll, width, 0, true) {
		t.Fatal("expected preemptive when backspacing last visible rune")
	}
}

func TestShouldPreemptiveScrollRevealOnEraseLastValueWithGhostSuffix(t *testing.T) {
	valueLen := 10
	cursor := 10
	scroll := 7
	width := 8
	suffixLen := 4
	if !ShouldPreemptiveScrollRevealOnErase(valueLen, cursor, scroll, width, suffixLen, true) {
		t.Fatal("expected preemptive when backspacing last visible value rune before ghost tail")
	}
}

func TestShouldPreemptiveScrollRevealOnEraseHiddenPrefix(t *testing.T) {
	if !ShouldPreemptiveScrollRevealOnErase(20, 5, 10, 8, 0, true) {
		t.Fatal("expected preemptive when backspacing rune left of viewport")
	}
}

func TestAdjustScrollRevealOnEraseAfterCompletionAtEnd(t *testing.T) {
	value := "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/~/synthetic/workspace/catalog/"
	runes := []rune(value)
	width := 72
	cursor := len(runes)
	scroll, _ := AdjustScrollForCompletion(len(runes), cursor, 0, width, 0)
	if scroll == 0 {
		t.Fatal("completion adjust should scroll for long path")
	}
	_, revealScroll := AdjustScrollRevealOnErase(value, cursor, scroll, width, 0)
	if revealScroll >= scroll {
		t.Fatalf("reveal scroll = %d want < %d after completion at %d", revealScroll, scroll, scroll)
	}
}

func TestAdjustScrollForCompletionKeepsScrollWhenSuffixCleared(t *testing.T) {
	const width = 8
	valueLen := 10
	cursor := 10
	_, scrollWith := AdjustScrollForCompletion(valueLen, cursor, 0, width, 4)
	_, scrollAfter := AdjustScrollForCompletion(valueLen, cursor, scrollWith, width, 0)
	if scrollAfter != scrollWith {
		t.Fatalf("scroll after clear = %d, want unchanged %d", scrollAfter, scrollWith)
	}
}

func TestDrawScrollingDialogInputGhostSuffixUsesPlaceholderStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 10)

	th := theme.Default()
	_, wantPhFG, _ := th.DialogInputActivePlaceholder.Decompose()
	value := "/a"
	suffix := "bc"
	cursor := len([]rune(value))
	DrawScrollingDialogInput(screen, 2, 2, 10, value, cursor, 0, suffix, true, false, th)

	// Last ghost rune (caret cell uses reversed committed style, not placeholder).
	ghostCol := 2 + cursor + len([]rune(suffix)) - 1
	gotStr, gotSt, _ := screen.Get(ghostCol, 2)
	gotFG, _, _ := gotSt.Decompose()
	if gotFG != wantPhFG {
		t.Fatalf("ghost cell fg = %v want placeholder %v", gotFG, wantPhFG)
	}
	if gotStr != "c" {
		t.Fatalf("ghost cell ch = %q want c", gotStr)
	}
}

func TestOverflowMarkersNeverUseErrorStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 10)

	th := theme.Default()
	value := "0123456789ABCDEFGHIJ"
	DrawScrollingDialogInput(screen, 2, 2, 10, value, 5, 0, "", true, true, th)

	_, st, _ := screen.Get(2+9, 2)
	errFG, _, _ := th.DialogInputActiveError.Decompose()
	gotFG, _, _ := st.Decompose()
	if gotFG == errFG {
		t.Fatal("right overflow marker must not use error foreground")
	}
}

func TestOverflowMarkerRightVisibleWithCursorInLastTextCell(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 10)

	value := "0123456789ABCDEFGHIJ"
	width := 10
	scroll := 8
	cursor := 15
	DrawScrollingDialogInput(screen, 2, 2, width, value, cursor, scroll, "", true, false, theme.Default())

	got, _, _ := screen.Get(2+width-1, 2)
	if got != "▶" {
		t.Fatalf("right cell = %q want ▶", got)
	}
	gotRow := tcelltest.TextAt(screen, 2, 2, width)
	if !strings.HasSuffix(gotRow, "▶") {
		t.Fatalf("row = %q; ▶ missing at end", gotRow)
	}
}

func TestScrollContentLenExtraColumnAtEnd(t *testing.T) {
	if got := ScrollContentLen(30, 30); got != 31 {
		t.Fatalf("ScrollContentLen(30,30) = %d want 31", got)
	}
	if got := ScrollContentLen(30, 29); got != 30 {
		t.Fatalf("ScrollContentLen(30,29) = %d want 30", got)
	}
}

func TestEnsureScrollInputVisibleCaretPastLastRuneInViewport(t *testing.T) {
	value := "catalog.release-" + strings.Repeat("Y", 20) + "/"
	width := 16
	valueLen := len([]rune(value))
	cursor := valueLen
	_, scroll := EnsureScrollInputVisible(valueLen, cursor, 0, width)
	lay := ScrollingInputLayoutFor(scroll, width, ScrollContentLen(valueLen, cursor))
	if cursor >= scroll+lay.TextCols {
		t.Fatalf("caret %d not in [%d,%d) at scroll %d", cursor, scroll, scroll+lay.TextCols, scroll)
	}
}

func TestDrawScrollingDialogInputShowsRightMarkerWhenTailStillHidden(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(60, 10)

	value := strings.Repeat("x", 79) + "/"
	width := 16
	valueLen := len([]rune(value))
	cursor := 40
	_, scroll := EnsureScrollInputVisible(valueLen, cursor, 0, width)
	lay := ScrollingInputLayoutFor(scroll, width, ScrollContentLen(valueLen, cursor))
	if lay.RightPad == 0 {
		t.Fatalf("expected hidden tail at scroll %d", scroll)
	}
	DrawScrollingDialogInput(screen, 2, 2, width, value, cursor, scroll, "", true, false, theme.Default())

	got, _, _ := screen.Get(2+width-1, 2)
	if got != "▶" {
		t.Fatalf("right cell = %q want ▶ at scroll %d", got, scroll)
	}
}

func TestDrawScrollingDialogInputTailVisibleWhenCaretAfterLastRune(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(60, 10)

	value := "catalog.release-" + strings.Repeat("Y", 20) + "/"
	width := 16
	runes := []rune(value)
	cursor := len(runes)
	_, scroll := EnsureScrollInputVisible(cursor, cursor, 0, width)
	DrawScrollingDialogInput(screen, 2, 2, width, value, cursor, scroll, "", true, false, theme.Default())

	gotRow := tcelltest.TextAt(screen, 2, 2, width)
	if !strings.Contains(gotRow, string(runes[len(runes)-1])) {
		t.Fatalf("row %q must include trailing %q", gotRow, runes[len(runes)-1])
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
	DrawScrollingDialogInput(screen, 2, 2, width, value, cursor, 0, "", true, false, theme.Default())

	got := tcelltest.TextAt(screen, 2, 2, width)
	want := "◀CDEFGHIJ " // scroll=11 hides 0..A; overflow marker on first cell, cursor blank tail
	if got != want {
		t.Fatalf("visible row = %q want %q", got, want)
	}
}

func TestDrawDialogFrameTitleTextUsesFrameForegroundAndSurfaceBackground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)

	th := theme.Default()
	bfg, _, _ := th.DialogFrame.Decompose()
	_, dbg, _ := th.DialogSurface.Decompose()
	wantSt := th.DialogTitle.Foreground(bfg).Background(dbg)

	rect := Rect{X: 5, Y: 2, Width: 40, Height: 6}
	title := "Hi"
	DrawDialogFrame(screen, rect, title, th)

	innerW := rect.Width - 2
	tr := strings.TrimSpace(title)
	var titleRunes []rune
	titleRunes = append(append(titleRunes, ' '), []rune(tr)...)
	titleRunes = append(titleRunes, ' ')
	tlen := len(titleRunes)
	leftPad := (innerW - tlen) / 2
	x := rect.X + 1 + leftPad + 1

	str, st, width := screen.Get(x, rect.Y)
	r, _ := utf8.DecodeRuneInString(str)
	if r != 'H' || width < 1 {
		inner := tcelltest.TextAt(screen, rect.X+1, rect.Y, rect.Width-2)
		t.Fatalf("at x=%d want H width>=1; rune=%q str=%q width=%d inner=%q", x, r, str, width, inner)
	}
	if st != wantSt {
		t.Fatalf("title cell style = %v, want composed title style %v", st, wantSt)
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
