package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the file dialog family served by DrawFileDialog: rename (and its Sanitize,
// Slugify and Encoding tool screens), duplicate, mkdir, delete, chmod, chown, symlink and hardlink.
// Every member has its own dialog name under testdata/simple_dialogs/. The cases are named
// <w>x<h>_<value>_opts-<letters>_focus-<name>; setFileGoldenFocus is the only code that knows how
// the state encodes focus.

func init() {
	for _, name := range []string{"file_rename", "file_rename_tool", "file_duplicate", "file_mkdir", "file_delete", "file_chmod", "file_chown", "file_symlink", "file_hardlink"} {
		goldenBandRows[name] = 40
	}
}

// fileGoldenDeleteIcon stands in for the devicon painter: one marker glyph per row.
func fileGoldenDeleteIcon(screen tcell.Screen, x, y int, e DeleteListEntry, styles theme.Theme) {
	glyph := '#'
	if e.Type == localfs.EntryDirectory {
		glyph = '+'
	}
	screen.SetContent(x, y, glyph, nil, styles.DialogText)
}

// setFileGoldenFocus names a focus position: f0/f1 (text inputs), opt0.. (the rows after the inputs
// of the main screen, the options of a rename tool screen), ok / cancel (also yes / no on the
// delete confirmation). It is the only code that knows how the state encodes focus.
func setFileGoldenFocus(st *FileDialogState, focus string) {
	switch focus {
	case "ok", "yes":
		st.Focus = dlg.OK
		return
	case "cancel", "no":
		st.Focus = dlg.Cancel
		return
	}
	n := goldenIndex(focus)
	switch {
	case strings.HasPrefix(focus, "f"):
		st.Focus = FileFieldID(n)
	case st.RenamePhase == RenamePhaseSanitize:
		st.Focus = []dlg.ID{FileSanitizeDots, FileSanitizeUnderscores}[n]
	case st.RenamePhase == RenamePhaseSlugify:
		st.Focus = []dlg.ID{FileSlugifyDot, FileSlugifyUnderscore}[n]
	case st.RenamePhase == RenamePhaseEncoding:
		st.Focus = renameEncodingID(n)
	case st.DialogType == FileDialogMkdir:
		st.Focus = MkdirActionID(MkdirAction(n))
	default:
		st.Focus = []dlg.ID{FileRenameFocusAfter, FileRenameOpenOther}[n]
	}
}

// fileGoldenValue applies a named value variant to a field: empty, typed, prefill (unedited
// suggestion), long (wider than the field, caret at the end), longmid (caret mid-text, scrolled),
// ghost (single completion candidate), dropdown (3 candidates), scrollbar (8 candidates).
func fileGoldenValue(f *FileDialogField, value string) {
	names := func(n ...string) []pathpick.Candidate {
		var items []pathpick.Candidate
		for _, name := range n {
			items = append(items, pathpick.Candidate{Name: name, IsDir: true})
		}
		return items
	}
	set := func(v string, cursor int) { f.Value, f.Cursor = v, cursor }
	switch value {
	case "typed":
		set("lantern.txt", 11)
	case "prefill":
		set("lantern.txt", 11)
		f.Prefill, f.PrefillPending = "lantern.txt", true
	case "long":
		v := strings.Repeat("harbor-orchard-meadow-", 5) + "lantern.txt"
		set(v, len(v))
	case "longmid":
		v := strings.Repeat("harbor-orchard-meadow-", 5) + "lantern.txt"
		set(v, 40)
		f.Scroll = 30
	case "path":
		set("/srv/harbor/orchard", 19)
	case "ghost":
		set("/srv/harbor/or", 14)
		f.Completion.Set("/srv/harbor/or", 12, names("orchard"))
	case "dropdown":
		set("/srv/harbor/", 12)
		f.Completion.Set("/srv/harbor/o", 12, names("orchard", "otter", "oak"))
		f.Completion.Open = true
	case "scrollbar":
		set("/srv/harbor/", 12)
		f.Completion.Set("/srv/harbor/m", 12, names("meadow", "marble", "mast", "moss", "mill", "maple", "mint", "mule"))
		f.Completion.Open = true
		f.Completion.Selected = 6
		f.Completion.Scroll = 2
	}
}

func fileGoldenCtx() DialogRenderContext {
	return DialogRenderContext{Styles: asciiDialogStyles(), UserHomeDir: "/home/alice", ShowIcons: true, IconLead: 2, ScrollbarStyle: uiscrollbar.StyleThumb}
}

func addFileGolden(cases *[]simpleGolden, member string, w, h int, st FileDialogState, label string) {
	name := fmt.Sprintf("%dx%d_%s", w, h, label)
	*cases = append(*cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
		DrawFileDialog(s, l, st, fileGoldenCtx(), fileGoldenDeleteIcon)
	}})
	_ = member
}

