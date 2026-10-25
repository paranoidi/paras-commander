package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the file dialog family: rename (and its Sanitize / Slugify / Encoding
// screens), duplicate, mkdir, delete, chmod, chown, symlink and hardlink. They use the harness of
// simple_dialog_keys_golden_test.go. Link dialogs use relative paths, so nothing is ever created
// outside the temp dir.

// fileFocusName names the focused widget of a file dialog: f0/f1 (inputs), opt0.. (the rows after
// the inputs, or the options of a rename tool screen), ok / cancel (yes / no on the delete
// confirmation). It is the only code that knows how the state encodes focus.
func fileFocusName(st dialog.FileDialogState) string {
	delete := st.DialogType == dialog.FileDialogDelete
	switch f := st.Focus; {
	case f == dlg.OK && delete:
		return "yes"
	case f == dlg.Cancel && delete:
		return "no"
	case f == dlg.OK:
		return "ok"
	case f == dlg.Cancel:
		return "cancel"
	case f == "" && delete:
		return "no"
	case f == "" && dialog.FileDialogHasRenamePhase(st.DialogType) && st.RenamePhase != dialog.RenamePhaseMain:
		return "ok"
	}
	if i, ok := dialog.FileFieldIndex(st.Focus); ok {
		return fmt.Sprintf("f%d", i)
	}
	if st.Focus == "" {
		return "f0"
	}
	ids := map[dlg.ID]int{
		dialog.FileRenameFocusAfter: 0, dialog.FileRenameOpenOther: 1,
		dialog.FileSanitizeDots: 0, dialog.FileSanitizeUnderscores: 1,
		dialog.FileSlugifyDot: 0, dialog.FileSlugifyUnderscore: 1,
	}
	if n, ok := ids[st.Focus]; ok {
		return fmt.Sprintf("opt%d", n)
	}
	for i := range 3 {
		if st.Focus == dialog.MkdirActionID(dialog.MkdirAction(i)) {
			return fmt.Sprintf("opt%d", i)
		}
	}
	for i := range st.RenameEncodingCandidates {
		if id := dlg.ID(fmt.Sprintf("encoding-%d", i)); st.Focus == id {
			return fmt.Sprintf("opt%d", i)
		}
	}
	return "?" + string(st.Focus)
}

func fileDialogState(app *App) string {
	st := app.model.FileDialog
	mask := func(s string) string { return strings.ReplaceAll(s, app.model.UserHomeDir, "<tmp>") }
	var fields []string
	for _, f := range st.Fields {
		cursor, scroll := fmt.Sprint(f.Cursor), fmt.Sprint(f.Scroll)
		if strings.Contains(f.Value, app.model.UserHomeDir) {
			cursor, scroll = fmt.Sprintf("end-%d", len([]rune(f.Value))-f.Cursor), "-"
		}
		fields = append(fields, fmt.Sprintf("%q@%s/%s pending=%v dropdown=%v/%d", mask(f.Value), cursor, scroll, f.PrefillPending, f.Completion.Open, f.Completion.Selected))
	}
	return fmt.Sprintf("focus=%s phase=%d fields=[%s] focusAfter=%v openOther=%v mkdirAction=%d sanitize=%v/%v slugify=%c enc=%d delScroll=%d message=%q",
		fileFocusName(st), st.RenamePhase, strings.Join(fields, "; "), st.RenameFocusAfter, st.RenameOpenInOther, st.MkdirAction,
		st.RenameSanitizeDots, st.RenameSanitizeUnderscores, st.RenameSlugifySep, st.RenameEncodingSelected, st.DeleteListScroll, mask(app.model.Message))
}

// fileKeyDialog builds a key-golden dialog. open opens the dialog under test on the prepared app
// (panel at <dir>/projects/harbor holding lantern.txt, willow.txt, anchor.txt, the directories
// orchard and meadow, and files f01..f25.txt). listing prints the harbor listing on close, for
// dialogs whose effect is synchronous.
func fileKeyDialog(name string, listing bool, open func(t *testing.T, app *App)) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			harbor := filepath.Join(dir, "projects", "harbor")
			for _, d := range []string{harbor, filepath.Join(harbor, "orchard"), filepath.Join(harbor, "meadow")} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range []string{"lantern.txt", "willow.txt", "anchor.txt"} {
				writeFile(t, filepath.Join(harbor, f))
			}
			for i := 1; i <= 25; i++ {
				writeFile(t, filepath.Join(harbor, fmt.Sprintf("f%02d.txt", i)))
			}
			app := newApp(t, newScreen(t, 80, 24), harbor)
			app.model.Message = ""
			app.model.UserHomeDir = dir
			open(t, app)
			if !app.model.FileDialog.Open {
				t.Fatal("file dialog should be open")
			}
			return app
		},
		state: fileDialogState,
		open:  func(app *App) bool { return app.model.FileDialog.Open },
		outcome: func(app *App, _ bool) string {
			mask := func(s string) string { return strings.ReplaceAll(s, app.model.UserHomeDir, "<tmp>") }
			var jobsList []string
			for _, j := range app.jobState.AllJobs() {
				jobsList = append(jobsList, fmt.Sprintf("%s sources=%d", j.Type, len(j.Sources)))
			}
			out := fmt.Sprintf("message=%q jobs=[%s]", mask(app.model.Message), strings.Join(jobsList, "; "))
			if listing {
				entries, _ := os.ReadDir(filepath.Join(app.model.UserHomeDir, "projects", "harbor"))
				var names []string
				for _, e := range entries {
					if !strings.HasPrefix(e.Name(), "f") || e.IsDir() {
						names = append(names, e.Name())
					}
				}
				out += " files=" + strings.Join(names, ",")
			}
			return out
		},
	}
}

