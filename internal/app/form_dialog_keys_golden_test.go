package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the single-input file dialogs outside the rename family: add bookmark,
// the SFTP password prompt and archive extract. They use the harness of
// simple_dialog_keys_golden_test.go. The dialogs open on temp paths, so each setup rewrites the
// text the dialog shows to fixed words before the first state is recorded.

// formFocusName names the focused widget of a single-input file dialog: f0 (the input), ok,
// cancel. It is the only code that knows how the state encodes focus.
func formFocusName(st dialog.FileDialogState) string {
	switch st.Focus {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "f0"
}

func formDialogState(app *App) string {
	st := app.model.FileDialog
	var fields []string
	for _, f := range st.Fields {
		cursor, scroll := fmt.Sprint(f.Cursor), fmt.Sprint(f.Scroll)
		if strings.Contains(f.Value, app.model.UserHomeDir) {
			cursor, scroll = fmt.Sprintf("end-%d", len([]rune(f.Value))-f.Cursor), "-"
		}
		fields = append(fields, fmt.Sprintf("%q@%s/%s pending=%v dropdown=%v/%d", maskDir(f.Value, app.model.UserHomeDir), cursor, scroll, f.PrefillPending, f.Completion.Open, f.Completion.Selected))
	}
	return fmt.Sprintf("focus=%s fields=[%s] message=%q", formFocusName(st), strings.Join(fields, "; "), st.Message)
}

// formKeyDialog builds a key-golden dialog on an app whose panel sits in a fresh temp dir. open
// opens the dialog under test; outcome describes what closing did.
func formKeyDialog(name string, open func(t *testing.T, app *App, dir string), outcome func(app *App, dir string) string) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			app.model.UserHomeDir = dir
			open(t, app, dir)
			if !app.model.FileDialog.Open {
				t.Fatal("dialog should be open")
			}
			return app
		},
		state: formDialogState,
		open:  func(app *App) bool { return app.model.FileDialog.Open },
		outcome: func(app *App, _ bool) string {
			return outcome(app, app.model.UserHomeDir)
		},
	}
}

func maskDir(s, dir string) string { return strings.ReplaceAll(s, dir, "<tmp>") }

