package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the Rsync dialog, using the harness of simple_dialog_keys_golden_test.go.
// The dialog opens on a fixed fake source (/srv/harbor/orchard) and a "~" destination under the
// temp home, so the preview never shows a temp path; Enter and OK queue a job (the sources do not
// exist, so it fails at once) and report it.

// rsyncFocusName names the focused widget of the Rsync dialog: f0..f3 (the text inputs), update,
// mirror, move (the mode radios), archive, checksum, newer, skip, contents, compress, dry (the
// checkboxes), ok, cancel. It is the only code that knows how the state encodes focus.
func rsyncFocusName(st dialog.FileDialogState) string {
	names := map[dlg.ID]string{
		dlg.OK: "ok", dlg.Cancel: "cancel",
		dialog.RsyncModeUpdate: "update", dialog.RsyncModeMirror: "mirror", dialog.RsyncModeMove: "move",
		dialog.RsyncArchive: "archive", dialog.RsyncChecksum: "checksum", dialog.RsyncOnlyNewer: "newer",
		dialog.RsyncSkipExist: "skip", dialog.RsyncContentsDir: "contents", dialog.RsyncCompress: "compress", dialog.RsyncDryRun: "dry",
	}
	if n, ok := names[st.Focus]; ok {
		return n
	}
	if i, ok := dialog.FileFieldIndex(st.Focus); ok {
		return fmt.Sprintf("f%d", i)
	}
	return "f0" // an unset focus is the first widget
}

func rsyncDialogState(app *App) string {
	st := app.model.FileDialog
	var fields []string
	for _, f := range st.Fields {
		cursor, scroll := fmt.Sprint(f.Cursor), fmt.Sprint(f.Scroll)
		if strings.Contains(f.Value, app.model.UserHomeDir) { // the temp dir name's length varies
			cursor, scroll = fmt.Sprintf("end-%d", len([]rune(f.Value))-f.Cursor), "-"
		}
		fields = append(fields, fmt.Sprintf("%q@%s/%s pending=%v dropdown=%v", maskDir(f.Value, app.model.UserHomeDir), cursor, scroll, f.PrefillPending, f.Completion.Open))
	}
	o := st.RsyncOptions
	flags := ""
	for _, f := range []struct {
		on bool
		c  string
	}{{o.Archive, "a"}, {o.Checksum, "c"}, {o.OnlyNewer, "u"}, {o.SkipExisting, "s"}, {o.ContentsOnly, "d"}, {o.Compress, "z"}, {o.DryRun, "n"}} {
		if f.on {
			flags += f.c
		}
	}
	return fmt.Sprintf("focus=%s fields=[%s] mode=%d flags=%s preview=%q err=%q",
		rsyncFocusName(st), strings.Join(fields, "; "), o.Mode, flags, maskDir(strings.Join(st.RsyncPreview, " "), app.model.UserHomeDir), st.RsyncError)
}

// rsyncKeyDialog builds the Rsync key-golden dialog; dirs makes the fake source a directory, so
// Copy dir contents is enabled.
func rsyncKeyDialog(name string, dirs bool) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			writeFile(t, filepath.Join(dir, "lantern.txt"))
			app := newApp(t, newScreen(t, 100, 40), dir)
			app.model.Message = ""
			app.model.UserHomeDir = dir
			app.dialogCtrl.SetLookPathForTest(func(string) (string, error) { return "/usr/bin/rsync", nil })
			selectPanelEntryByName(t, app.activePanel(), "lantern.txt")
			app.dispatch(keymap.ActionFileRsync)
			fd := &app.model.FileDialog
			if !fd.Open || fd.DialogType != dialog.FileDialogRsync {
				t.Fatal("rsync dialog should be open")
			}
			fd.RsyncSources, fd.RsyncAllDirs = pathloc.PathsForTest("/srv/harbor/orchard"), dirs
			f := &fd.Fields[dialog.RsyncFieldDest]
			f.Value, f.Prefill, f.Cursor, f.PrefillPending, f.Scroll = "~/projects/meadow/", "~/projects/meadow/", 19, true, 0
			f.Completion.Clear()
			// Any event recomputes the preview; this one changes nothing.
			app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: "none"})
			return app
		},
		state: rsyncDialogState,
		open:  func(app *App) bool { return app.model.FileDialog.Open },
		outcome: func(app *App, _ bool) string {
			jobs := app.jobState.AllJobs()
			typ := ""
			if len(jobs) > 0 {
				typ = string(jobs[0].Type)
			}
			return fmt.Sprintf("jobs=%d type=%q message=%q", len(jobs), typ, maskDir(app.model.Message, app.model.UserHomeDir))
		},
	}
}