func dispatchFileOp(t *testing.T, app *App, action string) {
	t.Helper()
	if !app.dialogCtrl.TryDispatchFileOps(action) {
		t.Fatalf("action %s not dispatched", action)
	}
}

func openRenameOn(name string) func(t *testing.T, app *App) {
	return func(t *testing.T, app *App) {
		selectPanelEntryByName(t, app.activePanel(), name)
		dispatchFileOp(t, app, keymap.ActionFileRename)
	}
}

func TestFileRenameKeyScripts(t *testing.T) {
	file := fileKeyDialog("file_rename", true, openRenameOn("lantern.txt"))
	runKeyGoldenScripts(t, file, []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"commit-prefill-right", "Right x"},
		{"commit-prefill-home", "Home x"},
		{"commit-prefill-end", "End x"},
		{"commit-prefill-delete", "Home Delete Delete"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"restore-default", "x y z Alt+Backspace"},
		{"space-in-name", "a Space b"},
		{"letters-in-name", "a t p r o c y n O C"},
		{"space-checkbox", "Down Space Space"},
		{"enter-checkbox", "Down Enter Enter"},
		{"mnemonic-in-name", "a Alt+a b Alt+A"},
		{"mnemonic-on-button", "Down Down Alt+a Alt+a"},
		{"mnemonic-open-other-ignored", "Alt+p Alt+x"},
		{"plain-letters-on-checkbox", "Down a p Space"},
		{"plain-letters-on-buttons", "Down Down o c a x"},
		{"whitespace-warning", "x Space Down Up"},
		{"whitespace-warning-lead", "Home Space Down Tab"},
		{"enter-in-name-renames", "e m b e r . t x t Enter"},
		{"enter-in-name-same", "Enter"},
		{"enter-in-name-empty", "a Backspace Enter"},
		{"enter-in-name-existing", "w i l l o w . t x t Enter"},
		{"enter-ok", "e m b e r . t x t Down Down Enter"},
		{"enter-cancel", "Down Down Right Enter"},
		{"space-ok", "e m b e r . t x t Down Down Space"},
		{"space-cancel", "Down Down Right Space"},
		{"alt-o", "e m b e r . t x t Alt+o"},
		{"alt-o-from-checkbox", "e m b e r . t x t Down Alt+o"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "e m b e r . t x t Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Down Right Esc"},
		{"focus-after-then-rename", "Alt+a e m b e r . t x t Enter"},
		{"tool-chords-sanitize", "F2"},
		{"tool-chords-slugify", "F3"},
		{"tool-chord-encoding-without-candidates", "F4"},
		{"tool-chord-from-checkbox", "Down F2"},
	})
	dir := fileKeyDialog("file_rename_dir", true, openRenameOn("orchard"))
	runKeyGoldenScripts(t, dir, []simpleKeyScript{
		{"space-checkboxes", "Down Space Down Space"},
		{"enter-checkboxes", "Down Enter Down Enter"},
		{"mnemonics-in-name", "a Alt+a Alt+p b"},
		{"mnemonics-on-button", "Down Down Down Alt+p Alt+a"},
		{"plain-letters", "Down Down a p x"},
		{"open-other-then-rename", "Alt+p e m b e r Enter"},
		{"whitespace-warning", "x Space Down Down"},
		{"enter-ok", "e m b e r Down Down Down Enter"},
		{"esc", "Down Down Esc"},
	})
}

