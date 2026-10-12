package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawFilterDialogTypeColumnsBesideRadios(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	const w, h = 62, 24
	screen.SetSize(w, h)
	DrawFilterDialog(screen, Layout{Width: w, Height: h}, FilterDialogState{Open: true, PatternMode: panel.GroupPatternShell, Focus: FilterFocusPattern}, theme.Default())

	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			ch, _, _ := screen.Get(x, y)
			if ch == "" {
				ch = " "
			}
			sb.WriteString(ch)
		}
		rows[y] = sb.String()
	}
	find := func(sub string) int {
		for y, r := range rows {
			if strings.Contains(r, sub) {
				return y
			}
		}
		t.Fatalf("no row contains %q:\n%s", sub, strings.Join(rows, "\n"))
		return -1
	}
	reg := rows[find("Regular expression")]
	for _, want := range []string{"Pictures", "Documents"} {
		if !strings.Contains(reg, want) {
			t.Fatalf("regex row %q missing %q", reg, want)
		}
	}
	if y := find("Shell patterns"); !strings.Contains(rows[y], "Videos") || !strings.Contains(rows[y], "Audio") {
		t.Fatalf("shell row %q missing Videos/Audio", rows[y])
	}
	if y := find("Simple"); !strings.Contains(rows[y], "Archives") {
		t.Fatalf("simple row %q missing Archives", rows[y])
	}
	// Right margin: the char before the right border is blank on the widest row.
	trimmed := strings.TrimRight(reg, " ")
	runes := []rune(trimmed)
	if len(runes) < 3 || string(runes[len(runes)-1]) != "│" || string(runes[len(runes)-2]) != " " {
		t.Fatalf("regex row lost its right margin: %q", reg)
	}
	// Exactly one blank inner row above the buttons.
	btn := find("OK")
	if strings.Trim(rows[btn-1], " │") != "" || strings.Trim(rows[btn-2], " │─├┤") != "" {
		t.Fatalf("want blank row then separator above buttons:\n%q\n%q", rows[btn-2], rows[btn-1])
	}
}

func TestFilterMoveFocusGrid(t *testing.T) {
	shell, regex := panel.GroupPatternShell, panel.GroupPatternRegex
	step := func(focus int, key tcell.Key, mode panel.GroupPatternMode, meta int) int {
		got, _ := FilterMoveFocus(focus, key, FilterDialogState{PatternMode: mode, MetaColumnCount: meta})
		return got
	}
	okIdx := FilterContentCount(0)
	steps := []struct {
		name  string
		focus int
		key   tcell.Key
		mode  panel.GroupPatternMode
		meta  int
		want  int
	}{
		{"radio right to type", FilterFocusShellRadio, tcell.KeyRight, shell, 0, FilterFocusTypeFirst},
		{"type right to audio col", FilterFocusTypeFirst, tcell.KeyRight, shell, 0, FilterFocusTypeFirst + 3},
		{"type left back to radio", FilterFocusTypeFirst, tcell.KeyLeft, shell, 0, FilterFocusShellRadio},
		{"down keeps type column", FilterFocusTypeFirst, tcell.KeyDown, shell, 0, FilterFocusTypeFirst + 1},
		{"down from empty cell falls left", FilterFocusTypeFirst + 4, tcell.KeyDown, shell, 0, FilterFocusTypeFirst + 2},
		{"down from archives to pattern", FilterFocusTypeFirst + 2, tcell.KeyDown, shell, 0, FilterFocusPattern},
		{"up from pattern to simple radio", FilterFocusPattern, tcell.KeyUp, shell, 0, FilterFocusSimpleRadio},
		{"dirs down to case", FilterFocusDirsOnly, tcell.KeyDown, shell, 0, FilterFocusCase},
		{"dirs down to case in regex", FilterFocusDirsOnly, tcell.KeyDown, regex, 0, FilterFocusCase},
		{"invert right to case", FilterFocusInvert, tcell.KeyRight, shell, 0, FilterFocusCase},
		{"invert right to case in regex", FilterFocusInvert, tcell.KeyRight, regex, 0, FilterFocusCase},
		{"invert down to OK", FilterFocusInvert, tcell.KeyDown, regex, 0, okIdx},
		{"case down to meta", FilterFocusCase, tcell.KeyDown, shell, 2, FilterFocusOnlyMeta},
		{"OK up to last row", okIdx, tcell.KeyUp, shell, 0, FilterFocusInvert},
		{"OK right to Cancel", okIdx, tcell.KeyRight, shell, 0, okIdx + 1},
		{"first row up nothing", FilterFocusTypeFirst, tcell.KeyUp, shell, 0, FilterFocusTypeFirst},
		{"tab radios to types", FilterFocusRegexRadio, tcell.KeyTab, shell, 0, FilterFocusTypeFirst},
		{"tab types to pattern", FilterFocusTypeFirst + 4, tcell.KeyTab, shell, 0, FilterFocusPattern},
		{"tab pattern to options", FilterFocusPattern, tcell.KeyTab, shell, 0, FilterFocusFilesOnly},
		{"tab options to buttons", FilterFocusInvert, tcell.KeyTab, shell, 0, okIdx},
		{"tab buttons wraps", okIdx + 1, tcell.KeyTab, shell, 0, FilterFocusShellRadio},
		{"backtab pattern to types", FilterFocusPattern, tcell.KeyBacktab, shell, 0, FilterFocusTypeFirst},
		{"backtab radios wraps to buttons", FilterFocusShellRadio, tcell.KeyBacktab, shell, 0, okIdx},
	}
	for _, s := range steps {
		if got := step(s.focus, s.key, s.mode, s.meta); got != s.want {
			t.Errorf("%s: got %d, want %d", s.name, got, s.want)
		}
	}
}

func TestFilterMoveFocusCompareHidesFilesDirs(t *testing.T) {
	st := FilterDialogState{PatternMode: panel.GroupPatternShell, Target: FilterTargetCompare}
	if got, _ := FilterMoveFocus(FilterFocusPattern, tcell.KeyDown, st); got != FilterFocusInvert {
		t.Fatalf("Down from pattern in compare = %d, want Invert %d", got, FilterFocusInvert)
	}
	if got, _ := FilterMoveFocus(FilterFocusPattern, tcell.KeyTab, st); got != FilterFocusInvert {
		t.Fatalf("Tab from pattern in compare = %d, want Invert %d", got, FilterFocusInvert)
	}
	if filterDialogInnerRows(st) != filterDialogInnerRows(FilterDialogState{PatternMode: panel.GroupPatternShell})-1 {
		t.Fatal("compare dialog should be one row shorter")
	}
}
