package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestDrawFilterDialogTypeColumnsBesideRadios(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	const w, h = 62, 24
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
	steps := []struct {
		name  string
		focus dlg.ID
		key   tcell.Key
		mode  panel.GroupPatternMode
		meta  int
		want  dlg.ID
	}{
		{"radio right to type", PatternShellRadio, tcell.KeyRight, shell, 0, videos},
		{"type right to audio col", videos, tcell.KeyRight, shell, 0, audio},
		{"type left back to radio", videos, tcell.KeyLeft, shell, 0, PatternShellRadio},
		{"down keeps type column", videos, tcell.KeyDown, shell, 0, PatternTypeID(FilterTypePictures)},
		{"down from empty cell falls left", documents, tcell.KeyDown, shell, 0, archives},
		{"down from archives to pattern", archives, tcell.KeyDown, shell, 0, PatternInput},
		{"up from pattern to simple radio", PatternInput, tcell.KeyUp, shell, 0, PatternSimpleRadio},
		{"dirs down to case", PatternDirsOnly, tcell.KeyDown, shell, 0, PatternCase},
		{"dirs down to case in regex", PatternDirsOnly, tcell.KeyDown, regex, 0, PatternCase},
		{"invert right to case", PatternInvert, tcell.KeyRight, shell, 0, PatternCase},
		{"invert right to case in regex", PatternInvert, tcell.KeyRight, regex, 0, PatternCase},
		{"invert down to OK", PatternInvert, tcell.KeyDown, regex, 0, dlg.OK},
		{"case down to meta", PatternCase, tcell.KeyDown, shell, 2, PatternOnlyMeta},
		{"OK up to the last row, the cell drawn above it", dlg.OK, tcell.KeyUp, shell, 0, PatternCase},
		{"OK up to meta row", dlg.OK, tcell.KeyUp, shell, 2, PatternIncludeMeta},
		{"OK right to Cancel", dlg.OK, tcell.KeyRight, shell, 0, dlg.Cancel},
		{"first row up nothing", videos, tcell.KeyUp, shell, 0, videos},
		{"tab radios to types", PatternRegexRadio, tcell.KeyTab, shell, 0, videos},
		{"tab types to pattern", documents, tcell.KeyTab, shell, 0, PatternInput},
		{"tab pattern to options", PatternInput, tcell.KeyTab, shell, 0, PatternFilesOnly},
		{"tab options to buttons", PatternInvert, tcell.KeyTab, shell, 0, dlg.OK},
		{"tab buttons wraps", dlg.Cancel, tcell.KeyTab, shell, 0, PatternShellRadio},
		{"backtab pattern to types", PatternInput, tcell.KeyBacktab, shell, 0, videos},
		{"backtab radios wraps to buttons", PatternShellRadio, tcell.KeyBacktab, shell, 0, dlg.OK},
	}
	for _, s := range steps {
		if got := patternStep(view(s.mode, s.meta), s.focus, s.key); got != s.want {
			t.Errorf("%s: got %q, want %q", s.name, got, s.want)
		}
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
// mnemonics or clicked while Directories only is on.
func TestFilterDisabledTypes(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	st := FilterDialogState{PatternMode: panel.GroupPatternShell, DirsOnly: true}
	spec := FilterDialogView(st, layout, theme.Default())
	if got := patternStep(spec, PatternShellRadio, tcell.KeyRight); got != PatternShellRadio {
		t.Errorf("Right from a radio over disabled types = %q, want to stay", got)
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