func TestFileRenameToolKeyScripts(t *testing.T) {
	open := func(chord string, candidates bool) func(t *testing.T, app *App) {
		return func(t *testing.T, app *App) {
			openRenameOn("lantern.txt")(t, app)
			d := &app.model.FileDialog
			d.Fields[0].Value, d.Fields[0].Cursor = "lantern orchard_willow.meadow txt", 5
			if candidates {
				d.RenameEncodingCandidates = []dialog.RenameEncodingCandidate{
					{Label: "Shift-JIS", UTF8: "lantern orchard.txt"},
					{Label: "EUC-JP", UTF8: "willow meadow.txt"},
					{Label: "Windows-31J", UTF8: "harbor ribbon.txt"},
				}
			}
			app.handleKey(goldenKeyEvent(t, chord))
			if d.RenamePhase == dialog.RenamePhaseMain {
				t.Fatalf("%s did not open a tool screen", chord)
			}
		}
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_rename_sanitize", false, open("F2", false)), []simpleKeyScript{
		{"down-walk", "Down Down Down Down"},
		{"space-toggles", "Up Space Up Space Space"},
		{"enter-toggles", "Up Enter Up Enter"},
		{"mnemonic-dot", "Alt+. Alt+."},
		{"mnemonic-underscore", "Alt+_ Alt+_"},
		{"mnemonics-from-ok", "Alt+. Alt+_ Down"},
		{"plain-letters", "o c . _ x"},
		{"enter-ok-applies", "Up Space Down Enter"},
		{"enter-ok-direct", "Enter"},
		{"enter-cancel", "Right Enter"},
		{"space-on-ok", "Space"},
		{"esc", "Esc"},
		{"alt-o", "Alt+o"},
		{"alt-c", "Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"esc-then-main", "Up Space Esc Down"},
	})
	runKeyGoldenScripts(t, fileKeyDialog("file_rename_slugify", false, open("F3", false)), []simpleKeyScript{
		{"down-walk", "Down Down Down Down"},
		{"space-selects", "Up Down Up Space"},
		{"enter-selects", "Up Down Up Enter Up Enter"},
		{"mnemonics", "Alt+_ Alt+. Alt+_"},
		{"enter-ok-applies", "Alt+_ Down Down Enter"},
		{"enter-cancel", "Right Enter"},
		{"esc", "Esc"},
		{"alt-o", "Alt+_ Alt+o"},
		{"alt-c", "Alt+c"},
	})
	runKeyGoldenScripts(t, fileKeyDialog("file_rename_encoding", false, open("F4", true)), []simpleKeyScript{
		{"down-walk", "Down Down Down Down"},
		{"space-selects", "Up Space Up Space Up Space"},
		{"enter-selects", "Up Enter Up Enter"},
		{"mnemonics", "Alt+E Alt+W Alt+S"},
		{"mnemonics-lower", "Alt+e Alt+w"},
		{"enter-ok-applies", "Alt+E Down Down Down Enter"},
		{"enter-cancel", "Right Enter"},
		{"esc", "Esc"},
		{"alt-o", "Alt+E Alt+o"},
		{"alt-c", "Alt+c"},
	})
}

