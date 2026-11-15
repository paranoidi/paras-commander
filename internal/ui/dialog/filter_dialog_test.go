package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestDrawFilterDialogGrid(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	const w, h = 80, 24
	screen.SetSize(w, h)
	DrawFilterDialog(screen, Layout{Width: w, Height: h}, FilterDialogState{Open: true, PatternMode: panel.GroupPatternShell, Focus: PatternInput}, theme.Default())

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
	// The three modes share one row; the types take three columns under them, and the scope
	// radios the same three columns.
	simple := rows[find("Simple / auto shell")]
	if !strings.Contains(simple, "Shell patterns") || !strings.Contains(simple, "Regular expression") {
		t.Fatalf("mode row %q missing a mode", simple)
	}
	videos := rows[find("Videos")]
	if !strings.Contains(videos, "Pictures") || !strings.Contains(videos, "Archives") {
		t.Fatalf("first type row %q missing Pictures/Archives", videos)
	}
	if y := find("Audio"); !strings.Contains(rows[y], "Documents") {
		t.Fatalf("second type row %q missing Documents", rows[y])
	}
	files := rows[find("Files")]
	col := func(row, label string) int { return len([]rune(row[:strings.Index(row, label)])) }
	for _, c := range [][2]string{{"Shell patterns", "Pictures"}, {"Regular expression", "Archives"}} {
		if col(simple, c[0]) != col(videos, c[1]) {
			t.Errorf("%s and %s are not in one column:\n%s\n%s", c[0], c[1], simple, videos)
		}
	}
	for _, c := range [][2]string{{"Pictures", "Dirs"}, {"Archives", "Both"}} {
		if col(videos, c[0]) != col(files, c[1]) {
			t.Errorf("%s and %s are not in one column:\n%s\n%s", c[0], c[1], videos, files)
		}
	}
	// Right margin: the char before the right border is blank on the widest row.
	trimmed := strings.TrimRight(simple, " ")
	runes := []rune(trimmed)
	if len(runes) < 3 || string(runes[len(runes)-1]) != "│" || string(runes[len(runes)-2]) != " " {
		t.Fatalf("simple row lost its right margin: %q", simple)
	}
	// The button strip sits directly under its separator (no blank row between them).
	btn := find("OK")
	if strings.Trim(rows[btn-1], " ─├┤") != "" {
		t.Fatalf("want the separator directly above the buttons:\n%q", rows[btn-1])
	}
}

// patternStep presses key with focus on from and returns where focus lands.
func patternStep(spec dlg.Spec, from dlg.ID, key tcell.Key) dlg.ID {
	focus := from
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(key, 0, tcell.ModNone))
	return focus
}

func TestFilterFocusGrid(t *testing.T) {
	shell, regex := panel.GroupPatternShell, panel.GroupPatternRegex
	view := func(mode panel.GroupPatternMode, meta int) dlg.Spec {
		return FilterDialogView(FilterDialogState{PatternMode: mode, MetaColumnCount: meta}, Layout{Width: 80, Height: 24}, theme.Default())
	}
	videos, audio, archives, documents := PatternTypeID(FilterTypeVideos), PatternTypeID(FilterTypeAudio), PatternTypeID(FilterTypeArchives), PatternTypeID(FilterTypeDocuments)
	pictures := PatternTypeID(FilterTypePictures)
	steps := []struct {
		name  string
		focus dlg.ID
		key   tcell.Key
		mode  panel.GroupPatternMode
		meta  int
		want  dlg.ID
	}{
		{"radio right to next radio", PatternSimpleRadio, tcell.KeyRight, shell, 0, PatternShellRadio},
		{"radio down to types", PatternSimpleRadio, tcell.KeyDown, shell, 0, videos},
		{"type right to next col", videos, tcell.KeyRight, shell, 0, pictures},
		{"type left back to first col", pictures, tcell.KeyLeft, shell, 0, videos},
		{"types up to radios", videos, tcell.KeyUp, shell, 0, PatternSimpleRadio},
		{"down keeps type column", videos, tcell.KeyDown, shell, 0, audio},
		{"down from empty cell falls left", archives, tcell.KeyDown, shell, 0, documents},
		{"down from last type row to pattern", audio, tcell.KeyDown, shell, 0, PatternInput},
		{"up from pattern to audio", PatternInput, tcell.KeyUp, shell, 0, audio},
		{"dirs down to case", PatternScopeDirs, tcell.KeyDown, shell, 0, PatternCase},
		{"dirs down to case in regex", PatternScopeDirs, tcell.KeyDown, regex, 0, PatternCase},
		{"invert right to case", PatternInvert, tcell.KeyRight, shell, 0, PatternCase},
		{"invert right to case in regex", PatternInvert, tcell.KeyRight, regex, 0, PatternCase},
		{"invert down to OK", PatternInvert, tcell.KeyDown, regex, 0, dlg.OK},
		{"case down to meta", PatternCase, tcell.KeyDown, shell, 2, PatternOnlyMeta},
		{"OK up to the last row, the cell drawn above it", dlg.OK, tcell.KeyUp, shell, 0, PatternCase},
		{"OK up to meta row, the cell drawn above it", dlg.OK, tcell.KeyUp, shell, 2, PatternOnlyMeta},
		{"OK right to Cancel", dlg.OK, tcell.KeyRight, shell, 0, dlg.Cancel},
		{"first row up nothing", PatternSimpleRadio, tcell.KeyUp, shell, 0, PatternSimpleRadio},
		{"tab radios to types", PatternRegexRadio, tcell.KeyTab, shell, 0, videos},
		{"tab types to pattern", documents, tcell.KeyTab, shell, 0, PatternInput},
		{"tab pattern to options", PatternInput, tcell.KeyTab, shell, 0, PatternScopeFiles},
		{"tab options to buttons", PatternInvert, tcell.KeyTab, shell, 0, dlg.OK},
		{"tab buttons wraps", dlg.Cancel, tcell.KeyTab, shell, 0, PatternSimpleRadio},
		{"backtab pattern to types", PatternInput, tcell.KeyBacktab, shell, 0, videos},
		{"backtab radios wraps to buttons", PatternSimpleRadio, tcell.KeyBacktab, shell, 0, dlg.OK},
	}
	for _, s := range steps {
		if got := patternStep(view(s.mode, s.meta), s.focus, s.key); got != s.want {
			t.Errorf("%s: got %q, want %q", s.name, got, s.want)
		}
	}
}

