package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the dialogs. Each script is fed through the real App.handleKey path; the
// transcript holds a one-line semantic state after every key and, once the dialog closes, what
// the close did. How the dialog looks is the render goldens' job (internal/ui/dialog). Focus is
// reported by name; the *Name functions below are the only code that knows how each state
// encodes it.

const simpleKeyGoldenDir = "testdata/simple_dialog_keys"

// keyGoldenDialog describes one dialog under test.
type keyGoldenDialog struct {
	name string
	// setup returns the opened-dialog app; dir is a fresh temp dir.
	setup func(t *testing.T, dir string) *App
	// state is a one-line semantic summary; open reports whether the dialog is still open.
	state func(app *App) string
	open  func(app *App) bool
	// outcome describes what closing did.
	outcome func(app *App, quit bool) string
	// action runs a script token starting with "@": an injected asynchronous result (streamed
	// output, a finished run) rather than a key. nil = every token is a key.
	action func(t *testing.T, app *App, tok string)
	// settle, when set, runs after every key before the state is recorded: it drains the
	// asynchronous work a key starts (find's ranking) so the transcript does not depend on timing.
	settle func(t *testing.T, app *App)
	// serial keeps the scripts off t.Parallel, for setups that call t.Setenv (forbidden in
	// parallel tests) or share fixture state between scripts.
	serial bool
}

func runKeyGolden(t *testing.T, d keyGoldenDialog, keys []string) string {
	t.Helper()
	dir := t.TempDir()
	app := d.setup(t, dir)
	var sb strings.Builder
	record := func(label string) {
		app.render()
		fmt.Fprintf(&sb, "== %s ==\n%s\n", label, d.state(app))
	}
	record("open")
	for _, tok := range keys {
		if d.action != nil && strings.HasPrefix(tok, "@") {
			d.action(t, app, tok)
			record(tok)
			continue
		}
		quit, _ := app.handleKey(goldenKeyEvent(t, tok))
		if d.settle != nil && !quit && d.open(app) {
			d.settle(t, app)
		}
		if !d.open(app) || quit {
			fmt.Fprintf(&sb, "== %s ==\nCLOSED: %s\n", tok, d.outcome(app, quit))
			return sb.String()
		}
		record(tok)
	}
	return sb.String()
}

type simpleKeyScript struct {
	name string
	keys string // space-separated tokens
}

func runKeyGoldenScripts(t *testing.T, d keyGoldenDialog, scripts []simpleKeyScript) {
	t.Helper()
	for _, sc := range scripts {
		t.Run(sc.name, func(t *testing.T) {
			// Each script builds its own App on its own screen and temp dir. A yank reads the
			// process-wide lineedit kill buffer, which parallel scripts' kills would overwrite;
			// serial subtests finish before the parallel ones are released.
			if !d.serial && !slices.Contains(strings.Fields(sc.keys), "Ctrl+Y") {
				t.Parallel()
			}
			// Resolved before the script runs: a setup may change the working directory (t.Chdir).
			path, err := filepath.Abs(filepath.Join(simpleKeyGoldenDir, d.name, sc.name+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			got := runKeyGolden(t, d, strings.Fields(sc.keys))
			if *updateKeyGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if string(want) != got {
				t.Errorf("%s differs from golden %s\n--- got ---\n%s\n--- want ---\n%s", sc.name, path, got, want)
			}
		})
	}
}

// ---- undo confirm ----

func undoName(st dialog.UndoConfirmState) string { return string(st.Focus) }

var undoKeyDialog = keyGoldenDialog{
	name: "undo_confirm",
	setup: func(t *testing.T, dir string) *App {
		app := newApp(t, newScreen(t, 80, 24), dir)
		app.model.Message = ""
		app.model.UndoConfirm = dialog.UndoConfirmState{
			Open: true, Focus: dialog.UndoNo, JobID: "job-harbor",
			Line1: "Undo copy of 3 items to willow (5 steps)?",
			Line2: "Copied files are removed again.",
		}
		return app
	},
	state:   func(app *App) string { return "focus=" + undoName(app.model.UndoConfirm) },
	open:    func(app *App) bool { return app.model.UndoConfirm.Open },
	outcome: func(app *App, _ bool) string { return fmt.Sprintf("message=%q", app.model.Message) },
}

func TestUndoConfirmKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, undoKeyDialog, []simpleKeyScript{
		{"enter-no", "Enter"},
		{"enter-yes", "Left Enter"},
		{"space-no", "Space"},
		{"space-yes", "Left Space"},
		{"alt-y", "Alt+y"},
		{"alt-n", "Left Alt+n"},
		{"alt-upper", "Alt+Y"},
		{"plain-y", "y"},
		{"plain-n", "Left n"},
		{"plain-other", "x q Down"},
		{"esc", "Left Esc"},
		{"alt-o-on-no", "Alt+o"},
		{"alt-o-on-yes", "Left Alt+o"},
		{"alt-c", "Left Alt+c"},
		{"alt-enter-on-no", "Alt+Enter"},
		{"alt-enter-on-yes", "Left Alt+Enter"},
		{"alt-unknown", "Alt+x Alt+q Right"},
	})
}