func TestFileDuplicateKeyScripts(t *testing.T) {
	open := func(t *testing.T, app *App) {
		selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
		dispatchFileOp(t, app, keymap.ActionFileDuplicate)
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_duplicate", false, open), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d"},
		{"restore-default", "x y z Alt+Backspace"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"space-checkbox", "Down Space Space"},
		{"enter-checkbox", "Down Enter Enter"},
		{"mnemonic-in-name", "a Alt+a b"},
		{"mnemonic-on-button", "Down Down Alt+a"},
		{"mnemonic-p-ignored", "Alt+p"},
		{"whitespace-warning", "x Space Down"},
		{"enter-in-name-queues", "e m b e r . t x t Enter"},
		{"enter-in-name-empty", "a Backspace Enter"},
		{"enter-ok", "e m b e r . t x t Down Down Enter"},
		{"enter-cancel", "Down Down Right Enter"},
		{"space-ok", "e m b e r . t x t Down Down Space"},
		{"alt-o", "e m b e r . t x t Alt+o"},
		{"alt-c", "a Alt+c"},
		{"esc", "a Esc"},
		{"tool-chords-ignored-or-open", "F2"},
	})
}

func TestFileMkdirKeyScripts(t *testing.T) {
	plain := func(t *testing.T, app *App) {
		selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
		dispatchFileOp(t, app, keymap.ActionFileMkdir)
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_mkdir", true, plain), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d"},
		{"restore-default", "x y z Alt+Backspace"},
		{"extract-chord-without-selection", "F7"},
		{"down-walk", "Down Down Down Down"},
		{"mnemonics-without-actions", "a Alt+r Alt+y Alt+m b"},
		{"plain-letters-on-buttons", "Down o c r y m x"},
		{"enter-creates", "e m b e r Enter"},
		{"enter-empty", "a Backspace Enter"},
		{"enter-existing", "o r c h a r d Enter"},
		{"enter-ok", "e m b e r Down Enter"},
		{"enter-cancel", "Down Right Enter"},
		{"space-ok", "e m b e r Down Space"},
		{"space-cancel", "Down Right Space"},
		{"alt-o", "e m b e r Alt+o"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "e m b e r Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
	})
	withActions := func(t *testing.T, app *App) {
		p := app.activePanel()
		p.AddSelection(filepath.Join(app.model.UserHomeDir, "projects", "harbor", "willow.txt"))
		p.AddSelection(filepath.Join(app.model.UserHomeDir, "projects", "harbor", "anchor.txt"))
		selectPanelEntryByName(t, p, "lantern.txt")
		dispatchFileOp(t, app, keymap.ActionFileMkdir)
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_mkdir_actions", false, withActions), []simpleKeyScript{
		{"space-radios", "Down Space Down Space Down Space Up Space"},
		{"enter-radios", "Down Down Enter"},
		{"enter-first-radio", "Down Enter"},
		{"mnemonics-in-name", "a Alt+y Alt+m Alt+r b"},
		{"mnemonics-on-radio", "Down Alt+m Alt+y Alt+r"},
		{"mnemonics-on-button", "Down Down Down Down Alt+y Alt+m"},
		{"mnemonic-upper", "Alt+Y Alt+M Alt+R"},
		{"plain-letters-on-radio", "Down y m r x Space"},
		{"extract-chord-needs-two", "F7"},
		{"enter-name-creates-copy", "Alt+y e m b e r Enter"},
		{"enter-name-creates-move", "Alt+m e m b e r Enter"},
		{"enter-ok-after-radio", "e m b e r Alt+y Down Down Down Down Enter"},
		{"space-ok", "e m b e r Down Down Down Down Space"},
		{"alt-o-copy", "e m b e r Alt+y Alt+o"},
		{"alt-c-cancels", "Alt+c"},
		{"esc", "Down Esc"},
	})
}

