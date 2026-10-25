package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the command output dialog: finished and running (Background / Cancel focus,
// queued) states, short and overflowing output at several scroll positions, follow mode, ANSI
// colored and clipped lines, detected path matches (cursor, selected, at the line end), title
// variants, configured sizes and narrow / short screens.

// setCommandOutputGoldenFocus selects the running-state button: "background" or "cancel". It and
// commandOutputGoldenFocusName are the only code that knows how the state encodes focus.
func setCommandOutputGoldenFocus(st *CommandOutputDialogState, focus string) {
	st.Focus = map[string]dlg.ID{"background": OutputBackground, "cancel": OutputCancel}[focus]
}

func commandOutputGoldenLines(n int) []string {
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor"}
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %02d %s %s", i, words[i%len(words)], words[(i*3+1)%len(words)])
	}
	return out
}

func commandOutputGoldenState(lines []string, scroll int, follow bool) CommandOutputDialogState {
	return CommandOutputDialogState{Open: true, Title: "Build willow", Lines: lines, Scroll: scroll, Follow: follow}
}

func TestCommandOutputGoldenRender(t *testing.T) {
	goldenBandRows["command_output"] = 40
	var cases []simpleGolden
	selected := func(p string) bool { return strings.HasSuffix(p, "harbor.txt") }
	add := func(name string, w, h int, st CommandOutputDialogState, styles theme.Theme) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawCommandOutputDialog(s, l, st, styles, uiscrollbar.StyleThumb, selected)
		}})
	}
	ascii := asciiDialogStyles()
	nerd := theme.Default()
	nerd.UseNerdfontIcons = true

	// finished: empty, short, exactly filling (9 rows at 80x24), overflowing at several scrolls
	add("done_empty", 80, 24, commandOutputGoldenState(nil, 0, false), ascii)
	add("done_3_lines", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(3), 0, false), ascii)
	add("done_9_lines", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(9), 0, false), ascii)
	add("done_10_lines_top", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(10), 0, false), ascii)
	add("done_10_lines_bottom", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(10), 1, false), ascii)
	for _, scroll := range []int{0, 1, 20, 40, 41, 60} {
		add(fmt.Sprintf("done_50_lines_scroll%d", scroll), 80, 24, commandOutputGoldenState(commandOutputGoldenLines(50), scroll, false), ascii)
	}
	add("done_50_lines_follow", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(50), 0, true), ascii)
	add("done_3_lines_follow", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(3), 0, true), ascii)
	add("done_1000_lines_scroll500", 80, 24, commandOutputGoldenState(commandOutputGoldenLines(1000), 500, false), ascii)

	// long, ANSI colored and empty lines
	long := commandOutputGoldenLines(6)
	long[1] = strings.Repeat("harbor-lantern-", 12)
	long[3] = ""
	long[4] = "\x1b[31mred\x1b[0m plain \x1b[1;32mbold green\x1b[0m \x1b[4munderline\x1b[0m tail"
	long[5] = "tab\tseparated\tcolumns and 日本語 wide runes"
	add("done_long_ansi", 80, 24, commandOutputGoldenState(long, 0, false), ascii)
	add("done_long_ansi_100", 100, 40, commandOutputGoldenState(long, 0, false), ascii)

	// path matches: the cursor, a selected one, one reaching the end of its line
	pst := commandOutputGoldenState([]string{
		"built /srv/harbor/willow.txt ok", "nothing here", "see /srv/harbor/harbor.txt", "/srv/amber/lantern and /srv/amber/dune",
		"last /srv/ember", "tail",
	}, 0, false)
	pst.Paths = []OutputPathMatch{
		{Line: 0, Start: 6, End: 28, Path: "/srv/harbor/willow.txt"},
		{Line: 2, Start: 4, End: 26, Path: "/srv/harbor/harbor.txt"},
		{Line: 3, Start: 0, End: 17, Path: "/srv/amber/lantern", IsDir: true},
		{Line: 3, Start: 22, Path: "/srv/amber/dune", End: 37, IsDir: true},
		{Line: 4, Start: 5, End: 15, Path: "/srv/ember", IsDir: true},
	}
	for cursor := range pst.Paths {
		c := pst
		c.PathCursor = cursor
		add(fmt.Sprintf("done_paths_cursor%d", cursor), 80, 24, c, ascii)
	}
	add("done_paths_cursor1_selected_nerd", 80, 24, func() CommandOutputDialogState { c := pst; c.PathCursor = 1; return c }(), nerd)
	many := commandOutputGoldenState(commandOutputGoldenLines(30), 12, false)
	for _, line := range []int{13, 15, 20} {
		many.Paths = append(many.Paths, OutputPathMatch{Line: line, Start: 5, End: 12, Path: fmt.Sprintf("/srv/amber/dune-%d", line)})
	}
	many.PathCursor = 1
	add("done_paths_scrolled", 80, 24, many, ascii)

	// titles
	t0 := commandOutputGoldenState(commandOutputGoldenLines(3), 0, false)
	t0.Title = ""
	add("done_title_empty", 80, 24, t0, ascii)
	t0.Title = "Build willow (exit 2)"
	add("done_title_exit", 80, 24, t0, ascii)
	t0.Title = strings.Repeat("Lantern ", 12)
	add("done_title_long", 80, 24, t0, ascii)

	// running: both focus positions, queued, with and without output, ascii and nerd icons
	run := commandOutputGoldenState(commandOutputGoldenLines(30), 0, true)
	run.Running = true
	for _, focus := range []string{"background"} {
		r := run
		setCommandOutputGoldenFocus(&r, focus)
		add("running_follow_focus-"+focus, 80, 24, r, ascii)
	}
	rn := run
	setCommandOutputGoldenFocus(&rn, "cancel")
	add("running_follow_focus-cancel_nerd", 80, 24, rn, nerd)
	rq := run
	rq.Queued = true
	add("running_queued", 80, 24, rq, ascii)
	add("running_queued_nerd", 80, 24, rq, nerd)
	re := commandOutputGoldenState(nil, 0, true)
	re.Running = true
	add("running_empty", 80, 24, re, ascii)
	add("running_empty_nerd", 80, 24, re, nerd)
	rs := commandOutputGoldenState(commandOutputGoldenLines(30), 5, false)
	rs.Running = true
	add("running_scrolled_up", 80, 24, rs, ascii)
	rt := rs
	rt.Title = ""
	add("running_no_title", 80, 24, rt, ascii)
	rl := rs
	rl.Title = strings.Repeat("Lantern ", 12)
	add("running_title_long", 80, 24, rl, ascii)
	rb := commandOutputGoldenState(commandOutputGoldenLines(3), 0, true)
	rb.Running = true
	add("running_short_output", 80, 24, rb, ascii)

	// configured sizes
	sz := commandOutputGoldenState(commandOutputGoldenLines(60), 10, false)
	for _, c := range []struct{ name, w, h string }{
		{"pref_50pct_30pct", "50%", "30%"}, {"pref_100pct_100pct", "100%", "100%"}, {"pref_60_20", "60", "20"},
		{"pref_20_5", "20", "5"}, {"pref_bad", "wide", "tall"}, {"pref_zero", "0", "0%"},
	} {
		s := sz
		s.PrefWidth, s.PrefHeight = c.w, c.h
		add(c.name, 100, 40, s, ascii)
	}
	add("big_120x50", 120, 50, commandOutputGoldenState(commandOutputGoldenLines(80), 20, false), ascii)

	// narrow and short screens
	nst := commandOutputGoldenState(commandOutputGoldenLines(40), 3, false)
	nst.Paths = []OutputPathMatch{{Line: 4, Start: 5, End: 12, Path: "/srv/amber/dune"}}
	add("narrow_60x20", 60, 20, nst, ascii)
	add("narrow_50x16", 50, 16, nst, ascii)
	add("narrow_44x12", 44, 12, nst, ascii)
	add("narrow_44x14", 44, 14, nst, ascii)
	add("narrow_40x24_too_narrow", 40, 24, nst, ascii) // below the 40 floor + 4 margin: nothing is drawn
	add("narrow_44x24", 44, 24, nst, ascii)
	add("short_80x12", 80, 12, nst, ascii)
	add("short_80x10", 80, 10, nst, ascii)
	add("short_80x9_too_short", 80, 9, nst, ascii)
	runningShort := nst
	runningShort.Running = true
	runningShort.Follow = true
	add("short_running_80x12", 80, 12, runningShort, ascii)
	add("narrow_running_44x14", 44, 14, runningShort, nerd)
	runSimpleGoldens(t, "command_output", cases)
}