// ---- quit confirm ----

func quitName(st dialog.QuitConfirmState) string { return string(st.Focus) }

var quitKeyDialog = keyGoldenDialog{
	name: "quit_confirm",
	setup: func(t *testing.T, dir string) *App {
		app := newApp(t, newScreen(t, 80, 24), dir)
		app.model.QuitConfirm = dialog.QuitConfirmState{Open: true, Focus: dialog.QuitStay}
		return app
	},
	state:   func(app *App) string { return "focus=" + quitName(app.model.QuitConfirm) },
	open:    func(app *App) bool { return app.model.QuitConfirm.Open },
	outcome: func(_ *App, quit bool) string { return fmt.Sprintf("quit=%v", quit) },
}

func TestQuitConfirmKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, quitKeyDialog, []simpleKeyScript{
		{"enter-stay", "Enter"},
		{"enter-quit", "Right Enter"},
		{"space-stay", "Space"},
		{"space-quit", "Right Space"},
		{"alt-s", "Right Alt+s"},
		{"alt-q", "Alt+q"},
		{"alt-upper-q", "Alt+Q"},
		{"alt-upper-s", "Right Alt+S"},
		{"plain-s", "Right s"},
		{"plain-q", "q"},
		{"plain-other", "x o Down"},
		{"esc", "Right Esc"},
		{"alt-o-swallowed", "Right Alt+o Down"},
		{"alt-c", "Right Alt+c"},
		{"alt-enter-swallowed", "Right Alt+Enter Alt+x"},
	})
}

// ---- listing format ----

func listingName(st dialog.ListingFormatDialogState) string {
	switch st.Focus {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	for i, r := range panel.ListFormatDialogRadios() {
		if st.Focus == dlg.ID("format:"+r.Label) {
			return fmt.Sprintf("radio%d", i)
		}
	}
	return "radio0" // an unset focus is the first widget
}

var listingKeyDialog = keyGoldenDialog{
	name: "listing_format",
	setup: func(t *testing.T, dir string) *App {
		app := newApp(t, newScreen(t, 80, 24), dir)
		app.model.ActivePanel = ui.PrimaryPanel
		app.model.Message = ""
		app.openListingFormatDialog()
		return app
	},
	state: func(app *App) string {
		st := app.model.ListingFormatDialog
		return fmt.Sprintf("focus=%s format=%v", listingName(st), st.ListFormat)
	},
	open: func(app *App) bool { return app.model.ListingFormatDialog.Open },
	outcome: func(app *App, _ bool) string {
		return fmt.Sprintf("message=%q panel format=%v", app.model.Message, app.model.Primary.ListFormat)
	},
}

func TestListingFormatKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, listingKeyDialog, []simpleKeyScript{
		{"space-radios", "Down Space Down Space Up Up Space"},
		{"enter-radios", "Down Down Enter Up Enter"},
		{"space-ok", "Down Space Tab Space"},
		{"enter-ok", "Down Space Tab Enter"},
		{"space-cancel", "Down Space Tab Right Space"},
		{"enter-cancel", "Down Space Tab Right Enter"},
		{"mnemonics", "Alt+p Alt+b Alt+m Alt+P Alt+B Alt+M"},
		{"mnemonics-from-button", "Tab Right Alt+b Down Alt+m"},
		{"mnemonic-unknown", "Alt+x Alt+q Down"},
		{"mnemonic-then-ok", "Alt+b Tab Enter"},
		{"plain-o", "Down Space o"},
		{"plain-c", "Down Space c"},
		{"plain-upper-o", "Down Space O"},
		{"plain-upper-c", "Down Space C"},
		{"plain-radio-letters", "m p b x"},
		{"esc", "Down Space Esc"},
		{"alt-o", "Down Space Alt+o"},
		{"alt-c", "Down Space Alt+c"},
		{"alt-enter", "Alt+b Alt+Enter"},
	})
}

