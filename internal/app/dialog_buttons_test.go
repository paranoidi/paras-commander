package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

var (
	keyAltO     = tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt)
	keyAltEnter = tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt)
	keyAltC     = tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt)
	keyEsc      = tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	keyF9       = tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone)
)

func TestCentralDialogOKCancelKeys(t *testing.T) {
	okKeys := map[string]*tcell.EventKey{"Alt+O": keyAltO, "Alt+Enter": keyAltEnter}
	cancelKeys := map[string]*tcell.EventKey{"Alt+C": keyAltC, "Esc": keyEsc, "F9": keyF9}
	dialogs := []struct {
		name   string
		open   func(*App)
		isOpen func(*App) bool
	}{
		{"message", func(a *App) { a.openMessageDialog("Otter", "lantern") },
			func(a *App) bool { return a.model.MessageDialog.Open }},
		{"help", func(a *App) { a.openHelpDialog() }, func(a *App) bool { return a.model.HelpView.Open }},
		{"listing format", func(a *App) { a.openListingFormatDialog() }, func(a *App) bool { return a.model.ListingFormatDialog.Open }},
		{"transfer", func(a *App) { a.dialogCtrl.OpenCopyDialog() }, func(a *App) bool { return a.model.TransferDialog.Open }},
		{"mkdir", func(a *App) { a.dispatch(keymap.ActionFileMkdir) }, func(a *App) bool { return a.model.FileDialog.Open }},
	}
	run := func(t *testing.T, name string, ev *tcell.EventKey, d int) {
		t.Run(dialogs[d].name+"/"+name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "heron.txt"))
			app := newApp(t, newScreen(t, 100, 30), dir)
			dialogs[d].open(app)
			if !dialogs[d].isOpen(app) {
				t.Fatal("dialog did not open")
			}
			app.handleKey(ev)
			if name == "Alt+O" || name == "Alt+Enter" {
				// OK may stay open (empty mkdir name, same-directory copy rename prompt); every other dialog closes.
				if dialogs[d].name == "mkdir" || dialogs[d].name == "transfer" {
					return
				}
			}
			if dialogs[d].isOpen(app) {
				t.Fatalf("dialog still open after %s", name)
			}
		})
	}
	for d := range dialogs {
		for name, ev := range okKeys {
			run(t, name, ev, d)
		}
		for name, ev := range cancelKeys {
			run(t, name, ev, d)
		}
	}
}

func TestCentralDialogRenameToolPhase(t *testing.T) {
	for name, ev := range map[string]*tcell.EventKey{"Esc": keyEsc, "Alt+C": keyAltC, "Alt+O": keyAltO, "Alt+Enter": keyAltEnter} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "a.b"))
			app := newApp(t, newScreen(t, 80, 20), dir)
			app.dispatch(keymap.ActionFileRename)
			app.handleKey(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
			if app.model.FileDialog.RenamePhase == dialog.RenamePhaseMain {
				t.Fatal("rename tool phase did not open")
			}
			app.handleKey(ev)
			if !app.model.FileDialog.Open || app.model.FileDialog.RenamePhase != dialog.RenamePhaseMain {
				t.Fatalf("%s must return to the main rename dialog, open=%v phase=%v", name, app.model.FileDialog.Open, app.model.FileDialog.RenamePhase)
			}
		})
	}
}

func TestCentralCancelCommandOutputEscKeepsRunning(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ev         *tcell.EventKey
		wantCancel bool
	}{{"Esc", keyEsc, false}, {"Alt+Enter", keyAltEnter, false}, {"Alt+C", keyAltC, true}, {"F9", keyF9, true}} {
		t.Run(tc.name, func(t *testing.T) {
			app := newApp(t, newScreen(t, 100, 30), t.TempDir())
			canceled := false
			app.commandsCtrl.OpenRunningOutputDialog("row1", " heron ", false, func() { canceled = true }, "", "")
			app.handleKey(tc.ev)
			if app.model.CommandOutputDialog.Open {
				t.Fatal("dialog still open")
			}
			if canceled != tc.wantCancel {
				t.Fatalf("run canceled = %v, want %v", canceled, tc.wantCancel)
			}
		})
	}
}