func TestBookmarkAddKeyScripts(t *testing.T) {
	d := formKeyDialog("bookmark_add",
		func(t *testing.T, app *App, dir string) {
			app.config.Bookmarks.File = filepath.Join(dir, ".fzf-marks")
			app.dispatch(keymap.ActionBookmarkAdd)
			// The dialog opened on a temp path; show fixed words instead.
			fd := &app.model.FileDialog
			fd.Message = "/srv/harbor/orchard"
			fd.Fields[0].Value, fd.Fields[0].Prefill, fd.Fields[0].Cursor, fd.Fields[0].PrefillPending = "orchard", "orchard", 7, true
		},
		func(app *App, dir string) string {
			data, _ := os.ReadFile(filepath.Join(dir, ".fzf-marks"))
			return fmt.Sprintf("message=%q urgency=%v marks=%q", maskDir(app.model.Message, dir), app.model.MessageUrgency, string(data))
		})
	runKeyGoldenScripts(t, d, []simpleKeyScript{
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
		{"down-walk", "Down Down Down Down"},
		{"up-walk", "Down Down Up Up Up"},
		{"tab", "Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab"},
		{"left-right-buttons", "Down Left Right Right Right Left Left Left"},
		{"mnemonic-in-name", "a Alt+x Alt+q b"},
		{"mnemonic-on-button", "Down Alt+x"},
		{"plain-letters-on-buttons", "Down o c a x"},
		{"enter-in-name-adds", "e m b e r Enter"},
		{"enter-in-name-prefill", "Enter"},
		{"enter-in-name-empty", "Backspace Backspace Enter"},
		{"enter-in-name-blank", "Space Enter"},
		{"enter-ok", "e m b e r Down Enter"},
		{"enter-cancel", "e m b e r Down Right Enter"},
		{"space-ok", "e m b e r Down Space"},
		{"space-cancel", "Down Right Space"},
		{"alt-o", "e m b e r Alt+o"},
		{"alt-o-upper", "e m b e r Alt+O"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "e m b e r Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
		{"f-keys-ignored", "F2 F3 F4 F7 a"},
		{"page-keys-ignored", "PgUp PgDn a"},
	})
}

// sftpPasswordReplies holds the reply channel of each SFTP password key-golden app, so the
// outcome can report what the prompt answered.
var sftpPasswordReplies sync.Map

func TestSFTPPasswordKeyScripts(t *testing.T) {
	d := formKeyDialog("sftp_password",
		func(t *testing.T, app *App, _ string) {
			reply := make(chan sftpPasswordReply, 1)
			prompt := sftpb.PasswordPrompt{User: "alice", Host: "example.com"}
			app.sftp.mu.Lock()
			app.sftp.passwordWait = &sftpPasswordWait{id: 1, prompt: prompt, reply: reply}
			app.sftp.mu.Unlock()
			sftpPasswordReplies.Store(app, reply)
			app.openSFTPPasswordDialog(prompt)
		},
		func(app *App, _ string) string {
			ch, _ := sftpPasswordReplies.Load(app)
			select {
			case r := <-ch.(chan sftpPasswordReply):
				return fmt.Sprintf("password=%q canceled=%v", r.password, r.err != nil)
			default:
				return "no reply"
			}
		})
	runKeyGoldenScripts(t, d, []simpleKeyScript{
		{"type", "s e c r e t"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"space-in-password", "a Space b"},
		{"letters-in-password", "a t p r o c y n O C"},
		{"down-walk", "Down Down Down Down"},
		{"up-walk", "Down Down Up Up Up"},
		{"tab", "Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab"},
		{"left-right-buttons", "Down Left Right Right Right Left Left Left"},
		{"mnemonic-in-password", "a Alt+x Alt+q b"},
		{"mnemonic-on-button", "Down Alt+x"},
		{"plain-letters-on-buttons", "Down o c a x"},
		{"enter-in-password", "s e c r e t Enter"},
		{"enter-empty", "Enter"},
		{"enter-ok", "s e c r e t Down Enter"},
		{"enter-cancel", "s e c r e t Down Right Enter"},
		{"space-ok", "s e c r e t Down Space"},
		{"space-cancel", "Down Right Space"},
		{"alt-o", "s e c r e t Alt+o"},
		{"alt-o-upper", "s e c r e t Alt+O"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "s e c r e t Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
		{"f-keys-ignored", "F2 F3 F4 F7 a"},
		{"page-keys-ignored", "PgUp PgDn a"},
	})
}

// openExtractOn opens the extract dialog on a panel holding two archives (empty files) and a text
// file, all selected when note is true (so the dialog carries the skipped-items note), else only
// the archives. The destination is rewritten to a "~" path under the temp home so the state and the
// completion never depend on the temp dir name; Enter on it fails the plan (no such directory)
// and shows the failure in a message dialog, so no job runs. The extract does not expand "~", so
// the test runs from the temp dir: the relative path it resolves stays inside the sandbox.
// extractKeyDialog is formKeyDialog over openExtractOn, kept serial because its setup changes the
// working directory (t.Chdir).
func extractKeyDialog(name string, note bool) keyGoldenDialog {
	d := formKeyDialog(name, openExtractOn(note), extractOutcome)
	d.serial = true
	return d
}

func openExtractOn(note bool) func(t *testing.T, app *App, dir string) {
	return func(t *testing.T, app *App, dir string) {
		t.Chdir(dir)
		harbor := app.activePanel().PathString()
		for _, d := range []string{"orchard", "otter", "oak"} {
			if err := os.MkdirAll(filepath.Join(dir, "projects", "meadow", d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		names := []string{"lantern.tar.gz", "willow.zip"}
		if note {
			names = append(names, "anchor.txt")
		}
		for _, n := range names {
			writeFile(t, filepath.Join(harbor, n))
			app.activePanel().AddSelection(filepath.Join(harbor, n))
		}
		app.dispatch(keymap.ActionFileExtract)
		fd := &app.model.FileDialog
		if !fd.Open || fd.DialogType != dialog.FileDialogExtract {
			t.Fatal("extract dialog should be open")
		}
		f := &fd.Fields[0]
		f.Value, f.Prefill, f.Cursor, f.PrefillPending = "~/projects/meadow/", "~/projects/meadow/", 18, true
		f.Scroll = 0
		f.Completion.Clear()
	}
}

func extractOutcome(app *App, dir string) string {
	md := app.model.MessageDialog
	return fmt.Sprintf("jobs=%d messageDialog=%v title=%q text=%q message=%q", len(app.jobState.AllJobs()), md.Open, md.Title, maskDir(md.Message, dir), maskDir(app.model.Message, dir))
}

func TestExtractKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"commit-prefill-right", "Right x"},
		{"commit-prefill-home", "Home x"},
		{"commit-prefill-end", "End x"},
		{"commit-prefill-delete", "Home Delete Delete"},
		{"edit-cursor", "End x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "End x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"restore-default", "End x y z Alt+Backspace"},
		{"space-in-dest", "End a Space b"},
		{"letters-in-dest", "End a t p r o c y n O C"},
		{"completion-dropdown", "End o"},
		{"completion-down-up", "End o Down Down Up Down"},
		{"completion-tab-accept", "End o Tab"},
		{"completion-enter-accept", "End o Down Enter"},
		{"completion-esc-closes", "End o Esc"},
		{"completion-esc-twice", "End o Esc Esc"},
		{"completion-then-tab-focus", "End o Esc Tab"},
		{"down-walk", "Down Down Down Down"},
		{"up-walk", "Down Down Up Up Up"},
		{"tab", "Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab"},
		{"left-right-buttons", "Down Left Right Right Right Left Left Left"},
		{"mnemonic-in-dest", "End a Alt+x Alt+q b"},
		{"mnemonic-on-button", "Down Alt+x"},
		{"plain-letters-on-buttons", "Down o c a x"},
		{"enter-in-dest", "End x Enter"},
		{"enter-ok", "End x Down Enter"},
		{"enter-cancel", "End x Down Right Enter"},
		{"space-ok", "End x Down Space"},
		{"space-cancel", "Down Right Space"},
		{"alt-o", "End x Alt+o"},
		{"alt-o-upper", "End x Alt+O"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "End x Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
		{"dest-shortcut-active", "End Shift+Left"},
		{"dest-shortcut-inactive", "End Shift+Right"},
		{"dest-shortcuts-off-input", "Down Shift+Left Shift+Right"},
		{"f-keys-ignored", "F2 F3 F4 F7 a"},
		{"page-keys-ignored", "PgUp PgDn a"},
	}
	runKeyGoldenScripts(t, extractKeyDialog("extract", true), scripts)
	runKeyGoldenScripts(t, extractKeyDialog("extract_plain", false), []simpleKeyScript{
		{"open-tab", "Tab Tab Tab"},
		{"down-walk", "Down Down Down"},
		{"enter-in-dest", "End x Enter"},
		{"enter-ok", "End x Down Enter"},
		{"space-ok", "End x Down Space"},
		{"alt-enter", "End x Alt+Enter"},
		{"esc", "a Esc"},
	})
}