// renameGoldenState builds the rename or duplicate main screen. opts: f focus-after on, d source is a
// directory (adds the open-in-other-panel checkbox), o open-in-other on, i input invalid.
func renameGoldenState(dup bool, value, opts, focus string) FileDialogState {
	st := FileDialogState{
		Open:             true,
		DialogType:       FileDialogRename,
		Fields:           []FileDialogField{{Label: "Name"}},
		RenamePhase:      RenamePhaseMain,
		RenameSlugifySep: RenameSlugifyDot,
		RenameFocusAfter: strings.Contains(opts, "f"),
		RenameSource:     "/srv/harbor/lantern.txt",
	}
	if dup {
		st.DialogType = FileDialogDuplicate
		st.DuplicateSource, st.RenameSource = "/srv/harbor/lantern.txt", ""
	} else {
		st.RenameSourceIsDir = strings.Contains(opts, "d")
		st.RenameOpenInOther = strings.Contains(opts, "o")
	}
	f := &st.Fields[0]
	fileGoldenValue(f, value)
	switch value {
	case "trail":
		f.Value, f.Cursor = "lantern.txt  ", 13
	case "lead":
		f.Value, f.Cursor = "  lantern.txt", 13
	case "both":
		f.Value, f.Cursor = " lantern.txt ", 13
	}
	f.InputInvalid = strings.Contains(opts, "i")
	setFileGoldenFocus(&st, focus)
	return st
}

// renameToolGoldenState builds a rename tool screen: phase sanitize, slugify or encoding. opts for
// sanitize: d dots on, u underscores on; for slugify: u underscore separator; for encoding: the
// digit selects the candidate.
func renameToolGoldenState(phase, opts, focus string) FileDialogState {
	st := FileDialogState{
		Open:             true,
		DialogType:       FileDialogRename,
		Fields:           []FileDialogField{{Label: "Name", Value: "lantern orchard_willow.meadow txt", Cursor: 5}},
		RenameSlugifySep: RenameSlugifyDot,
		RenameSource:     "/srv/harbor/lantern.txt",
	}
	switch phase {
	case "sanitize":
		st.RenamePhase = RenamePhaseSanitize
		st.RenameSanitizeDots = strings.Contains(opts, "d")
		st.RenameSanitizeUnderscores = strings.Contains(opts, "u")
	case "slugify":
		st.RenamePhase = RenamePhaseSlugify
		if strings.Contains(opts, "u") {
			st.RenameSlugifySep = RenameSlugifyUnderscore
		}
	case "encoding":
		st.RenamePhase = RenamePhaseEncoding
		st.RenameEncodingCandidates = []RenameEncodingCandidate{
			{Label: "Shift-JIS", UTF8: "lantern orchard.txt"},
			{Label: "EUC-JP", UTF8: "willow meadow.txt"},
			{Label: "Windows-31J", UTF8: "harbor ribbon.txt"},
		}
		for i := range st.RenameEncodingCandidates {
			if strings.Contains(opts, fmt.Sprint(i)) {
				st.RenameEncodingSelected = i
			}
		}
	}
	setFileGoldenFocus(&st, focus)
	return st
}

// mkdirGoldenState builds the mkdir dialog. opts: a adds the post-action radios (a selection),
// 1 / 2 pick the copy / move action.
func mkdirGoldenState(value, opts, focus string) FileDialogState {
	st := FileDialogState{
		Open:             true,
		DialogType:       FileDialogMkdir,
		Fields:           []FileDialogField{{Label: "Directory name"}},
		MkdirShowActions: strings.Contains(opts, "a"),
	}
	fileGoldenValue(&st.Fields[0], value)
	switch {
	case strings.Contains(opts, "1"):
		st.MkdirAction = MkdirActionCreateCopySelect
	case strings.Contains(opts, "2"):
		st.MkdirAction = MkdirActionCreateMoveSelect
	}
	setFileGoldenFocus(&st, focus)
	return st
}

