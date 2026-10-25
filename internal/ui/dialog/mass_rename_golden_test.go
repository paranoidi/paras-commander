package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the mass rename dialog: the main screen in all four modes (every focus
// position, preview shapes, errors and hints), the save-pattern prompt and the three pattern
// pickers. setMassRenameGoldenFocus is the only code that knows how the state encodes focus.

func init() {
	for _, name := range []string{"mass_rename", "mass_rename_save", "mass_rename_picker"} {
		goldenBandRows[name] = 50
	}
}

func addMassRenameGolden(cases *[]simpleGolden, w, h int, st FileDialogState, label string) {
	name := fmt.Sprintf("%dx%d_%s", w, h, label)
	*cases = append(*cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
		ctx := fileGoldenCtx()
		DrawFileDialog(s, l, st, ctx, nil)
	}})
}

// setMassRenameGoldenFocus names a focus position of the main screen: r0..r3 (mode radios), o0..o3
// (the option checkbox of column 0..3), find, repl, cw / cp (capitalize each word / punctuation),
// ok, apply, cancel; of the save prompt f0, f1, ok, cancel.
func setMassRenameGoldenFocus(st *FileDialogState, focus string) {
	radios := []dlg.ID{MassRenameRadioSimple, MassRenameRadioRegex, MassRenameRadioEditor, MassRenameRadioCapitalize}
	cols := []dlg.ID{MassRenameOptShowModified, MassRenameOptTrim, MassRenameOptCase, MassRenameOptIgnoreExt}
	if st.MassRenameMode == MassRenameModeUICapitalize {
		cols = []dlg.ID{MassRenameOptShowModified, MassRenameOptTrim, MassRenameOptIgnoreExt, ""}
	}
	switch {
	case focus == "find":
		st.Focus = FileFieldID(0)
	case focus == "repl":
		st.Focus = FileFieldID(1)
	case focus == "cw":
		st.Focus = MassRenameOptCapEachWord
	case focus == "cp":
		st.Focus = MassRenameOptCapPunct
	case focus == "ok":
		st.Focus = dlg.OK
	case focus == "apply":
		st.Focus = MassRenameApply
	case focus == "cancel":
		st.Focus = dlg.Cancel
	case strings.HasPrefix(focus, "r"):
		st.Focus = radios[goldenIndex(focus)]
	case strings.HasPrefix(focus, "o"):
		st.Focus = cols[goldenIndex(focus)]
	case strings.HasPrefix(focus, "f"):
		st.Focus = FileFieldID(goldenIndex(focus))
	}
}

// massRenameGoldenWords are the file names the previews are built from.
var massRenameGoldenWords = []string{"lantern", "orchard", "willow", "meadow", "timber", "ribbon", "velvet", "anchor", "marble", "otter"}