func TestRsyncKeyScripts(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Log("rsync not installed: the queued job fails to start, which the goldens do not read")
	}
	runKeyGoldenScripts(t, rsyncKeyDialog("rsync", false), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"commit-prefill-home", "Home x"},
		{"edit-cursor", "End x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-keys", "End x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"restore-default", "End x y z Alt+Backspace"},
		{"space-in-dest", "End a Space b"},
		{"letters-in-dest", "End a t p r o c y n O C"},
		{"down-walk", "Down Down Down Down Down Down Down Down Down Down Down Down Down Down"},
		{"up-from-ok", "Down Down Down Down Down Down Down Down Down Down Down Down Up Up Up"},
		{"tab-walk", "Tab Tab Tab Tab Tab Tab Tab Tab"},
		{"shift-tab-walk", "Backtab Backtab Backtab Backtab"},
		{"mode-select-mirror", "Down Right Space"},
		{"mode-select-move", "Down Right Right Enter"},
		{"mode-then-update", "Down Right Space Left Space"},
		{"mode-mnemonics", "Alt+m Alt+v Alt+u"},
		{"checkbox-toggle-space", "Down Down Space"},
		{"checkbox-toggle-enter", "Down Down Enter"},
		{"checkbox-mnemonics", "Alt+a Alt+h Alt+n Alt+s Alt+z Alt+r"},
		{"contents-disabled", "Alt+d"},
		{"contents-skipped-by-down", "Down Down Down Down Down"},
		{"exclude-typing", "Tab Tab x Space y"},
		{"bandwidth-typing", "Tab Tab Tab 5 1 2"},
		{"bandwidth-invalid-enter", "Tab Tab Tab a Enter"},
		{"extra-typing", "Tab Tab Tab Tab - - h Space - - a"},
		{"extra-unbalanced-quote", "Tab Tab Tab Tab ' Enter"},
		{"mnemonic-in-dest", "End a Alt+x Alt+q b"},
		{"mnemonic-on-button", "Tab Tab Tab Tab Tab Alt+x"},
		{"enter-in-dest-queues", "End x Enter"},
		{"enter-in-dest-empty", "Backspace Backspace Enter"},
		{"enter-ok", "Tab Tab Tab Tab Tab Enter"},
		{"enter-cancel", "Tab Tab Tab Tab Tab Right Enter"},
		{"alt-o", "x Alt+o"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "x Alt+Enter"},
		{"mirror-then-ok", "Alt+m Alt+o"},
		{"dry-run-then-ok", "Alt+r Alt+o"},
		{"esc", "a Esc"},
		{"esc-from-button", "Tab Tab Tab Tab Tab Right Esc"},
		{"f-keys-ignored", "F2 F3 F4 F7 a"},
		{"page-keys-ignored", "PgUp PgDn a"},
		{"dest-shortcut-active", "End Shift+Left"},
		{"dest-shortcut-inactive", "End Shift+Right"},
		{"dest-shortcuts-off-input", "Down Shift+Left Shift+Right"},
	})
	runKeyGoldenScripts(t, rsyncKeyDialog("rsync_dirs", true), []simpleKeyScript{
		{"contents-toggle", "Alt+d"},
		{"contents-walk", "Down Down Down Down Space"},
		{"contents-then-ok", "Alt+d Alt+o"},
	})
}