// Clear is shown only while the target has a filter, after Cancel, with its Alt letter.
func TestFilterClearButtonWhileFilterActive(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	button := func(spec dlg.Spec) (dlg.Button, bool) {
		for _, b := range spec.Buttons {
			if b.ID == PatternClearButton {
				return b, true
			}
		}
		return dlg.Button{}, false
	}
	if _, ok := button(FilterDialogView(FilterDialogState{}, layout, theme.Default())); ok {
		t.Fatal("Clear must be hidden without an active filter")
	}
	spec := FilterDialogView(FilterDialogState{FilterActive: true}, layout, theme.Default())
	if b, ok := button(spec); !ok || b.Mnemonic != PatternMnemonicClear {
		t.Fatalf("Clear button = %+v %v, want shown with mnemonic %q", b, ok, PatternMnemonicClear)
	}
	if got := patternStep(spec, dlg.Cancel, tcell.KeyRight); got != PatternClearButton {
		t.Fatalf("Cancel right = %q, want Clear", got)
	}
	focus := PatternInput
	ev, ok := dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if !ok || ev.ID != PatternClearButton {
		t.Fatalf("Alt+F = %+v %v, want the Clear button", ev, ok)
	}
	if PatternAltIsMnemonic('f') {
		t.Fatal("Alt+F must stay forward-word in the pattern input while Clear is hidden")
	}
}

func TestFilterCompareHidesFilesDirs(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	st := FilterDialogState{PatternMode: panel.GroupPatternShell, Target: FilterTargetCompare}
	spec := FilterDialogView(st, layout, theme.Default())
	if got := patternStep(spec, PatternInput, tcell.KeyDown); got != PatternInvert {
		t.Fatalf("Down from pattern in compare = %q, want Invert", got)
	}
	if got := patternStep(spec, PatternInput, tcell.KeyTab); got != PatternInvert {
		t.Fatalf("Tab from pattern in compare = %q, want Invert", got)
	}
	height := func(st FilterDialogState) int {
		p, ok := dlg.Place(layout.Width, layout.Height, FilterDialogView(st, layout, theme.Default()))
		if !ok {
			t.Fatal("dialog does not fit")
		}
		return p.Rect.Height
	}
	if height(st) != height(FilterDialogState{PatternMode: panel.GroupPatternShell})-1 {
		t.Fatal("compare dialog should be one row shorter")
	}
}

// TestFilterDisabledTypes: the type checkboxes cannot be focused, toggled, reached by their
// mnemonics or clicked while the scope is Dirs.
func TestFilterDisabledTypes(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	st := FilterDialogState{PatternMode: panel.GroupPatternShell, Scope: panel.ScopeDirs}
	spec := FilterDialogView(st, layout, theme.Default())
	if got := patternStep(spec, PatternSimpleRadio, tcell.KeyDown); got != PatternInput {
		t.Errorf("Down from the radios over disabled types = %q, want the pattern", got)
	}
	if got := patternStep(spec, PatternShellRadio, tcell.KeyTab); got != PatternInput {
		t.Errorf("Tab from the radios = %q, want the pattern (types are skipped)", got)
	}
	focus := PatternInput
	if ev, ok := dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModAlt)); ok || ev.ID != "" {
		t.Errorf("Alt+V on a disabled type = (%q,%v), want unhandled", ev.ID, ok)
	}
	if st.Apply(dlg.Event{ID: PatternTypeID(FilterTypeVideos)}) || st.Types[FilterTypeVideos] {
		t.Error("a disabled type must not toggle")
	}
}
