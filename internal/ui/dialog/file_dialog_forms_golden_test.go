package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the single-input file dialogs that are not part of the rename family: add
// bookmark, the SFTP password prompt and archive extract. The cases are named
// <w>x<h>_<value>_opts-<letters>_focus-<name>; setFormGoldenFocus is the only code that knows how
// the state encodes focus.

func init() {
	for _, name := range []string{"bookmark_add", "sftp_password", "extract"} {
		goldenBandRows[name] = 40
	}
}

func addFormGolden(cases *[]simpleGolden, w, h int, st FileDialogState, label string) {
	name := fmt.Sprintf("%dx%d_%s", w, h, label)
	*cases = append(*cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
		DrawFileDialog(s, l, st, fileGoldenCtx(), nil)
	}})
}

// setFormGoldenFocus names a focus position: f0 (the input), ok, cancel.
func setFormGoldenFocus(st *FileDialogState, focus string) {
	st.Focus = map[string]dlg.ID{"f0": FileFieldID(0), "ok": dlg.OK, "cancel": dlg.Cancel}[focus]
}

// formGoldenValue applies a value variant to a form field: empty, typed, prefill (unedited
// suggestion), long (wider than the field, caret at the end), longmid (caret mid-text, scrolled).
func formGoldenValue(f *FileDialogField, value string) {
	set := func(v string, cursor int) { f.Value, f.Cursor = v, cursor }
	switch value {
	case "typed":
		set("lantern", 7)
	case "prefill":
		set("orchard", 7)
		f.Prefill, f.PrefillPending = "orchard", true
	case "long":
		v := strings.Repeat("harbor-orchard-meadow-", 5) + "lantern"
		set(v, len(v))
	case "longmid":
		v := strings.Repeat("harbor-orchard-meadow-", 5) + "lantern"
		set(v, 40)
		f.Scroll = 30
	}
}

func bookmarkGoldenState(value, opts, focus string) FileDialogState {
	path := "/srv/harbor/orchard"
	if strings.Contains(opts, "p") {
		path = "/srv/" + strings.Repeat("harbor-orchard-meadow/", 5) + "lantern"
	}
	st := FileDialogState{
		Open:       true,
		DialogType: FileDialogAddBookmark,
		Fields:     []FileDialogField{{Label: "Name"}},
		Message:    path,
	}
	formGoldenValue(&st.Fields[0], value)
	st.Fields[0].InputInvalid = strings.Contains(opts, "i")
	setFormGoldenFocus(&st, focus)
	return st
}

func TestFileDialogBookmarkGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		addFormGolden(&cases, w, h, bookmarkGoldenState(value, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", value, opts, focus))
	}
	for _, focus := range []string{"f0", "ok"} {
		add(80, 24, "typed", "-", focus)
	}
	for _, value := range []string{"empty", "prefill", "long", "longmid"} {
		add(80, 24, value, "-", "f0")
	}
	add(80, 24, "prefill", "-", "ok")
	add(80, 24, "typed", "i", "f0") // invalid input
	add(80, 24, "typed", "i", "ok")
	add(80, 24, "typed", "p", "f0") // a long path is cut
	add(60, 24, "typed", "-", "f0")
	add(60, 24, "long", "p", "f0")
	add(36, 24, "typed", "-", "f0")
	add(36, 24, "typed", "p", "ok")
	add(80, 12, "typed", "-", "f0") // clamped heights
	add(80, 11, "typed", "-", "ok")
	add(80, 9, "typed", "-", "ok")
	add(80, 7, "typed", "-", "cancel")
	runSimpleGoldens(t, "bookmark_add", cases)
}

// sftpPasswordGoldenState builds the SSH password prompt. opts: l a long user and host label, i
// invalid input. The prompt has no suggested default, so the prefill variant is not used.
func sftpPasswordGoldenState(value, opts, focus string) FileDialogState {
	label := "Password for alice@example.com"
	if strings.Contains(opts, "l") {
		label = "Password for " + strings.Repeat("willow-", 6) + "alice@" + strings.Repeat("harbor.", 6) + "example.com"
	}
	st := FileDialogState{
		Open:       true,
		DialogType: FileDialogSFTPPassword,
		Fields:     []FileDialogField{{Label: label}},
	}
	formGoldenValue(&st.Fields[0], value)
	st.Fields[0].InputInvalid = strings.Contains(opts, "i")
	setFormGoldenFocus(&st, focus)
	return st
}

func TestFileDialogSFTPPasswordGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		addFormGolden(&cases, w, h, sftpPasswordGoldenState(value, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", value, opts, focus))
	}
	for _, focus := range []string{"f0", "ok"} {
		add(80, 24, "typed", "-", focus)
	}
	for _, value := range []string{"empty", "long", "longmid"} {
		add(80, 24, value, "-", "f0")
	}
	add(80, 24, "typed", "i", "f0") // invalid input
	add(80, 24, "typed", "i", "ok")
	add(80, 24, "typed", "l", "f0") // a long label widens the dialog
	add(60, 24, "typed", "l", "f0")
	add(60, 24, "typed", "-", "f0")
	add(36, 24, "typed", "-", "f0")
	add(36, 24, "typed", "l", "ok")
	add(80, 12, "typed", "-", "f0") // clamped heights
	add(80, 8, "typed", "-", "ok")
	add(80, 7, "typed", "-", "ok")
	add(80, 6, "typed", "-", "cancel")
	runSimpleGoldens(t, "sftp_password", cases)
}

// extractGoldenState builds the archive extract dialog. value is a fileGoldenValue variant of the
// destination (typed means a path); opts: m the skipped-items note, n a two-line note, l a note
// longer than the dialog, i invalid input.
func extractGoldenState(value, opts, focus string) FileDialogState {
	st := FileDialogState{
		Open:           true,
		DialogType:     FileDialogExtract,
		Fields:         []FileDialogField{{Label: "Destination", PathPicker: true}},
		ExtractSources: []string{"/srv/harbor/orchard.zip"},
	}
	if value == "typed" {
		value = "path"
	}
	fileGoldenValue(&st.Fields[0], value)
	if value == "prefill" {
		st.Fields[0].Value, st.Fields[0].Prefill, st.Fields[0].Cursor = "/srv/harbor/meadow/", "/srv/harbor/meadow/", 19
	}
	switch {
	case strings.Contains(opts, "m"):
		st.Message = "2 non-archive item(s) will be skipped."
	case strings.Contains(opts, "n"):
		st.Message = "2 non-archive item(s) will be skipped.\nlantern.txt is not an archive."
	case strings.Contains(opts, "l"):
		st.Message = "3 non-archive item(s) will be skipped: " + strings.Repeat("willow-orchard-", 8)
	}
	st.Fields[0].InputInvalid = strings.Contains(opts, "i")
	setFormGoldenFocus(&st, focus)
	return st
}

func TestFileDialogExtractGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, value, opts, focus string) {
		addFormGolden(&cases, w, h, extractGoldenState(value, opts, focus), fmt.Sprintf("%s_opts-%s_focus-%s", value, opts, focus))
	}
	for _, focus := range []string{"f0", "ok"} {
		add(80, 24, "typed", "-", focus)
		add(80, 24, "typed", "m", focus)
	}
	for _, value := range []string{"empty", "prefill", "long", "longmid", "ghost", "dropdown", "scrollbar"} {
		add(80, 24, value, "-", "f0")
		add(80, 24, value, "m", "f0")
	}
	add(80, 24, "dropdown", "-", "ok") // a stale open dropdown while a button has focus
	add(80, 24, "typed", "n", "f0")
	add(80, 24, "typed", "n", "ok")
	add(80, 24, "typed", "l", "f0")
	add(80, 24, "typed", "i", "f0") // invalid input
	add(80, 24, "typed", "i", "ok")
	add(60, 24, "typed", "m", "f0")
	add(60, 24, "long", "l", "f0")
	add(36, 24, "typed", "-", "f0")
	add(36, 24, "typed", "m", "ok")
	add(80, 12, "typed", "m", "f0") // clamped heights
	add(80, 12, "dropdown", "-", "f0")
	add(80, 9, "typed", "n", "ok")
	add(80, 8, "typed", "m", "ok")
	add(80, 7, "typed", "-", "cancel")
	runSimpleGoldens(t, "extract", cases)
}
