package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the copy / move destination dialog: single location, the self-copy rename
// phase and the multi-location preview. The dump is the dialog's own rect (found on the screen
// after every key), so a change of the dialog height or position does not drag panel cells into
// the transcript. Paths shown in the dialog are fixed (or "~/" relative to a temp home); steps
// that would show a temp path run silently ("!" tokens) and a destination that really queues a
// job is swapped in by "!@tmpdest", so no job ever leaves the temp dir.

// dialogFrameRect finds the on-screen rect of the open dialog: the bounding box of the frame
// corners drawn in the dialog surface.
func dialogFrameRect(app *App) (x, y, w, h int) {
	screen := app.screen
	sw, sh := screen.Size()
	_, surface, _ := app.styles.DialogSurface.Decompose()
	x0, y0, x1, y1 := -1, -1, -1, -1
	for row := 0; row < sh; row++ {
		for col := 0; col < sw; col++ {
			ch, st, _ := screen.Get(col, row)
			_, bg, _ := st.Decompose()
			if bg != surface {
				continue
			}
			switch ch {
			case "┌":
				if x0 < 0 {
					x0, y0 = col, row
				}
			case "┘":
				x1, y1 = col, row
			}
		}
	}
	if x0 < 0 || x1 < 0 {
		return 0, 0, 1, 1
	}
	return x0, y0, x1 - x0 + 1, y1 - y0 + 1
}

func transferName(st dialog.TransferDialogState) string {
	switch st.Focus {
	case dialog.TransferNewName, dialog.TransferDest:
		return "dest"
	case dialog.TransferPerm:
		return "perm"
	case dialog.TransferTimes:
		return "ts"
	case dialog.TransferDeref:
		return "deref"
	case dialog.TransferFlatten:
		return "flat"
	case dlg.OK:
		return "ok"
	case dialog.TransferAddPaused:
		return "paused"
	case dlg.Cancel:
		return "cancel"
	}
	return "dest" // an unset focus is the first widget
}

type transferKeyOpts struct {
	move     bool
	multi    bool // multi-location selection (six files in two directories)
	self     bool // opened on the self-copy rename step
	remote   bool // source panel is remote: dereference is greyed out
	dropdown bool // three completion candidates preloaded
}