// deleteGoldenEntries builds n entries: nested labels (contextual, path-fitted) mixed with
// plain names, files and directories.
func deleteGoldenEntries(n int, long bool) []DeleteListEntry {
	words := []string{"lantern", "orchard", "willow", "meadow", "timber", "ribbon", "velvet", "anchor", "marble", "otter"}
	var out []DeleteListEntry
	for i := 0; i < n; i++ {
		w := words[i%len(words)]
		name, path := fmt.Sprintf("%s%d.txt", w, i), fmt.Sprintf("/srv/harbor/%s%d.txt", w, i)
		typ := localfs.EntryFile
		switch i % 4 {
		case 1:
			name, path, typ = fmt.Sprintf("%s%d", w, i), fmt.Sprintf("/srv/harbor/%s%d", w, i), localfs.EntryDirectory
		case 2:
			name, path = fmt.Sprintf("thicket/%s%d.txt", w, i), fmt.Sprintf("/srv/harbor/thicket/%s%d.txt", w, i)
		}
		if long && i == 0 {
			name = strings.Repeat("harbor-orchard-meadow/", 4) + "lantern.txt"
			path = "/srv/" + name
		}
		out = append(out, DeleteListEntry{Name: name, Path: path, Type: typ})
	}
	return out
}

// deleteGoldenState builds the delete confirmation. opts: w warning line, d dangling-directories
// prompt, l one long entry, s scrolled halfway (value of scroll in the entries: set by the case),
// e empty summary.
func deleteGoldenState(entries int, opts, focus string, scroll int) FileDialogState {
	st := FileDialogState{
		Open:             true,
		DialogType:       FileDialogDelete,
		DeleteSummary:    fmt.Sprintf("%d files, 2 directories (1.2 MiB)", entries),
		DeleteEntries:    deleteGoldenEntries(entries, strings.Contains(opts, "l")),
		DeleteListScroll: scroll,
	}
	if strings.Contains(opts, "w") {
		st.DeleteWarning = "Directories are removed with their contents."
	}
	if strings.Contains(opts, "d") {
		st.DeleteDanglingDirs = true
		st.DeleteSummary = fmt.Sprintf("%d directories left empty", entries)
	}
	if strings.Contains(opts, "e") {
		st.DeleteSummary = ""
	}
	st.DeleteLayoutMinWidth = ComputeDeleteDialogLayoutMinWidth(st, 2)
	setFileGoldenFocus(&st, focus)
	return st
}

// twoFieldGoldenState builds chmod (one field), chown (two) and the symlink / hardlink dialogs
// (two path fields). v0 / v1 are the value variants of the first and second field.
func twoFieldGoldenState(typ FileDialogType, v0, v1, opts, focus string) FileDialogState {
	st := FileDialogState{Open: true, DialogType: typ}
	pathFields := typ == FileDialogSymlink || typ == FileDialogHardlink
	labels := map[FileDialogType][]string{
		FileDialogChmod:    {"Mode"},
		FileDialogChown:    {"User", "Group"},
		FileDialogSymlink:  {"Target", "Link path"},
		FileDialogHardlink: {"Source", "New path"},
	}[typ]
	for i, l := range labels {
		st.Fields = append(st.Fields, FileDialogField{Label: l, PathPicker: pathFields})
		v := v0
		if i == 1 {
			v = v1
		}
		if pathFields && v == "typed" {
			v = "path"
		}
		fileGoldenValue(&st.Fields[i], v)
		if typ == FileDialogChmod && v == "typed" {
			st.Fields[i].Value, st.Fields[i].Cursor = "0644", 4
		}
		if typ == FileDialogChown && v == "typed" {
			st.Fields[i].Value, st.Fields[i].Cursor = []string{"willow", "meadow"}[i], 6
		}
		st.Fields[i].InputInvalid = strings.Contains(opts, "i") && i == 0
	}
	st.Message = ""
	if strings.Contains(opts, "m") {
		st.Message = "Invalid mode"
	}
	setFileGoldenFocus(&st, focus)
	return st
}