func TestCentralCancelBlockerEscPostpones(t *testing.T) {
	app := newApp(t, newScreen(t, 100, 30), t.TempDir())
	app.model.ConflictDialog = dialog.ConflictDialogState{Open: true, JobID: "otter"}
	app.handleKey(keyEsc)
	if app.model.ConflictDialog.Open {
		t.Fatal("Esc must postpone (close) the blocker dialog")
	}
}

func TestCentralCancelConfigResetConfirmClosesOnlyConfirm(t *testing.T) {
	app := newApp(t, newScreen(t, 100, 30), t.TempDir())
	app.openConfigDialog()
	app.model.ConfigDialog.ResetDefaultsConfirm = true
	app.handleKey(keyEsc)
	if !app.model.ConfigDialog.Open || app.model.ConfigDialog.ResetDefaultsConfirm {
		t.Fatalf("Esc must close only the confirm: open=%v confirm=%v", app.model.ConfigDialog.Open, app.model.ConfigDialog.ResetDefaultsConfirm)
	}
	app.handleKey(keyEsc)
	if app.model.ConfigDialog.Open {
		t.Fatal("second Esc must close the config dialog")
	}
}

func footerHints(app *App) string {
	var b strings.Builder
	for _, fk := range app.activeFooterKeys() {
		b.WriteString(fk.Hint + "|")
	}
	return b.String()
}

func TestBookmarkDialogOpenInPanelAndOK(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "harbor", "lantern")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte(fmt.Sprintf("lantern : %s\n", target)), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	screen := newScreen(t, 100, 30)
	app := newTestApp(t, screen, Options{CWD: func() (string, error) { return root, nil }, Config: cfg})

	app.dialogCtrl.OpenBookmarkDialog()
	if h := footerHints(app); !strings.Contains(h, "Open ◄") || !strings.Contains(h, "Open ►") {
		t.Fatalf("footer = %q, want both open-in-panel hints", h)
	}
	pk := &app.model.PathPicker
	target = filepath.Clean(pk.Items[pk.Ranked[pk.Selected]].Path)
	app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool { return app.model.Primary.PathString() == filepath.Clean(target) })
	if got := app.model.Primary.PathString(); got != filepath.Clean(target) {
		t.Fatalf("primary = %q, want %q", got, target)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool { return app.model.Secondary.PathString() == filepath.Clean(target) })
	if got := app.model.Secondary.PathString(); got != filepath.Clean(target) {
		t.Fatalf("secondary = %q, want %q", got, target)
	}
	if !app.model.PathPicker.Open {
		t.Fatal("picker must stay open after open-in-panel")
	}
	app.handleKey(keyAltEnter)
	if app.model.PathPicker.Open {
		t.Fatal("Alt+Enter is OK in the bookmarks picker")
	}
}

func TestHistoryDialogOpenInPanel(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "harbor")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, root)
	if err := app.navigatePanelToDirectory(ui.PrimaryPanel, sub, ""); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	app.openHistoryDialog(ui.PrimaryPanel)
	if !app.model.HistoryDialog.Open {
		t.Fatal("history dialog did not open")
	}
	if h := footerHints(app); !strings.Contains(h, "Open ◄") || !strings.Contains(h, "Open ►") {
		t.Fatalf("footer = %q, want both open-in-panel hints", h)
	}
	st := &app.model.HistoryDialog
	want := filepath.Clean(st.Paths[st.Ranked[st.Selected]])
	app.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool { return app.model.Secondary.PathString() == want })
	if got := app.model.Secondary.PathString(); got != want {
		t.Fatalf("secondary = %q, want %q", got, want)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool { return app.model.Primary.PathString() == want })
	if got := app.model.Primary.PathString(); got != want {
		t.Fatalf("primary = %q, want %q", got, want)
	}
	if !app.model.HistoryDialog.Open {
		t.Fatal("history dialog must stay open after open-in-panel")
	}
}