func transferKeyDialog(name string, o transferKeyOpts) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		rect: dialogFrameRect,
		setup: func(t *testing.T, dir string) *App {
			root := filepath.Join(dir, "projects")
			harbor, orchard := filepath.Join(root, "harbor"), filepath.Join(root, "orchard")
			for _, d := range []string{harbor, orchard, filepath.Join(root, "dst")} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range []string{"lantern.txt", "willow.txt", "anchor.txt"} {
				writeFile(t, filepath.Join(harbor, f))
			}
			for _, f := range []string{"meadow.txt", "timber.txt", "velvet.txt"} {
				writeFile(t, filepath.Join(orchard, f))
			}
			app := newApp(t, newScreen(t, 80, 24), harbor)
			app.model.Message = ""
			app.model.UserHomeDir = dir
			p := app.activePanel()
			switch {
			case o.multi:
				for _, f := range []string{"harbor/lantern.txt", "harbor/willow.txt", "harbor/anchor.txt", "orchard/meadow.txt", "orchard/timber.txt", "orchard/velvet.txt"} {
					p.AddSelection(filepath.Join(root, f))
				}
			default:
				selectPanelEntryByName(t, p, "lantern.txt")
			}
			kind := dialog.TransferKindCopy
			if o.move {
				kind = dialog.TransferKindMove
			}
			if o.self {
				app.dialogCtrl.OpenTransferDialogSelfCopyRename(kind, harbor, filepath.Join(harbor, "lantern.txt"))
			} else if o.move {
				app.dialogCtrl.ActivateMoveAction()
			} else {
				app.dialogCtrl.ActivateCopyAction()
			}
			d := &app.model.TransferDialog
			if !d.Open {
				t.Fatal("transfer dialog should be open")
			}
			if !o.self {
				dirsOnly := d.Destination.CompletionDirsOnly
				d.Destination = dialogctrl.TransferPrefilledDestination("/srv/thicket-lantern")
				d.Destination.CompletionDirsOnly = dirsOnly
			}
			d.SourceIsRemote = o.remote
			if o.dropdown {
				c := &d.Destination.Completion
				c.Set("/srv/thicket-lantern/o", 21, []pathpick.Candidate{{Name: "orchard", IsDir: true}, {Name: "otter", IsDir: true}, {Name: "oak", IsDir: true}})
				c.Open = true
			}
			return app
		},
		state: func(app *App) string {
			st := app.model.TransferDialog
			mask := func(s string) string { return strings.ReplaceAll(s, app.model.UserHomeDir, "<tmp>") }
			c := st.Destination.Completion
			// A destination holding the temp path has a run-dependent length: report the cursor
			// from the end and no scroll offset.
			cursor, scroll := fmt.Sprint(st.Destination.Cursor), fmt.Sprint(st.Destination.Scroll)
			if strings.Contains(st.Destination.Value, app.model.UserHomeDir) {
				cursor, scroll = fmt.Sprintf("end-%d", len([]rune(st.Destination.Value))-st.Destination.Cursor), "-"
			}
			return fmt.Sprintf("phase=%d focus=%s dest=%q cursor=%s scroll=%s pending=%v name=%q ncursor=%d perm=%v ts=%v deref=%v flat=%v listScroll=%d dropdown=%v selected=%d message=%q",
				st.Phase, transferName(st), mask(st.Destination.Value), cursor, scroll, st.Destination.PrefillPending,
				st.SelfCopyNewName.Value, st.SelfCopyNewName.Cursor, st.PreservePermissions, st.PreserveTimestamps, st.DereferenceSymlinks,
				st.FlattenIntoDest, st.EntriesScroll, c.Open, c.Selected, mask(app.model.Message))
		},
		open: func(app *App) bool { return app.model.TransferDialog.Open },
		outcome: func(app *App, _ bool) string {
			mask := func(s string) string { return strings.ReplaceAll(s, app.model.UserHomeDir, "<tmp>") }
			var parts []string
			for _, j := range app.jobState.AllJobs() {
				parts = append(parts, fmt.Sprintf("%s paused=%v sources=%d dest=%s", j.Type, j.PausedAfterScan, len(j.Sources), mask(j.Destination.String())))
			}
			return fmt.Sprintf("message=%q jobs=[%s] selected=%d", mask(app.model.Message), strings.Join(parts, "; "), len(app.activePanel().SelectedPaths))
		},
		action: func(t *testing.T, app *App, tok string) {
			if tok != "@tmpdest" {
				t.Fatalf("bad action %q", tok)
			}
			d := &app.model.TransferDialog
			dirsOnly := d.Destination.CompletionDirsOnly
			d.Destination = dialogctrl.TransferPrefilledDestination(filepath.Join(app.model.UserHomeDir, "projects", "dst"))
			d.Destination.CompletionDirsOnly = dirsOnly
			d.Destination.PrefillPending = false
		},
	}
}

func TestTransferCopyKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, transferKeyDialog("transfer_copy", transferKeyOpts{}), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"commit-prefill-right", "Right Right x"},
		{"commit-prefill-home", "Home x"},
		{"commit-prefill-end", "End x"},
		{"commit-prefill-delete", "Home Delete Delete"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"restore-default", "x y z Alt+Backspace"},
		{"space-in-dest", "a Space b"},
		{"letters-in-dest", "r t d i o c p y n O C P"},
		{"complete-ghost", "~ / p r o j e c t s / o"},
		{"complete-ghost-tab", "~ / p r o j e c t s / o Tab"},
		{"complete-open-tab", "~ / p r o j e c t s / Tab"},
		{"complete-open-down-enter", "~ / p r o j e c t s / Tab Down Enter"},
		{"complete-open-esc", "~ / p r o j e c t s / Tab Esc"},
		{"complete-open-esc-esc", "~ / p r o j e c t s / Tab Esc Esc"},
		{"down-walk", "Down Down Down Down Down Down Down"},
		{"up-walk", "Down Down Down Down Up Up Up Up Up Up"},
		{"tab", "Tab Tab Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab Backtab Backtab"},
		{"tab-from-checkbox", "Down Down Tab Backtab Backtab"},
		{"left-right-buttons", "Tab Tab Left Right Right Right Left Left Left"},
		{"left-right-checkbox", "Down Left Right Down Left Right"},
		{"space-checkboxes", "Down Space Down Space Down Space"},
		{"enter-checkboxes", "Down Enter Down Enter Down Enter"},
		{"mnemonics-in-dest", "a Alt+r Alt+t Alt+d Alt+R b"},
		{"mnemonics-on-checkbox", "Down Down Alt+t Alt+r"},
		{"mnemonics-on-button", "Tab Tab Alt+r Right Alt+t Alt+d"},
		{"plain-letters-on-checkbox", "Down r t d Space"},
		{"plain-letters-on-buttons", "Tab Tab o c p x r"},
		{"mnemonic-unknown", "a Alt+x Alt+q b"},
		{"dest-shortcut-active", "a b !Shift+Left"},
		{"dest-shortcut-inactive", "a b !Shift+Right"},
		{"dest-shortcut-off-dest", "Down Shift+Left Shift+Right"},
		{"enter-dest-queues", "!@tmpdest Enter"},
		{"enter-dest-empty", "a Backspace Enter"},
		{"enter-dest-whitespace", "Space Space Enter"},
		{"enter-ok", "!@tmpdest Tab Tab Enter"},
		{"enter-paused", "!@tmpdest Tab Tab Right Enter"},
		{"enter-cancel", "Tab Tab Right Right Enter"},
		{"space-ok", "!@tmpdest Tab Tab Space"},
		{"space-paused", "!@tmpdest Tab Tab Right Space"},
		{"space-cancel", "Tab Tab Right Right Space"},
		{"alt-o", "!@tmpdest Alt+o"},
		{"alt-o-from-checkbox", "!@tmpdest Down Alt+o"},
		{"alt-p", "!@tmpdest Alt+p"},
		{"alt-p-upper", "!@tmpdest Alt+P"},
		{"alt-p-from-checkbox", "!@tmpdest Down Alt+p"},
		{"alt-p-empty", "a Backspace Alt+p"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "!@tmpdest Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Tab Tab Right Esc"},
		{"toggles-then-ok", "!@tmpdest Alt+r Alt+t Tab Tab Enter"},
		{"pgup-pgdn-ignored", "PgUp PgDn Down PgDn PgUp"},
	})
	runKeyGoldenScripts(t, transferKeyDialog("transfer_copy_remote", transferKeyOpts{remote: true}), []simpleKeyScript{
		{"deref-inert", "Down Down Down Space Enter Alt+d"},
		{"walk", "Down Down Down Down Down Up Up"},
		{"mnemonic-in-dest", "a Alt+d Alt+r"},
	})
	runKeyGoldenScripts(t, transferKeyDialog("transfer_copy_dropdown", transferKeyOpts{dropdown: true}), []simpleKeyScript{
		{"enter-accepts-first", "Enter"},
		{"down-enter", "Down Enter"},
		{"down-down-enter", "Down Down Enter"},
		{"down-past-end", "Down Down Down Down Enter"},
		{"up-clamped", "Up Up Enter"},
		{"tab-cycles", "Tab Tab Tab Tab"},
		{"tab-then-enter", "Tab Tab Enter"},
		{"esc-closes-dropdown", "Esc"},
		{"esc-twice-closes-dialog", "Esc Esc"},
		{"esc-then-down", "Esc Down"},
		{"typing-resyncs", "x"},
		{"alt-c-closes-dropdown", "Alt+c"},
		{"alt-c-twice", "Alt+c Alt+c"},
		{"alt-o-confirms-invalid", "!@tmpdest Alt+o"},
		{"left-edits-cursor", "Left Left x"},
		{"mnemonics", "Alt+r Alt+t"},
	})
}

func TestTransferMoveKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, transferKeyDialog("transfer_move", transferKeyOpts{move: true}), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"complete-open-tab", "~ / p r o j e c t s / Tab"},
		{"down-walk", "Down Down Down Down Down"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"tab", "Tab Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab"},
		{"left-right-buttons", "Down Left Right Right Right Left Left Left"},
		{"mnemonics-in-dest", "a Alt+r Alt+t Alt+d Alt+i b"},
		{"plain-letters-on-buttons", "Down o c p x"},
		{"dest-shortcut-active", "a !Shift+Left"},
		{"dest-shortcut-inactive", "a !Shift+Right"},
		{"enter-dest-queues", "!@tmpdest Enter"},
		{"enter-dest-empty", "a Backspace Enter"},
		{"enter-ok", "!@tmpdest Down Enter"},
		{"enter-paused", "!@tmpdest Down Right Enter"},
		{"enter-cancel", "Down Right Right Enter"},
		{"space-ok", "!@tmpdest Down Space"},
		{"space-cancel", "Down Right Right Space"},
		{"alt-o", "!@tmpdest Alt+o"},
		{"alt-p", "!@tmpdest Alt+p"},
		{"alt-c", "Alt+c"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
	})
}

func TestTransferSelfCopyKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, transferKeyDialog("transfer_self_copy", transferKeyOpts{self: true}), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"commit-prefill-home", "Home x"},
		{"commit-prefill-end", "End x"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d"},
		{"restore-default", "x y z Alt+Backspace"},
		{"space-in-name", "a Space b"},
		{"letters-in-name", "r t d i o c p y n"},
		{"down-walk", "Down Down Down Down"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"tab", "Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab"},
		{"left-right-buttons", "Down Left Right Right Right Left Left Left"},
		{"mnemonics-in-name", "a Alt+r Alt+t Alt+d Alt+i b"},
		{"plain-letters-on-buttons", "Down o c p x"},
		{"dest-shortcuts-ignored", "a Shift+Left Shift+Right"},
		{"enter-same-name", "Enter"},
		{"enter-empty", "a Backspace Enter"},
		{"enter-bad-name", "a / b Enter"},
		{"enter-new-name", "e m b e r . t x t Enter"},
		{"enter-ok", "e m b e r . t x t Down Enter"},
		{"enter-paused", "e m b e r . t x t Down Right Enter"},
		{"enter-cancel", "Down Right Right Enter"},
		{"space-ok", "p e b b l e . t x t Down Space"},
		{"space-paused", "p e b b l e . t x t Down Right Space"},
		{"space-cancel", "Down Right Right Space"},
		{"alt-o", "q u a r t z . t x t Alt+o"},
		{"alt-o-same-name", "Alt+o"},
		{"alt-p", "q u a r t z . t x t Alt+p"},
		{"alt-p-same-name", "Alt+p"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "q u a r t z . t x t Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Down Right Esc"},
	})
	runKeyGoldenScripts(t, transferKeyDialog("transfer_self_move", transferKeyOpts{self: true, move: true}), []simpleKeyScript{
		{"walk", "Down Right Left Up Tab Tab Tab"},
		{"enter-new-name", "j u n i p e r . t x t Enter"},
		{"alt-p", "j u n i p e r . t x t Alt+p"},
		{"esc", "Esc"},
	})
}

func TestTransferMultiLocationKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, transferKeyDialog("transfer_multi_copy", transferKeyOpts{multi: true}), []simpleKeyScript{
		{"down-walk", "Down Down Down Down Down Down Down Down"},
		{"up-walk", "Down Down Down Down Down Up Up Up Up Up Up Up"},
		{"tab", "Tab Tab Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab Backtab Backtab"},
		{"left-right-buttons", "Down Down Down Down Down Left Right Right Right Left Left Left"},
		{"space-checkboxes", "Down Space Down Space Down Space Down Space"},
		{"enter-checkboxes", "Down Enter Down Enter Down Enter Down Enter"},
		{"mnemonics-in-dest", "a Alt+r Alt+t Alt+d Alt+i b"},
		{"mnemonics-on-flat", "Down Down Down Down Alt+i Alt+r"},
		{"mnemonics-on-button", "Tab Tab Alt+i Right Alt+r"},
		{"scroll-pgdn", "PgDn PgDn PgDn PgUp"},
		{"scroll-from-checkbox", "Down PgDn PgDn PgUp PgUp PgUp"},
		{"scroll-from-button", "Tab Tab PgDn PgUp"},
		{"scroll-then-flatten", "PgDn Alt+i"},
		{"dest-shortcut-active", "a !Shift+Left"},
		{"dest-shortcut-inactive", "a !Shift+Right"},
		{"enter-dest-queues", "!@tmpdest Enter"},
		{"enter-flatten-queues", "!@tmpdest Alt+i Enter"},
		{"enter-ok", "!@tmpdest Tab Tab Enter"},
		{"enter-paused", "!@tmpdest Tab Tab Right Enter"},
		{"enter-cancel", "Tab Tab Right Right Enter"},
		{"space-ok", "!@tmpdest Tab Tab Space"},
		{"alt-o", "!@tmpdest Alt+o"},
		{"alt-p", "!@tmpdest Alt+p"},
		{"alt-c", "Alt+c"},
		{"esc", "Down Esc"},
		{"complete-open-tab", "~ / p r o j e c t s / Tab"},
	})
	runKeyGoldenScripts(t, transferKeyDialog("transfer_multi_move", transferKeyOpts{multi: true, move: true}), []simpleKeyScript{
		{"down-walk", "Down Down Down Down Down Down"},
		{"up-walk", "Down Down Down Down Up Up Up Up"},
		{"tab", "Tab Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab Backtab"},
		{"space-flatten", "Down Space Space"},
		{"mnemonics", "a Alt+i Alt+r Alt+d b"},
		{"scroll", "PgDn PgUp"},
		{"enter-queues", "!@tmpdest Alt+i Enter"},
		{"enter-paused", "!@tmpdest Down Down Right Enter"},
		{"alt-p", "!@tmpdest Alt+p"},
		{"alt-c", "Alt+c"},
		{"esc", "Esc"},
	})
}