// massRenameGoldenState builds the main screen. mode: simple, regex, external, cap. data: none
// (no sources), few (4 rows), many (40 rows), long (names wider than the dialog), same (rows that
// change nothing), err (compute error). opts: m show only modified, t trim off, i case off, x
// ignore extension off, w capitalize each word, p capitalize punctuation, h regex compile hint,
// v find matches nothing (invalid), r a row conflict, e external editor names loaded.
func massRenameGoldenState(mode, data, opts, focus string, scroll int) FileDialogState {
	st := FileDialogState{
		Open:                       true,
		DialogType:                 FileDialogMassRename,
		Fields:                     []FileDialogField{{Label: "Find"}, {Label: "Replace"}},
		MassRenameCaseFold:         !strings.Contains(opts, "i"),
		MassRenameStripSpaces:      !strings.Contains(opts, "t"),
		MassRenameIgnoreExt:        !strings.Contains(opts, "x"),
		MassRenameShowOnlyModified: strings.Contains(opts, "m"),
		MassRenameCapEachWord:      strings.Contains(opts, "w"),
		MassRenameCapPunctSep:      strings.Contains(opts, "p"),
		MassRenameMarkerWidth:      MassRenameMarkerWidth(asciiDialogStyles()),
		MassRenamePreviewScroll:    scroll,
	}
	switch mode {
	case "regex":
		st.MassRenameMode = MassRenameModeUIRegex
		st.Fields[0].Label, st.Fields[1].Label = "Pattern", "Replacement"
	case "external":
		st.MassRenameMode = MassRenameModeUIExternalEditor
		if strings.Contains(opts, "e") {
			st.MassRenameExternalNames = []string{"harbor"}
		}
	case "cap":
		st.MassRenameMode = MassRenameModeUICapitalize
	}
	if mode == "simple" || mode == "regex" {
		st.Fields[0].Value, st.Fields[0].Cursor = "lantern", 7
		st.Fields[1].Value, st.Fields[1].Cursor = "harbor", 6
	}
	n := map[string]int{"none": 0, "few": 4, "many": 40, "long": 3, "same": 3, "err": 0}[data]
	for i := 0; i < n; i++ {
		w := massRenameGoldenWords[i%len(massRenameGoldenWords)]
		base := fmt.Sprintf("%s-%d", w, i)
		if data == "long" {
			base = strings.Repeat("harbor-orchard-meadow-", 3) + w
		}
		before, after := base+".txt", strings.Replace(base, "lantern", "harbor", 1)+".txt"
		if data == "same" {
			after = before
		}
		st.MassRenamePreviewBefore = append(st.MassRenamePreviewBefore, before)
		st.MassRenamePreviewAfter = append(st.MassRenamePreviewAfter, after)
		var removed, replaced, added []search.Range
		if strings.HasPrefix(base, "lantern") {
			replaced = []search.Range{{Start: 0, End: 7}}
			added = []search.Range{{Start: 0, End: 6}}
		}
		st.MassRenamePreviewBeforeRemoved = append(st.MassRenamePreviewBeforeRemoved, removed)
		st.MassRenamePreviewBeforeReplaced = append(st.MassRenamePreviewBeforeReplaced, replaced)
		st.MassRenamePreviewBeforeIgnored = append(st.MassRenamePreviewBeforeIgnored, search.Range{Start: len(base), End: len(base) + 4})
		st.MassRenamePreviewAfterAdded = append(st.MassRenamePreviewAfterAdded, added)
		st.MassRenamePreviewAfterError = append(st.MassRenamePreviewAfterError, strings.Contains(opts, "r") && i == 1)
	}
	if data == "err" {
		st.MassRenameComputeError = "no files to rename"
	}
	if strings.Contains(opts, "h") {
		st.MassRenamePatternCompileHint = "missing closing )"
		st.Fields[0].InputInvalid = true
	}
	if strings.Contains(opts, "v") {
		st.Fields[0].InputInvalid = true
	}
	if mode == "simple" || mode == "regex" {
		st.MassRenameMatchCount = 3
	}
	setMassRenameGoldenFocus(&st, focus)
	return st
}

func TestMassRenameGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, mode, data, opts, focus string, scroll int) {
		addMassRenameGolden(&cases, w, h, massRenameGoldenState(mode, data, opts, focus, scroll),
			fmt.Sprintf("%s_%s_opts-%s_scroll%d_focus-%s", mode, data, opts, scroll, focus))
	}
	// Every focus position of every mode on the usual preview.
	for _, focus := range []string{"r0", "find", "ok"} {
		add(80, 40, "simple", "few", "-", focus, 0)
	}
	for _, focus := range []string{"r1", "find", "ok"} {
		add(80, 40, "regex", "few", "-", focus, 0)
	}
	for _, focus := range []string{"r2", "ok"} {
		add(80, 40, "external", "few", "-", focus, 0)
	}
	for _, focus := range []string{"r3", "ok"} {
		add(80, 40, "cap", "few", "-", focus, 0)
	}
	// Option states.
	add(80, 40, "simple", "few", "mtix", "o0", 0)
	add(80, 40, "simple", "few", "m", "o1", 0)
	add(80, 40, "regex", "few", "ti", "o2", 0)
	add(80, 40, "cap", "few", "w", "cw", 0)
	add(80, 40, "cap", "few", "wp", "cp", 0)
	add(80, 40, "cap", "few", "mx", "o2", 0)
	add(80, 40, "external", "few", "m", "o0", 0)
	// Preview shapes.
	for _, mode := range []string{"simple", "regex", "external", "cap"} {
		add(80, 40, mode, "none", "-", "find", 0)
		add(80, 40, mode, "many", "-", "o0", 0)
		add(80, 40, mode, "err", "-", "o0", 0)
		add(80, 40, mode, "long", "-", "o0", 0)
	}
	add(80, 40, "simple", "many", "-", "find", 6)
	add(80, 40, "simple", "many", "-", "find", 99) // stale scroll past the end
	add(80, 40, "external", "many", "-", "r2", 20)
	add(80, 40, "simple", "same", "m", "find", 0)
	add(80, 40, "simple", "few", "r", "ok", 0) // a row conflict disables OK and Apply
	add(80, 40, "simple", "few", "r", "apply", 0)
	add(80, 40, "external", "few", "e", "ok", 0) // external names loaded
	add(80, 40, "external", "few", "er", "apply", 0)
	// Invalid input and hints.
	add(80, 40, "simple", "few", "v", "find", 0)
	add(80, 40, "simple", "few", "v", "ok", 0)
	add(80, 40, "regex", "few", "h", "find", 0)
	add(80, 40, "regex", "few", "h", "repl", 0)
	add(80, 40, "regex", "few", "v", "find", 0)
	add(60, 40, "regex", "few", "h", "find", 0)
	add(36, 40, "regex", "few", "h", "find", 0)
	// Sizes.
	add(60, 40, "simple", "few", "-", "find", 0)
	add(60, 40, "cap", "few", "-", "r3", 0)
	add(36, 40, "simple", "few", "-", "find", 0)
	add(36, 40, "external", "long", "-", "r2", 0)
	add(80, 30, "simple", "many", "-", "find", 0)
	add(80, 24, "simple", "many", "-", "find", 0)
	add(80, 24, "regex", "few", "-", "repl", 0)
	add(80, 24, "external", "many", "-", "o0", 0)
	add(80, 24, "cap", "many", "-", "cw", 0)
	add(80, 18, "simple", "many", "-", "find", 0)
	add(80, 14, "simple", "few", "-", "ok", 0)
	add(80, 12, "simple", "few", "-", "r0", 0)
	add(80, 10, "regex", "few", "-", "cancel", 0)
	add(80, 8, "external", "few", "-", "ok", 0)
	runSimpleGoldens(t, "mass_rename", cases)
}

// massRenameSaveGoldenState builds the save-pattern prompt.
func massRenameSaveGoldenState(v0, v1, focus string) FileDialogState {
	st := FileDialogState{
		Open:                  true,
		DialogType:            FileDialogMassRename,
		MassRenamePhase:       MassRenamePhaseSavePrompt,
		Fields:                []FileDialogField{{Label: "Name"}, {Label: "Description"}},
		MassRenameSavedFields: []FileDialogField{{Label: "Find", Value: "lantern"}, {Label: "Replace", Value: "harbor"}},
		MassRenameMarkerWidth: MassRenameMarkerWidth(asciiDialogStyles()),
	}
	for i, v := range []string{v0, v1} {
		formGoldenValue(&st.Fields[i], v)
	}
	setMassRenameGoldenFocus(&st, focus)
	return st
}

func TestMassRenameSaveGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, v0, v1, focus string) {
		addMassRenameGolden(&cases, w, h, massRenameSaveGoldenState(v0, v1, focus), fmt.Sprintf("%s_%s_focus-%s", v0, v1, focus))
	}
	for _, focus := range []string{"f0", "ok"} {
		add(80, 24, "typed", "typed", focus)
	}
	for _, v := range []string{"empty", "long", "longmid"} {
		add(80, 24, v, "typed", "f0")
		add(80, 24, "typed", v, "f1")
	}
	add(60, 24, "typed", "typed", "f0")
	add(36, 24, "typed", "typed", "ok")
	add(80, 12, "typed", "typed", "f1")
	add(80, 9, "typed", "typed", "ok")
	add(80, 7, "typed", "typed", "cancel")
	runSimpleGoldens(t, "mass_rename_save", cases)
}