// ---- compare filter ----

func compareFilterName(st dialog.CompareFilterDialogState) string {
	switch st.Focus {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	for i, it := range dialog.CompareFilterView(st).Groups[0].Columns[0] {
		if st.Focus == it.ID {
			return fmt.Sprintf("item%d", i)
		}
	}
	return "item0" // an unset focus is the first widget
}

func compareFilterKeyDialog(name string, open func(app *App)) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			app.model.ViewMode = ui.ViewCompare
			app.model.CompareView.Filter = comparepkg.FilterAll
			open(app)
			return app
		},
		state: func(app *App) string {
			st := app.model.CompareFilterDialog
			return fmt.Sprintf("focus=%s pending=%d", compareFilterName(st), st.Filter)
		},
		open: func(app *App) bool { return app.model.CompareFilterDialog.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("view filter=%d message=%q", app.model.CompareView.Filter, app.model.Message)
		},
	}
}

func TestCompareFilterKeyScripts(t *testing.T) {
	category := compareFilterKeyDialog("compare_filter_category", func(app *App) { app.compareCtrl.OpenFilterDialog() })
	runKeyGoldenScripts(t, category, []simpleKeyScript{
		{"space-items", "Space Down Space Down Down Space Up Space"},
		{"enter-items", "Enter Down Enter Down Enter"},
		{"space-on-buttons", "Tab Space Right Space Tab"},
		{"enter-ok-keeps-pending", "Down Space Tab Enter"},
		{"enter-cancel", "Down Space Tab Right Enter"},
		{"mnemonics", "Alt+e Alt+r Alt+l Alt+d Alt+A Alt+E Alt+R"},
		{"mnemonic-master", "Alt+a Alt+a Alt+l Alt+a"},
		{"mnemonic-unknown", "Alt+x Alt+q Down"},
		{"alt-o-is-ok", "Down Space Alt+o"},
		{"alt-c", "Down Space Alt+c"},
		{"alt-enter", "Down Space Alt+Enter"},
		{"esc", "Down Space Esc"},
		{"plain-letters", "a e x o c"},
	})
	selectDlg := compareFilterKeyDialog("compare_filter_select", func(app *App) { app.compareCtrl.OpenSelectDialog(true) })
	runKeyGoldenScripts(t, selectDlg, []simpleKeyScript{
		{"mnemonics", "Alt+a Alt+e Alt+r Alt+m Alt+n Alt+m"},
		{"space-and-ok", "Down Space Down Down Down Down Space Tab Enter"},
		{"alt-o-is-ok", "Alt+m Alt+o"},
		{"esc", "Alt+m Esc"},
	})
	unselect := compareFilterKeyDialog("compare_filter_unselect", func(app *App) {
		app.model.CompareView.FocusColumn = ui.CompareColumnSecondary
		app.compareCtrl.OpenSelectDialog(false)
	})
	runKeyGoldenScripts(t, unselect, []simpleKeyScript{
		{"mnemonics-and-ok", "Alt+n Alt+m Alt+Enter"},
		{"cancel", "Alt+n Alt+c"},
	})
}

// ---- host key ----

func hostKeyName(st dialog.HostKeyDialogState) string {
	if st.Focus == "" {
		return "accept" // an unset focus is the first widget
	}
	return string(st.Focus)
}

// hostKeyReply is the channel the dialog's decision lands on.
type hostKeyReply = chan sftpHostKeyReply

var hostKeyReplies = map[*App]hostKeyReply{}

var hostKeyKeyDialog = keyGoldenDialog{
	name:   "hostkey",
	serial: true, // setup writes the shared hostKeyReplies map
	setup: func(t *testing.T, dir string) *App {
		app := newApp(t, newScreen(t, 80, 24), dir)
		reply := make(hostKeyReply, 1)
		app.sftp.hostKeyWait = &sftpHostKeyWait{id: 1, reply: reply}
		hostKeyReplies[app] = reply
		t.Cleanup(func() { delete(hostKeyReplies, app) })
		app.openHostKeyDialog(sftpb.HostKeyPrompt{
			Host: "harbor.example.net:22", KeyType: "ssh-ed25519", Fingerprint: "tulip+meadow/harbor+lantern=willow",
		})
		return app
	},
	state:   func(app *App) string { return "focus=" + hostKeyName(app.model.HostKeyDialog) },
	open:    func(app *App) bool { return app.model.HostKeyDialog.Open },
	outcome: func(app *App, _ bool) string { return hostKeyDecision(hostKeyReplies[app]) },
}

