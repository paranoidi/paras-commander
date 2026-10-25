package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the run-for-each dialog: the command form (files, directories and
// worker pools variants) and its command-history picker. They use the harness of
// simple_dialog_keys_golden_test.go. The dialog opens on fixed fake entries (/srv/harbor/...) so previews never show a temp path; commands used with
// Enter are harmless (echo).

// runForEachFocusName names the focused widget of the run-for-each dialog: f0 (the command), cb0
// (run in each selected directory), cb1 (pseudo-TTY), pool0.. (No pool, then the pools), ok,
// cancel; list, ok, cancel in the history picker. It is the only code that knows how the state
// encodes focus.
func runForEachFocusName(st dialog.FileDialogState) string {
	f := st.Focus
	if st.RunForEachHistoryOpen {
		switch st.RunForEachHistoryPicker.Focus {
		case dlg.OK:
			return "ok"
		case dlg.Cancel:
			return "cancel"
		}
		return "list"
	}
	switch f {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	case dialog.RunForEachInDirs:
		return "cb0"
	case dialog.RunForEachPTY:
		return "cb1"
	case dialog.RunForEachNoPool:
		return "pool0"
	case "":
		return "f0"
	}
	for i := range st.RunForEachPools {
		if f == dialog.RunForEachPoolID(i) {
			return fmt.Sprintf("pool%d", i+1)
		}
	}
	if _, ok := dialog.FileFieldIndex(f); ok {
		return "f0"
	}
	return "?" + string(f)
}

func runForEachDialogState(app *App) string {
	st := app.model.FileDialog
	f := st.Fields[0]
	pk := st.RunForEachHistoryPicker
	hist := ""
	if st.RunForEachHistoryOpen {
		hist = fmt.Sprintf(" history[items=%d ranked=%v query=%q@%d sel=%d scroll=%d]", len(pk.Items), pk.Ranked, pk.Query, pk.QueryCursor, pk.Selected, pk.ListScroll)
	}
	return fmt.Sprintf("focus=%s cmd=%q@%d/%d pending=%v inDirs=%v pty=%v pool=%q err=%q preview=%q invalid=%v%s",
		runForEachFocusName(st), f.Value, f.Cursor, f.Scroll, f.PrefillPending, st.RunForEachInDirs, st.RunForEachPTY,
		st.RunForEachPool, st.RunForEachCommandError, st.RunForEachPreview, f.InputInvalid, hist)
}

// runForEachKeyDialog builds a key-golden dialog on the run-for-each dialog. dirs selects
// directories instead of files; pools configures two worker pools; history opens the history
// picker over a fixed command list; noEntries leaves the dialog without entries, so running it
// only reports "No paths to run" instead of starting a batch.
func runForEachKeyDialog(name string, dirs bool, pools []string, history, noEntries bool) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			harbor := filepath.Join(dir, "harbor")
			for _, d := range []string{"orchard", "meadow"} {
				if err := os.MkdirAll(filepath.Join(harbor, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range []string{"lantern.txt", "willow.txt"} {
				writeFile(t, filepath.Join(harbor, f))
			}
			app := newApp(t, newScreen(t, 80, 24), harbor)
			app.model.Message = ""
			app.model.UserHomeDir = dir
			app.dispatch(keymap.ActionFileRunForEach)
			d := &app.model.FileDialog
			if !d.Open {
				t.Fatal("run-for-each dialog should be open")
			}
			typ := localfs.EntryFile
			names := []string{"lantern.txt", "willow.txt"}
			if dirs {
				typ = localfs.EntryDirectory
				names = []string{"orchard", "meadow"}
			}
			d.RunForEachEntries = nil
			for _, n := range names {
				d.RunForEachEntries = append(d.RunForEachEntries, localfs.Entry{Name: n, Path: "/srv/harbor/" + n, Type: typ})
			}
			d.RunForEachDir = "/srv/harbor"
			if noEntries {
				d.RunForEachEntries = nil
			}
			d.RunForEachPools = pools
			app.commandsCtrl.RecomputeRunForEachValidation()
			if history {
				app.handleKey(goldenKeyEvent(t, "F3"))
				pk := &d.RunForEachHistoryPicker
				pk.Items = []string{"echo orchard %f", "gzip -9 %f", "tar -czf %f.tgz %f", "echo willow %f", "chmod 600 %f", "touch %f", "echo lantern %f",
					"sha256sum %f", "echo meadow %f", "rsync -a %f meadow/", "echo anchor %f", "ffmpeg -i %f out.mkv"}
				pk.Ranked, pk.MatchRanges = nil, nil
				for i := range pk.Items {
					pk.Ranked = append(pk.Ranked, i)
				}
			}
			return app
		},
		state: runForEachDialogState,
		open:  func(app *App) bool { return app.model.FileDialog.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("viewMode=%v message=%q dialog=%v", app.model.ViewMode, strings.TrimSpace(app.model.Message), app.model.FileDialog.Open)
		},
	}
}

func TestRunForEachKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, runForEachKeyDialog("run_for_each", false, nil, false, false), []simpleKeyScript{
		{"type-valid", "e c h o Space % f"},
		{"type-invalid", "a b c"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"space-in-command", "a Space b"},
		{"letters-in-command", "a t r o c y n O C R T"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"space-checkboxes", "Down Space Down Space Space"},
		{"enter-checkboxes", "Down Enter Down Enter Enter"},
		{"plain-letters-on-checkbox", "Down r t o c x Space"},
		{"plain-letters-on-buttons", "Down Down Down o c x"},
		{"alt-r", "e c h o Space % f Alt+r"},
		{"alt-r-twice", "Alt+r Alt+R"},
		{"alt-t", "Alt+t Alt+T"},
		{"alt-r-from-checkbox", "Down Alt+r Alt+t"},
		{"alt-r-from-button", "Down Down Down Alt+r Alt+t"},
		{"mnemonic-unknown", "a Alt+x Alt+q b"},
		{"enter-empty", "Enter"},
		{"enter-invalid", "a Enter"},
		{"enter-ok-invalid", "Down Down Down Enter"},
		{"enter-cancel", "Down Down Down Right Enter"},
		{"space-cancel", "Down Down Down Right Space"},
		{"alt-o-invalid", "Alt+o"},
		{"alt-c", "a Alt+c"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Down Down Right Esc"},
		{"restore-default", "x y z Alt+Backspace"},
		{"history-key-empty", "F3"},
		{"history-key-from-checkbox", "Down F3"},
		{"history-esc", "F3 Esc"},
		{"history-esc-cancel", "F3 Tab Tab Esc"},
		{"f-keys-ignored", "F2 F4 F7 a"},
		{"page-keys-ignored", "PgUp PgDn a"},
	})
	runKeyGoldenScripts(t, runForEachKeyDialog("run_for_each_dirs", true, nil, false, false), []simpleKeyScript{
		{"type-git", "g i t Space p u l l"},
		{"in-dirs-on", "g i t Space p u l l Alt+r"},
		{"in-dirs-toggle", "Alt+r Alt+r"},
		{"in-dirs-checkbox", "Down Space"},
		{"files-mode-dirs", "e c h o Space % f"},
	})
	runKeyGoldenScripts(t, runForEachKeyDialog("run_for_each_exec", false, nil, false, true), []simpleKeyScript{
		{"enter-in-command", "Alt+r g i t Space p u l l Enter"},
		{"enter-ok", "Alt+r g i t Space p u l l Down Down Down Enter"},
		{"space-ok", "Alt+r g i t Space p u l l Down Down Down Space"},
		{"alt-o", "Alt+r g i t Space p u l l Alt+o"},
		{"alt-enter", "Alt+r g i t Space p u l l Alt+Enter"},
		{"enter-ok-from-checkbox", "Alt+r g i t Space p u l l Down Down Enter"},
		{"enter-cancel", "Alt+r g i t Space p u l l Down Down Down Right Enter"},
		{"space-cancel", "Alt+r g i t Space p u l l Down Down Down Right Space"},
		{"enter-ok-invalid", "Down Down Down Enter"},
	})
	runKeyGoldenScripts(t, runForEachKeyDialog("run_for_each_pools", false, []string{"batch", "render"}, false, false), []simpleKeyScript{
		{"space-pool", "Down Down Down Down Space Down Space"},
		{"enter-pool", "Down Down Down Down Enter Down Enter Up Up Enter"},
		{"plain-letters-pool", "Down Down Down Down b r o c x"},
		{"alt-r-on-pool", "Down Down Down Down Alt+r Alt+t"},
		{"mnemonic-unknown-on-pool", "Down Down Down Down Alt+b"},
		{"enter-ok-from-pool", "Down Down Down Down Down Down Down Enter"},
		{"esc-from-pool", "Down Down Down Down Down Esc"},
		{"alt-c", "Down Down Down Down Down Alt+c"},
	})
	runKeyGoldenScripts(t, runForEachKeyDialog("run_for_each_history", false, nil, true, false), []simpleKeyScript{
		{"type-filter", "e c h o"},
		{"type-no-match", "z z z"},
		{"type-edit", "e c h o Backspace Backspace Left Home Delete End"},
		{"type-space", "e c Space"},
		{"down-walk", "Down Down Down"},
		{"down-to-end", "Down Down Down Down Down Down Down Down Down Down Down Down Down Down"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"page-keys", "PgDn PgDn PgUp PgUp PgUp"},
		{"ctrl-home-end", "Ctrl+End Ctrl+Home"},
		{"up-from-ok", "Down Down Tab Up"},
		{"up-from-cancel", "Down Down Tab Tab Up"},
		{"enter-selects", "Down Down Enter"},
		{"enter-selects-filtered", "e c h o Down Enter"},
		{"enter-ok", "Down Tab Enter"},
		{"enter-cancel", "Tab Tab Enter"},
		{"enter-no-match", "z z z Enter"},
		{"space-on-ok", "Down Tab Space"},
		{"space-on-cancel", "Tab Tab Space"},
		{"space-in-list", "Down Space"},
		{"plain-letters-on-buttons", "Down Tab o"},
		{"plain-c-on-buttons", "Down Tab c"},
		{"plain-other-on-buttons", "Tab x y"},
		{"alt-o", "Down Alt+o"},
		{"alt-c", "Down Alt+c"},
		{"alt-enter", "Down Alt+Enter"},
		{"esc", "Down Esc"},
		{"esc-from-button", "Tab Esc"},
		{"esc-then-main", "Down Esc Down"},
		{"enter-then-edit", "Down Enter x"},
		{"f-keys-ignored", "F2 F4 F7 F8 e"},
	})
}