func TestFileDeleteKeyScripts(t *testing.T) {
	// The impact summary is fixed (the real one arrives from disk-usage events the tests never pump).
	fix := func(app *App) {
		d := &app.model.FileDialog
		d.DeleteSummary = fmt.Sprintf("%d files (0 B)", len(d.DeleteEntries))
	}
	few := func(t *testing.T, app *App) {
		p := app.activePanel()
		for _, f := range []string{"lantern.txt", "willow.txt", "anchor.txt"} {
			p.AddSelection(filepath.Join(app.model.UserHomeDir, "projects", "harbor", f))
		}
		dispatchFileOp(t, app, keymap.ActionFileDelete)
		fix(app)
	}
	many := func(t *testing.T, app *App) {
		p := app.activePanel()
		for i := 1; i <= 25; i++ {
			p.AddSelection(filepath.Join(app.model.UserHomeDir, "projects", "harbor", fmt.Sprintf("f%02d.txt", i)))
		}
		dispatchFileOp(t, app, keymap.ActionFileDelete)
		fix(app)
	}
	one := func(t *testing.T, app *App) {
		selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
		dispatchFileOp(t, app, keymap.ActionFileDelete)
		fix(app)
	}
	scripts := []simpleKeyScript{
		{"default-no", "Enter"},
		{"enter-yes", "Left Enter"},
		{"space-yes", "Left Space"},
		{"space-no", "Space"},
		{"alt-y", "Alt+y"},
		{"alt-y-upper", "Alt+Y"},
		{"alt-n", "Left Alt+n"},
		{"plain-y", "y"},
		{"plain-n", "Left n"},
		{"plain-other", "x q Down"},
		{"alt-o", "Alt+o"},
		{"alt-c", "Left Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"esc", "Left Esc"},
		{"pgdn-pgup", "PgDn PgDn PgUp PgUp PgUp"},
		{"pgdn-to-end", "PgDn PgDn PgDn PgDn PgDn"},
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_delete", false, few), scripts)
	runKeyGoldenScripts(t, fileKeyDialog("file_delete_many", false, many), scripts)
	runKeyGoldenScripts(t, fileKeyDialog("file_delete_one", false, one), []simpleKeyScript{
		{"default-no", "Enter"},
		{"enter-yes", "Left Enter"},
		{"alt-y", "Alt+y"},
		{"esc", "Esc"},
	})
	dangling := func(t *testing.T, app *App) {
		harbor := filepath.Join(app.model.UserHomeDir, "projects", "harbor")
		app.dialogCtrl.PromptDanglingDirDelete([]string{filepath.Join(harbor, "orchard"), filepath.Join(harbor, "meadow")})
		d := &app.model.FileDialog
		for i, n := range []string{"ember", "pebble"} {
			d.DeleteEntries[i].Name = n // the shown labels are paths; keep the temp dir out of the transcript
		}
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_delete_dangling", false, dangling), []simpleKeyScript{
		{"default-yes", "Enter"},
		{"right-enter", "Right Enter"},
		{"alt-n", "Alt+n"},
		{"alt-y", "Right Alt+y"},
		{"esc", "Esc"},
		{"space-no", "Right Space"},
	})
}

func attrScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"replace", "a b c"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d"},
		{"letters-in-input", "o c y n O C"},
		{"space-in-input", "a Space b"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"plain-letters-on-buttons", "Down Down o c x"},
		{"mnemonic-unknown", "a Alt+x Alt+q b"},
		{"enter-empty", "Enter"},
		{"enter-in-second", "Down Enter"},
		{"enter-ok", "Down Down Enter"},
		{"enter-cancel", "Down Down Right Enter"},
		{"space-ok", "Down Down Space"},
		{"space-cancel", "Down Down Right Space"},
		{"alt-o-empty", "Alt+o"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Down Right Esc"},
	}
}