// massRenamePickerGoldenState builds a pattern picker: phase load, history or overwrite; items
// patterns; query the typed filter.
func massRenamePickerGoldenState(phase string, items int, query string, selected, scroll int, focus string) FileDialogState {
	pk := MassRenamePatternPickerState{Query: query, QueryCursor: len(query), Selected: selected, ListScroll: scroll}
	for i := 0; i < items; i++ {
		w := massRenameGoldenWords[i%len(massRenameGoldenWords)]
		p := ops.MassRenamePattern{Mode: "simple", Find: w, Replace: fmt.Sprintf("%s%d", w, i)}
		if phase != "history" {
			p.Name, p.Description = fmt.Sprintf("%s-%d", w, i), "rename the "+w+" files"
		}
		pk.Items = append(pk.Items, p)
	}
	for i, it := range pk.Items {
		line := MassRenamePatternSearchLine(it)
		if query == "" {
			pk.Ranked = append(pk.Ranked, i)
			pk.MatchRanges = append(pk.MatchRanges, nil)
		} else if at := strings.Index(line, query); at >= 0 {
			pk.Ranked = append(pk.Ranked, i)
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
	st := FileDialogState{
		Open:                  true,
		DialogType:            FileDialogMassRename,
		Fields:                []FileDialogField{{Label: "Find"}, {Label: "Replace"}},
		MassRenameMarkerWidth: MassRenameMarkerWidth(asciiDialogStyles()),
	}
	switch phase {
	case "history":
		st.MassRenamePhase = MassRenamePhaseHistoryPicker
		st.MassRenameHistoryPicker = pk
	case "overwrite":
		st.MassRenamePhase = MassRenamePhaseOverwritePicker
		st.MassRenameLoadPicker = pk
	default:
		st.MassRenamePhase = MassRenamePhaseLoadPicker
		st.MassRenameLoadPicker = pk
	}
	return st
}

func TestMassRenamePickerGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, phase string, items int, query string, selected, scroll int, focus string) {
		addMassRenameGolden(&cases, w, h, massRenamePickerGoldenState(phase, items, query, selected, scroll, focus),
			fmt.Sprintf("%s_n%d_q-%s_sel%d_scroll%d_focus-%s", phase, items, query, selected, scroll, focus))
	}
	for _, phase := range []string{"load", "history", "overwrite"} {
		for _, focus := range []string{"list", "ok"} {
			add(80, 24, phase, 5, "", 0, 0, focus)
		}
	}
	add(80, 24, "load", 5, "", 3, 0, "list")
	add(80, 24, "load", 0, "", 0, 0, "list") // nothing saved
	add(80, 24, "history", 0, "", 0, 0, "ok")
	add(80, 24, "load", 5, "will", 0, 0, "list")
	add(80, 24, "load", 5, "zzz", 0, 0, "list")
	add(80, 24, "load", 40, "", 0, 0, "list")
	add(80, 24, "load", 40, "", 30, 19, "list")
	add(80, 24, "history", 40, "", 39, 28, "cancel")
	add(80, 40, "load", 40, "", 30, 21, "list")
	add(60, 24, "load", 5, "or", 0, 0, "list")
	add(36, 24, "history", 5, "", 2, 0, "list")
	add(80, 14, "load", 40, "", 3, 0, "list")
	add(80, 12, "overwrite", 5, "", 0, 0, "ok")
	add(80, 10, "load", 5, "", 0, 0, "list")
	add(80, 8, "history", 5, "", 0, 0, "cancel")
	runSimpleGoldens(t, "mass_rename_picker", cases)
}
