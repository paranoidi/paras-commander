package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the run-for-each dialog: the command form (every focus position, the
// validation error and preview rows, the two checkboxes, the worker pool radios) and its command
// history picker. The cases are named <w>x<h>_<value>_opts-<letters>_focus-<name>;
// setRunForEachGoldenFocus is the only code that knows how the state encodes focus.

func init() {
	for _, name := range []string{"run_for_each", "run_for_each_history"} {
		goldenBandRows[name] = 40
	}
}

// runForEachHelp is the help text the dialog opens with.
const runForEachGoldenHelp = "Runs once per selected item. Default mode must include %f (iterated item path).\n" +
	"Other macros: %d active dir, %F/%D other panel, %t/%T tagged paths.\n" +
	"Do not wrap % macros in quotes.\n" +
	">> | && etc. run via sh -c; otherwise argv is parsed without a shell.\n\n" +
	"Check \"Run in each selected directory\" (Alt+R) to cd into each selected directory " +
	"instead.\n" +
	"If enabled; %f becomes optional."

func addRunForEachGolden(cases *[]simpleGolden, w, h int, st FileDialogState, label string) {
	name := fmt.Sprintf("%dx%d_%s", w, h, label)
	*cases = append(*cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
		DrawFileDialog(s, l, st, fileGoldenCtx(), nil)
	}})
}

// setRunForEachGoldenFocus names a focus position: f0 (the command), cb0 (run in each selected
// directory), cb1 (pseudo-TTY), pool0.. (No pool, then the configured pools), ok, cancel. It is
// the only code that knows how the state encodes focus.
func setRunForEachGoldenFocus(st *FileDialogState, focus string) {
	switch focus {
	case "f0":
		st.Focus = FileFieldID(0)
	case "cb0":
		st.Focus = RunForEachInDirs
	case "cb1":
		st.Focus = RunForEachPTY
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		if n := goldenIndex(focus); n == 0 {
			st.Focus = RunForEachNoPool
		} else {
			st.Focus = RunForEachPoolID(n - 1)
		}
	}
}

// runForEachGoldenState builds the run-for-each form. value: empty, typed, long, longmid. opts:
// e a validation error, p a command preview, d run-in-directories on, t pseudo-TTY on, n two
// worker pools, s the second pool selected, h no help text, i invalid input, w a help line wider
// than the screen.
func runForEachGoldenState(value, opts, focus string) FileDialogState {
	st := FileDialogState{
		Open:             true,
		DialogType:       FileDialogRunForEach,
		Fields:           []FileDialogField{{Label: "Command"}},
		Message:          runForEachGoldenHelp,
		RunForEachDir:    "/srv/harbor",
		RunForEachInDirs: strings.Contains(opts, "d"),
		RunForEachPTY:    strings.Contains(opts, "t"),
	}
	f := &st.Fields[0]
	switch value {
	case "typed":
		f.Value, f.Cursor = "gzip -9 %f", 10
	case "long":
		v := strings.Repeat("harbor-orchard-meadow ", 5) + "%f"
		f.Value, f.Cursor = v, len(v)
	case "longmid":
		v := strings.Repeat("harbor-orchard-meadow ", 5) + "%f"
		f.Value, f.Cursor, f.Scroll = v, 40, 30
	}
	if strings.Contains(opts, "e") {
		st.RunForEachCommandError = "Selection must contain only directories"
	}
	if strings.Contains(opts, "p") {
		st.RunForEachPreview = "gzip -9 /srv/harbor/lantern.txt"
	}
	if strings.Contains(opts, "n") {
		st.RunForEachPools = []string{"batch", "render"}
	}
	if strings.Contains(opts, "s") {
		st.RunForEachPool = "render"
	}
	if strings.Contains(opts, "h") {
		st.Message = ""
	}
	if strings.Contains(opts, "w") {
		st.Message += "\n" + strings.Repeat("willow-", 20)
	}
	f.InputInvalid = strings.Contains(opts, "i") || strings.Contains(opts, "e")
	setRunForEachGoldenFocus(&st, focus)
	return st
}

func TestRunForEachGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		addRunForEachGolden(&cases, w, h, runForEachGoldenState(value, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", value, opts, focus))
	}
	for _, focus := range []string{"f0", "cb0", "cb1", "ok", "cancel"} {
		add(80, 40, "typed", "-", focus)
	}
	for _, focus := range []string{"f0", "cb0", "cb1", "pool0", "pool1", "pool2", "ok", "cancel"} {
		add(80, 40, "typed", "n", focus)
	}
	add(80, 40, "typed", "ns", "pool2")
	add(80, 40, "typed", "nds", "cb0")
	add(80, 40, "typed", "ndt", "cb1")
	for _, value := range []string{"empty", "long", "longmid"} {
		add(80, 40, value, "-", "f0")
		add(80, 40, value, "p", "f0")
	}
	add(80, 40, "typed", "p", "f0") // preview row
	add(80, 40, "typed", "p", "ok")
	add(80, 40, "typed", "e", "f0") // validation error row
	add(80, 40, "typed", "e", "ok")
	add(80, 40, "empty", "ep", "f0") // an error wins over the preview
	add(80, 40, "typed", "i", "f0")
	add(80, 40, "typed", "h", "f0") // no help text
	add(80, 40, "typed", "hn", "pool1")
	add(80, 40, "typed", "dt", "f0")
	add(80, 40, "typed", "w", "f0") // a help line wider than the screen
	add(60, 40, "typed", "p", "f0")
	add(60, 40, "typed", "n", "pool1")
	add(36, 40, "typed", "p", "f0")
	add(36, 40, "typed", "nds", "ok")
	add(80, 24, "typed", "-", "f0") // the usual terminal
	add(80, 24, "typed", "n", "ok")
	add(80, 24, "typed", "pn", "pool2")
	add(80, 20, "typed", "-", "ok") // clamped heights
	add(80, 16, "typed", "n", "f0")
	add(80, 12, "typed", "p", "cb1")
	add(80, 9, "typed", "-", "ok")
	add(80, 7, "typed", "-", "cancel")
	runSimpleGoldens(t, "run_for_each", cases)
}

// runForEachHistoryGoldenState builds the command-history picker. items: number of history
// lines; query: the typed filter; focus: list (the query row), ok, cancel.
func runForEachHistoryGoldenState(items int, query string, selected, scroll int, focus string) FileDialogState {
	words := []string{"gzip -9 %f", "tar -czf %f.tgz %f", "ffmpeg -i %f out.mkv", "chmod 600 %f", "echo orchard %f", "rsync -a %f meadow/", "convert %f lantern.png", "touch %f", "sha256sum %f"}
	pk := RunForEachHistoryPickerState{Query: query, QueryCursor: len(query), Selected: selected, ListScroll: scroll}
	for i := 0; i < items; i++ {
		pk.Items = append(pk.Items, fmt.Sprintf("%s %d", words[i%len(words)], i))
	}
	for i, it := range pk.Items {
		if query == "" {
			pk.Ranked = append(pk.Ranked, i)
			pk.MatchRanges = append(pk.MatchRanges, nil)
			continue
		}
		if strings.Contains(it, query) {
			pk.Ranked = append(pk.Ranked, i)
			at := strings.Index(it, query)
			pk.MatchRanges = append(pk.MatchRanges, []search.Range{{Start: at, End: at + len(query)}})
		} else {
			pk.MatchRanges = append(pk.MatchRanges, nil)
		}
	}
	switch focus {
	case "ok":
		pk.Focus = dlg.OK
	case "cancel":
		pk.Focus = dlg.Cancel
	}
	return FileDialogState{
		Open:                    true,
		DialogType:              FileDialogRunForEach,
		Fields:                  []FileDialogField{{Label: "Command", Value: "gzip %f"}},
		RunForEachHistoryOpen:   true,
		RunForEachHistoryPicker: pk,
	}
}

func TestRunForEachHistoryGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h, items int, query string, selected, scroll int, focus string) {
		addRunForEachGolden(&cases, w, h, runForEachHistoryGoldenState(items, query, selected, scroll, focus),
			fmt.Sprintf("n%d_q-%s_sel%d_scroll%d_focus-%s", items, query, selected, scroll, focus))
	}
	for _, focus := range []string{"list", "ok", "cancel"} {
		add(80, 24, 5, "", 0, 0, focus)
	}
	add(80, 24, 5, "", 3, 0, "list")
	add(80, 24, 5, "", 3, 0, "ok")   // the cursor row is not lit away from the list
	add(80, 24, 0, "", 0, 0, "list") // empty history
	add(80, 24, 0, "", 0, 0, "ok")
	add(80, 24, 5, "gzip", 0, 0, "list")
	add(80, 24, 5, "zzz", 0, 0, "list") // no match
	add(80, 24, 40, "", 0, 0, "list")
	add(80, 24, 40, "", 25, 12, "list")
	add(80, 24, 40, "", 39, 30, "list")
	add(80, 24, 40, "", 39, 30, "cancel")
	add(80, 40, 40, "", 0, 0, "list") // tall screen: the list is capped
	add(80, 40, 40, "", 30, 20, "list")
	add(60, 24, 5, "tar", 0, 0, "list")
	add(36, 24, 5, "", 2, 0, "list")
	add(80, 14, 40, "", 3, 0, "list") // clamped heights
	add(80, 12, 5, "", 0, 0, "ok")
	add(80, 10, 5, "", 0, 0, "list")
	add(80, 8, 5, "", 0, 0, "cancel")
	runSimpleGoldens(t, "run_for_each_history", cases)
}