func TestFileDialogRenameGoldenRender(t *testing.T) {
	for _, dup := range []bool{false, true} {
		member := "file_rename"
		tag := "rename"
		if dup {
			member, tag = "file_duplicate", "dup"
		}
		var cases []simpleGolden
		add := func(w, h int, value, opts, focus string) {
			addFileGolden(&cases, member, w, h, renameGoldenState(dup, value, opts, focus), fmt.Sprintf("%s_%s_opts-%s_focus-%s", tag, value, opts, focus))
		}
		for _, focus := range []string{"f0", "ok"} {
			add(80, 24, "typed", "-", focus)
		}
		add(80, 24, "typed", "f", "opt0")
		add(80, 24, "typed", "f", "f0")
		for _, value := range []string{"empty", "prefill", "long", "longmid", "trail", "lead", "both"} {
			add(80, 24, value, "-", "f0")
			add(80, 24, value, "f", "opt0") // the whitespace warning shows once focus left the name
		}
		add(80, 24, "typed", "i", "f0") // invalid input
		add(80, 24, "typed", "i", "ok")
		add(60, 24, "typed", "-", "f0")
		add(60, 24, "long", "-", "f0") // wide mode, capped by the screen
		add(36, 24, "typed", "-", "ok")
		add(36, 24, "long", "f", "opt0")
		add(80, 12, "typed", "-", "f0")
		add(80, 9, "typed", "f", "ok")
		add(80, 8, "typed", "-", "cancel")
		if !dup {
			for _, focus := range []string{"f0", "ok"} {
				add(80, 24, "typed", "d", focus) // directory: open in the other panel
			}
			add(80, 24, "typed", "dfo", "opt1")
			add(80, 24, "trail", "dfo", "opt1")
			add(36, 24, "typed", "df", "opt1")
			add(80, 10, "typed", "d", "ok")
		}
		runSimpleGoldens(t, member, cases)
	}
}

func TestFileDialogRenameToolGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, phase, opts, focus string) {
		addFileGolden(&cases, "file_rename_tool", w, h, renameToolGoldenState(phase, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", phase, opts, focus))
	}
	for _, focus := range []string{"opt0", "ok"} {
		add(80, 24, "sanitize", "-", focus)
		add(80, 24, "slugify", "-", focus)
	}
	for _, opts := range []string{"d", "u", "du"} {
		add(80, 24, "sanitize", opts, "opt0")
	}
	add(80, 24, "slugify", "u", "opt1")
	for _, focus := range []string{"opt0", "ok"} {
		add(80, 24, "encoding", "0", focus)
	}
	add(80, 24, "encoding", "1", "opt1")
	add(80, 24, "encoding", "2", "ok")
	add(36, 24, "sanitize", "du", "ok")
	add(36, 24, "encoding", "1", "opt2")
	add(80, 9, "sanitize", "d", "ok")
	add(80, 8, "encoding", "0", "cancel")
	runSimpleGoldens(t, "file_rename_tool", cases)
}

func TestFileDialogMkdirGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		addFileGolden(&cases, "file_mkdir", w, h, mkdirGoldenState(value, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", value, opts, focus))
	}
	for _, focus := range []string{"f0", "ok"} {
		add(80, 24, "typed", "-", focus)
	}
	for _, value := range []string{"empty", "prefill", "long", "longmid"} {
		add(80, 24, value, "-", "f0")
	}
	for _, focus := range []string{"f0", "ok"} {
		add(80, 24, "typed", "a", focus)
	}
	add(80, 24, "typed", "a1", "opt1")
	add(80, 24, "typed", "a2", "opt2")
	add(80, 24, "typed", "a1", "f0")
	add(80, 24, "long", "a", "f0")
	add(60, 24, "typed", "a", "opt0")
	add(36, 24, "typed", "-", "f0")
	add(36, 24, "typed", "a2", "ok")
	add(80, 12, "typed", "a", "opt2")
	add(80, 9, "typed", "a", "ok")
	add(80, 6, "typed", "-", "f0")
	runSimpleGoldens(t, "file_mkdir", cases)
}

func TestFileDialogDeleteGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h, entries int, opts, focus string, scroll int) {
		addFileGolden(&cases, "file_delete", w, h, deleteGoldenState(entries, opts, focus, scroll), fmt.Sprintf("n%d_opts-%s_scroll%d_focus-%s", entries, opts, scroll, focus))
	}
	for _, focus := range []string{"yes"} {
		add(80, 24, 3, "-", focus, 0)
		add(80, 24, 1, "-", focus, 0)
		add(80, 24, 3, "w", focus, 0)
		add(80, 24, 3, "d", focus, 0)
		add(80, 24, 1, "d", focus, 0)
	}
	add(80, 24, 6, "l", "no", 0) // one long entry widens the dialog
	add(80, 24, 6, "e", "no", 0) // no summary text
	add(80, 24, 0, "-", "no", 0) // nothing to list
	add(80, 24, 40, "-", "no", 0)
	add(80, 24, 40, "-", "yes", 0)
	add(80, 24, 40, "-", "no", 5)
	add(80, 24, 40, "-", "no", 22)
	add(80, 24, 40, "-", "no", 99) // stale scroll past the end
	add(80, 24, 40, "w", "no", 3)
	add(80, 24, 40, "wl", "yes", 0)
	add(80, 40, 40, "-", "no", 0) // tall screen: the list is capped
	add(80, 40, 40, "-", "no", 10)
	add(80, 12, 40, "-", "no", 0) // clamped heights
	add(80, 12, 40, "w", "yes", 2)
	add(80, 9, 5, "-", "no", 0)
	add(80, 8, 5, "w", "no", 0)
	add(60, 24, 6, "l", "no", 0) // narrow
	add(36, 24, 6, "-", "yes", 0)
	add(36, 24, 40, "w", "no", 4)
	runSimpleGoldens(t, "file_delete", cases)
}