func TestFileChmodChownKeyScripts(t *testing.T) {
	chmod := func(t *testing.T, app *App) {
		selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
		dispatchFileOp(t, app, keymap.ActionFileChmod)
	}
	chown := func(t *testing.T, app *App) {
		selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
		dispatchFileOp(t, app, keymap.ActionFileChown)
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_chmod", false, chmod), []simpleKeyScript{
		{"type-mode", "0 6 4 4"},
		{"edit-cursor", "0 6 4 4 Left Left Backspace Home Delete End 5 Right"},
		{"edit-keys", "7 5 5 Ctrl+W 6 4 4 Ctrl+U 7"},
		{"letters-in-input", "o c y n O C"},
		{"down-walk", "Down Down Down Down"},
		{"plain-letters-on-buttons", "Down o c x"},
		{"enter-empty", "Enter"},
		{"enter-bad-mode", "z z z Enter"},
		{"enter-cancel", "Down Right Enter"},
		{"space-cancel", "Down Right Space"},
		{"alt-c", "a Alt+c"},
		{"alt-o-empty", "Alt+o"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
		{"mnemonic-unknown", "a Alt+x b"},
	})
	runKeyGoldenScripts(t, fileKeyDialog("file_chown", false, chown), attrScripts())
}

func linkScripts() []simpleKeyScript {
	s := attrScripts()
	return append(s, []simpleKeyScript{
		{"complete-ghost", "~ / p r o j e c t s / h a r b o r / o"},
		{"complete-ghost-tab", "~ / p r o j e c t s / h a r b o r / o Tab"},
		{"complete-open-tab", "~ / p r o j e c t s / h a r b o r / Tab"},
		{"complete-open-down-enter", "~ / p r o j e c t s / h a r b o r / Tab Down Enter"},
		{"complete-open-esc", "~ / p r o j e c t s / h a r b o r / Tab Esc"},
		{"complete-open-esc-esc", "~ / p r o j e c t s / h a r b o r / Tab Esc Esc"},
		{"complete-open-then-tab-away", "~ / p r o j e c t s / h a r b o r / Tab Tab"},
		{"complete-open-then-down", "~ / p r o j e c t s / h a r b o r / Tab Down Down Down"},
		{"complete-second-field", "Tab ~ / p r o j e c t s / h a r b o r / Tab"},
		{"complete-second-then-up", "Tab ~ / p r o j e c t s / h a r b o r / Tab Up"},
		{"complete-both-open", "~ / p r o j e c t s / h a r b o r / Tab Down ~ / p r o j e c t s / h a r b o r / Tab"},
		{"complete-open-alt-c", "~ / p r o j e c t s / h a r b o r / Tab Alt+c"},
		{"submit-queues-or-creates", "Enter"},
		{"submit-ok", "Tab Tab Enter"},
		{"submit-space-ok", "Tab Tab Space"},
		{"submit-alt-o", "Alt+o"},
		{"submit-from-second", "Down Enter"},
		{"submit-same-path", "Down Enter"},
	}...)
}

func TestFileSymlinkHardlinkKeyScripts(t *testing.T) {
	link := func(action string) func(t *testing.T, app *App) {
		return func(t *testing.T, app *App) {
			selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
			dispatchFileOp(t, app, action)
			d := &app.model.FileDialog
			// The default paths hold the temp dir; relative placeholders (resolved against the
			// temp panel directory) keep the transcript stable and the effect inside it.
			set := func(i int, v string) {
				d.Fields[i].Value, d.Fields[i].Cursor = v, len([]rune(v))
				d.Fields[i].Scroll = 0
				d.Fields[i].Completion.Clear()
			}
			if d.DialogType == dialog.FileDialogSymlink {
				set(0, "lantern.txt")
				set(1, "ember-link")
			} else {
				set(0, "lantern.txt")
				set(1, "ember-hard")
			}
		}
	}
	runKeyGoldenScripts(t, fileKeyDialog("file_symlink", true, link(keymap.ActionFileSymlink)), linkScripts())
	runKeyGoldenScripts(t, fileKeyDialog("file_hardlink", true, link(keymap.ActionFileHardlink)), linkScripts())
}