func hostKeyDecision(reply hostKeyReply) string {
	select {
	case r := <-reply:
		return fmt.Sprintf("decision=%v err=%v", r.decision, r.err)
	default:
		return "no decision"
	}
}

func TestHostKeyKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, hostKeyKeyDialog, []simpleKeyScript{
		{"enter-accept", "Enter"},
		{"enter-save", "Right Enter"},
		{"enter-reject", "Right Right Enter"},
		{"space-accept", "Space"},
		{"space-reject", "Right Right Space"},
		{"alt-a", "Right Alt+a"},
		{"alt-s", "Alt+s"},
		{"alt-r", "Alt+R"},
		{"plain-a", "Right a"},
		{"plain-s", "s"},
		{"plain-r", "R"},
		{"plain-other", "x o c Down"},
		{"esc", "Right Esc"},
		{"alt-o-swallowed", "Right Alt+o Down"},
		{"alt-c", "Alt+c"},
		{"alt-enter-swallowed", "Alt+Enter Alt+x"},
	})
}

// ---- meta ----

func metaName(st dialog.MetaDialogState) string {
	switch st.Focus {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	for i := range st.Entries {
		if st.Focus == dialog.MetaEntryID(i) {
			return fmt.Sprintf("item%d", i)
		}
	}
	return "item0" // an unset focus is the first widget
}

const metaGoldenBody = `[[entry]]
name = "lantern"
description = "Shows the lantern count"
file = "echo meta"

[[entry]]
name = "orchard"
description = "Orchard yield"
file = "echo meta"

[[entry]]
name = "velvet"
description = "Velvet rating"
file = "echo meta"
`

var metaKeyDialog = keyGoldenDialog{
	name:   "meta",
	serial: true, // setup swaps the package-level externalEditorRunner
	setup: func(t *testing.T, dir string) *App {
		cfgDir := filepath.Join(dir, "config")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfgDir, config.DefaultMetaFileName), []byte(metaGoldenBody), 0o644); err != nil {
			t.Fatal(err)
		}
		app := testMetaDialogApp(t, dir, cfgDir)
		app.model.Message = ""
		prev := externalEditorRunner
		externalEditorRunner = func(context.Context, string, bool) error { return nil }
		t.Cleanup(func() { externalEditorRunner = prev })
		app.model.MetaDialog = dialog.MetaDialogState{
			Open: true, PanelID: ui.PrimaryPanel,
			Entries: []dialog.MetaEntry{
				{Name: "lantern", Description: "Shows the lantern count"},
				{Name: "orchard", Description: "Orchard yield"},
				{Name: "velvet", Description: "Velvet rating"},
			},
			Checked: []bool{false, true, false},
		}
		return app
	},
	state: func(app *App) string {
		st := app.model.MetaDialog
		return fmt.Sprintf("focus=%s checked=%v", metaName(st), st.Checked)
	},
	open: func(app *App) bool { return app.model.MetaDialog.Open },
	outcome: func(app *App, _ bool) string {
		var names []string
		for _, c := range app.model.MetaResults[ui.PrimaryPanel] {
			names = append(names, c.EntryName)
		}
		return fmt.Sprintf("meta columns=%v message=%q", names, app.model.Message)
	},
}

func TestMetaDialogKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, metaKeyDialog, []simpleKeyScript{
		{"space-toggles", "Space Down Space Down Space Space Up Space"},
		{"enter-toggles", "Enter Down Enter Down Enter"},
		{"space-ok", "Space Down Down Down Space"},
		{"enter-ok", "Space Down Down Down Enter"},
		{"space-cancel", "Space Down Down Down Right Space"},
		{"enter-cancel", "Space Down Down Down Right Enter"},
		{"mnemonics", "Alt+l Alt+r Alt+v Alt+L Alt+R"},
		{"mnemonic-from-button", "Down Down Down Alt+v Down Down Down Alt+l"},
		{"mnemonic-unknown", "Alt+x Alt+q Down"},
		{"plain-o", "Space o"},
		{"plain-c", "Space c"},
		{"plain-upper", "Space O"},
		{"plain-other", "l r v x"},
		{"esc", "Space Esc"},
		{"alt-o", "Space Alt+o"},
		{"alt-c", "Space Alt+c"},
		{"alt-enter", "Space Alt+Enter"},
		{"f9-on-item", "Down F9"},
		{"f9-on-ok", "Down Down Down F9"},
		{"f9-on-cancel", "Down Down Down Right F9"},
	})
}