func TestFileDialogAttrGoldenRender(t *testing.T) {
	type member struct {
		name string
		typ  FileDialogType
	}
	for _, m := range []member{{"file_chmod", FileDialogChmod}, {"file_chown", FileDialogChown}, {"file_symlink", FileDialogSymlink}, {"file_hardlink", FileDialogHardlink}} {
		var cases []simpleGolden
		fields := len(twoFieldGoldenState(m.typ, "empty", "empty", "-", "ok").Fields)
		add := func(w, h int, v0, v1, opts, focus string) {
			addFileGolden(&cases, m.name, w, h, twoFieldGoldenState(m.typ, v0, v1, opts, focus), fmt.Sprintf("%s_%s_opts-%s_focus-%s", v0, v1, opts, focus))
		}
		foci := []string{"f0", "ok"}
		if fields > 1 {
			foci = []string{"f0", "f1", "ok"}
		}
		for _, focus := range foci {
			add(80, 24, "typed", "typed", "-", focus)
		}
		for _, v := range []string{"empty", "long", "longmid"} {
			add(80, 24, v, "typed", "-", "f0")
			if fields > 1 {
				add(80, 24, "typed", v, "-", "f1")
			}
		}
		add(80, 24, "typed", "typed", "i", "f0") // invalid first field
		add(80, 24, "typed", "typed", "i", "ok")
		add(80, 24, "typed", "typed", "m", "f0") // a message turns the focused label to the warning style
		if fields > 1 {
			add(80, 24, "typed", "typed", "m", "f1")
		}
		if m.typ == FileDialogSymlink || m.typ == FileDialogHardlink {
			// Completion: ghost text, the dropdown over the next rows, and a stale open dropdown.
			for _, v := range []string{"prefill", "ghost", "dropdown", "scrollbar"} {
				add(80, 24, v, "typed", "-", "f0")
				add(80, 24, "typed", v, "-", "f1")
			}
			add(80, 24, "dropdown", "typed", "-", "f1") // first dropdown still open while the second field has focus
			add(80, 24, "dropdown", "dropdown", "-", "f1")
			add(80, 24, "dropdown", "typed", "-", "ok")
			add(80, 12, "dropdown", "typed", "-", "f0") // dropdown near the screen bottom
			add(80, 12, "typed", "scrollbar", "-", "f1")
		}
		add(60, 24, "long", "typed", "-", "f0")
		add(36, 24, "typed", "typed", "-", "ok")
		add(80, 12, "typed", "typed", "-", "f0")
		add(80, 9, "typed", "typed", "-", "ok")
		add(80, 7, "typed", "typed", "-", "cancel")
		runSimpleGoldens(t, m.name, cases)
	}
}