// ---- stash restore ----

func stashName(st dialog.StashRestoreDialogState) string {
	if st.Focus == "" {
		return "replace" // an unset focus is the first widget
	}
	return string(st.Focus)
}

var stashKeyDialog = keyGoldenDialog{
	name: "stash_restore",
	setup: func(t *testing.T, dir string) *App {
		harbor, orchard := filepath.Join(dir, "harbor.txt"), filepath.Join(dir, "orchard.txt")
		for _, p := range []string{harbor, orchard} {
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		app := newApp(t, newScreen(t, 80, 24), dir)
		app.model.Message = ""
		p := app.activePanel()
		p.AddSelection(harbor)
		app.togglePanelSelectionStash()
		p.AddSelection(orchard)
		app.togglePanelSelectionStash()
		app.model.Message = ""
		return app
	},
	state: func(app *App) string { return "focus=" + stashName(app.model.StashRestoreDialog) },
	open:  func(app *App) bool { return app.model.StashRestoreDialog.Open },
	outcome: func(app *App, _ bool) string {
		p := app.activePanel()
		return fmt.Sprintf("live=%d stash=%d message=%q", p.SelectedPathCount(), p.StashPathCount(), app.model.Message)
	},
}

func TestStashRestoreKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, stashKeyDialog, []simpleKeyScript{
		{"enter-replace", "Enter"},
		{"enter-merge", "Right Enter"},
		{"enter-drop-stash", "Right Right Enter"},
		{"enter-drop-all", "Right Right Right Enter"},
		{"enter-cancel", "Right Right Right Right Enter"},
		{"space", "Right Space"},
		{"alt-r", "Right Alt+r"},
		{"alt-m", "Alt+m"},
		{"alt-d", "Alt+d"},
		{"alt-a", "Alt+A"},
		{"plain-letters", "r m d a c x"},
		{"esc", "Right Esc"},
		{"alt-o-replace", "Alt+o"},
		{"alt-o-drop-all", "Right Right Right Alt+o"},
		{"alt-o-cancel", "Right Right Right Right Alt+o"},
		{"alt-c", "Right Alt+c"},
		{"alt-enter", "Right Alt+Enter"},
		{"alt-unknown", "Alt+x Alt+q Right"},
	})
}

// ---- dedup return ----

func dedupReturnKeyDialog(name string, scope []string) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			app.model.DedupSnapshot.Phase = comparepkg.DedupDone
			app.model.DedupSnapshot.Root = pathloc.MustParse("/srv/meadow/orchard")
			app.model.DedupReturnDialog = dialog.DedupReturnDialogState{Open: true, Dir: "/srv/meadow/orchard", ScopeDirs: scope}
			t.Cleanup(app.dedupCtrl.Close)
			return app
		},
		state: func(app *App) string {
			if f := app.model.DedupReturnDialog.Focus; f != "" {
				return "focus=" + string(f)
			}
			return "focus=first" // an unset focus is the first widget
		},
		open: func(app *App) bool { return app.model.DedupReturnDialog.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("view=%v progress=%v message=%q", app.model.ViewMode, app.model.DedupProgressDialog.Open, app.model.Message)
		},
	}
}

func TestDedupReturnKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"enter-first", "Enter"},
		{"enter-second", "Right Enter"},
		{"enter-cancel", "Right Right Enter"},
		{"space", "Space Right Space"},
		{"alt-s", "Right Alt+s"},
		{"alt-r", "Alt+r"},
		{"alt-d", "Alt+d"},
		{"alt-upper", "Alt+S"},
		{"plain-letters", "s r d c x"},
		{"esc", "Right Esc"},
		{"alt-o-swallowed", "Right Alt+o Down"},
		{"alt-c", "Alt+c"},
		{"alt-enter-swallowed", "Alt+Enter Alt+x"},
	}
	runKeyGoldenScripts(t, dedupReturnKeyDialog("dedup_return_kept", nil), scripts)
	runKeyGoldenScripts(t, dedupReturnKeyDialog("dedup_return_scope", []string{"/nonexistent/meadow/willow"}), scripts)
}
