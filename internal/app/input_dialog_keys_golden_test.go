package app

import (
	"fmt"
	"testing"

	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the dialogs that mix text inputs with options and buttons. They use the
// harness of simple_dialog_keys_golden_test.go: every script runs through App.handleKey, and the
// transcript records the dialog state after each key.

// ---- elevate ----

var elevateReply chan ops.PermissionAnswer

func elevateName(st dialog.ElevateDialogState) string {
	return elevateFocusName(st)
}

func elevateKeyDialog(name string, req ops.PermissionRequest) keyGoldenDialog {
	return keyGoldenDialog{
		name:   name,
		serial: true, // setup writes the package-level elevateReply
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			elevateReply = make(chan ops.PermissionAnswer, 1)
			app.elevate.wait = &elevateWait{id: 1, reply: elevateReply}
			app.applyElevateOpen(elevateOpenPayload{req: req})
			return app
		},
		state: func(app *App) string {
			st := app.model.ElevateDialog
			return fmt.Sprintf("focus=%s password=%q cursor=%d", elevateName(st), st.Password.Value, st.Password.Cursor)
		},
		open: func(app *App) bool { return app.model.ElevateDialog.Open },
		outcome: func(app *App, _ bool) string {
			select {
			case ans := <-elevateReply:
				return fmt.Sprintf("decision=%v password=%q", ans.Decision, ans.Password)
			default:
				return "no answer"
			}
		},
	}
}

func TestElevateKeyScripts(t *testing.T) {
	withPassword := elevateKeyDialog("elevate", ops.PermissionRequest{Op: "rename", Path: "/srv/harbor/orchard/meadow.txt"})
	runKeyGoldenScripts(t, withPassword, []simpleKeyScript{
		{"type-edit", "w i l l o w Left Left Backspace x Home Delete End 4 2"},
		{"type-space", "a Space b Space"},
		{"home-end", "a b c Home Home End Left Left Right"},
		{"edit-empty", "Backspace Delete Left Right Home End"},
		{"enter-on-input", "w i l Enter"},
		{"enter-elevate", "a Down Enter"},
		{"enter-skip", "a Down Right Enter"},
		{"enter-cancel", "a Down Right Right Enter"},
		{"space-elevate", "a Down Space"},
		{"space-skip", "a Down Right Space"},
		{"space-cancel", "a Down Right Right Space"},
		{"alt-e-in-input", "a b Alt+e"},
		{"alt-s-in-input", "a b Alt+s"},
		{"alt-upper-e", "a Alt+E"},
		{"alt-s-on-button", "a Down Right Right Alt+s"},
		{"alt-c-cancels", "a b Alt+c"},
		{"alt-o-elevates", "a b Alt+o"},
		{"alt-enter", "a Alt+Enter"},
		{"alt-unknown-in-input", "a Alt+x Alt+q b"},
		{"letters-in-input", "e s c o y n E S C O"},
		{"letters-on-buttons", "Down e s c o y n"},
		{"esc", "a b Esc"},
		{"esc-from-button", "a Down Right Esc"},
	})
	badPassword := elevateKeyDialog("elevate_badpw", ops.PermissionRequest{Op: "delete", Path: "/srv/harbor/orchard/meadow.txt", BadPassword: true})
	runKeyGoldenScripts(t, badPassword, []simpleKeyScript{
		{"type-and-nav", "a b Down Right Up Tab Backtab"},
		{"enter", "a Enter"},
	})
	noPassword := elevateKeyDialog("elevate_nopw", ops.PermissionRequest{Op: "", Path: "/srv/harbor/orchard/meadow.txt", NoPassword: true})
	runKeyGoldenScripts(t, noPassword, []simpleKeyScript{
		{"enter-elevate", "Enter"},
		{"enter-skip", "Right Enter"},
		{"enter-cancel", "Right Right Enter"},
		{"space", "Space Right Space"},
		{"alt-e", "Right Alt+e"},
		{"alt-s", "Alt+s"},
		{"alt-c", "Alt+c"},
		{"alt-o", "Right Alt+o"},
		{"letters", "e s c o x Space"},
		{"esc", "Right Esc"},
	})
}

func elevateFocusName(st dialog.ElevateDialogState) string {
	switch st.Focus {
	case dialog.ElevatePassword:
		return "input"
	case dialog.ElevateRetry:
		return "elevate"
	case dialog.ElevateSkip:
		return "skip"
	case dlg.Cancel:
		return "cancel"
	}
	if st.HasInput() { // an unset focus is the first focusable widget
		return "input"
	}
	return "elevate"
}

// ---- debounce calibrate ----

func debounceName(st dialog.DebounceCalibrateDialogState) string {
	switch st.Focus {
	case dialog.DebouncePreview:
		return "in1"
	case dlg.OK:
		return "ok"
	case dialog.DebounceCalibrate:
		return "calibrate"
	case dlg.Cancel:
		return "cancel"
	}
	return "in0" // an unset focus is the first widget
}

var debounceKeyDialog = keyGoldenDialog{
	name: "debounce_calibrate",
	setup: func(t *testing.T, dir string) *App {
		app := newApp(t, newScreen(t, 80, 24), dir)
		app.model.Message = ""
		app.openDebounceCalibrateDialog()
		return app
	},
	state: func(app *App) string {
		st := app.model.DebounceCalibrateDialog
		return fmt.Sprintf("phase=%d focus=%s value=%q cursor=%d preview=%q pcursor=%d status=%q",
			st.Phase, debounceName(st), st.Value, st.Cursor, st.PreviewValue, st.PreviewCursor, st.Status)
	},
	open: func(app *App) bool { return app.model.DebounceCalibrateDialog.Open },
	outcome: func(app *App, _ bool) string {
		return fmt.Sprintf("debounce=%d preview=%d message=%q", app.config.UI.KeyRepeatDebounceMS, app.config.UI.MediaPreviewDebounceMS, app.model.Message)
	},
}

func TestDebounceCalibrateKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, debounceKeyDialog, []simpleKeyScript{
		{"type-digits", "1 2 3 Left Backspace Delete Home End 9"},
		{"non-digits-swallowed", "a b Space - 5"},
		{"edit-second", "Down 7 8 Left Backspace 1 End"},
		{"edit-empty", "Backspace Backspace Backspace Delete Left Right Home End"},
		{"home-end", "Home Home 1 End End 2 Left Left Left Left"},
		{"left-right-inputs", "Left Left Right Right Right Down Left Right"},
		{"enter-input-applies", "Enter"},
		{"enter-second-input-applies", "Down Enter"},
		{"enter-ok", "Down Down Enter"},
		{"enter-calibrate", "Down Down Right Enter"},
		{"enter-cancel", "Down Down Right Right Enter"},
		{"invalid-first", "Backspace Backspace Enter 5 Down"},
		{"invalid-second", "Down Backspace Backspace Backspace Enter Up 1"},
		{"status-cleared-by-nav", "Backspace Backspace Enter Down Up"},
		{"space-on-buttons", "Down Down Space Right Space Right Space"},
		{"letters-on-buttons", "Down Down o c l O C L x"},
		{"alt-l", "Alt+l"},
		{"alt-l-on-button", "Down Down Alt+L"},
		{"alt-l-then-esc", "Alt+l Esc"},
		{"alt-mnemonic-in-input", "5 Alt+x Alt+q Alt+a 6"},
		{"alt-o", "9 Alt+o"},
		{"alt-c", "9 Alt+c"},
		{"alt-enter", "9 Alt+Enter"},
		{"esc", "9 Esc"},
		{"esc-from-button", "Down Down Right Esc"},
		{"alt-o-invalid", "Backspace Backspace Alt+o"},
	})
}

// ---- flatten ----

func flattenName(st dialog.FlattenDialogState) string {
	switch st.Focus {
	case dialog.FlattenRecursive:
		return "recursive"
	case dialog.FlattenRemoveEmpty:
		return "remove-empty"
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "dest" // an unset focus is the first widget
}

// flattenKeyDialog opens the real flatten dialog, then swaps in a fixed destination (the temp
// dir path would change every run). dropdown preloads three completion candidates.
func flattenKeyDialog(name string, dropdown bool) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, _ string) *App {
			app, _, _ := flattenDialogTestSetup(t)
			app.model.Message = ""
			dest := dialogctrl.TransferPrefilledDestination("/srv/thicket-lantern")
			dest.CompletionDirsOnly = true
			app.model.FlattenDialog.Destination = dest
			if dropdown {
				c := &app.model.FlattenDialog.Destination.Completion
				c.Set("/srv/thicket-lantern/o", 21, []pathpick.Candidate{{Name: "orchard", IsDir: true}, {Name: "otter", IsDir: true}, {Name: "oak", IsDir: true}})
				c.Open = true
			}
			return app
		},
		state: func(app *App) string {
			st := app.model.FlattenDialog
			c := st.Destination.Completion
			return fmt.Sprintf("focus=%s dest=%q cursor=%d pending=%v recursive=%v removeEmpty=%v dropdown=%v selected=%d",
				flattenName(st), st.Destination.Value, st.Destination.Cursor, st.Destination.PrefillPending, st.Recursive, st.RemoveEmpty, c.Open, c.Selected)
		},
		open: func(app *App) bool { return app.model.FlattenDialog.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("message=%q", app.model.Message)
		},
	}
}

func TestFlattenKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, flattenKeyDialog("flatten", false), []simpleKeyScript{
		{"replace-prefill", "a b c"},
		{"commit-prefill-backspace", "Backspace Backspace x"},
		{"commit-prefill-left", "Left x"},
		{"commit-prefill-home", "Home x"},
		{"commit-prefill-end", "End x"},
		{"commit-prefill-delete", "Home Delete Delete"},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"space-in-dest", "a Space b"},
		{"letters-in-dest", "r e o c y n O C Y N"},
		{"space-checkboxes", "Down Space Down Space Space"},
		{"enter-checkboxes", "Down Enter Down Enter Enter"},
		{"mnemonics-in-dest", "a Alt+r Alt+e Alt+R Alt+E b"},
		{"mnemonics-on-checkbox", "Down Alt+e Alt+r"},
		{"mnemonics-on-button", "Tab Tab Alt+r Alt+e Right Alt+r"},
		{"plain-letters-on-checkbox", "Down r e Space"},
		{"plain-letters-on-buttons", "Tab Tab o c r e"},
		{"mnemonic-unknown", "a Alt+x Alt+q b"},
		{"enter-dest-queues", "Enter"},
		{"enter-dest-typed", "x y Enter"},
		{"enter-dest-empty", "a Backspace Enter"},
		{"enter-ok", "Tab Tab Enter"},
		{"enter-cancel", "Tab Tab Right Enter"},
		{"space-ok", "Tab Tab Space"},
		{"space-cancel", "Tab Tab Right Space"},
		{"alt-o", "Alt+o"},
		{"alt-o-from-checkbox", "Down Alt+o"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", "a Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-button", "Tab Tab Right Esc"},
		{"toggles-then-ok", "Alt+r Alt+e Tab Tab Enter"},
	})
	runKeyGoldenScripts(t, flattenKeyDialog("flatten_dropdown", true), []simpleKeyScript{
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
		{"typing-keeps-dropdown", "x"},
		{"alt-c-closes-dropdown", "Alt+c"},
		{"alt-c-twice", "Alt+c Alt+c"},
		{"alt-o-confirms", "Alt+o"},
		{"left-edits-cursor", "Left Left x"},
		{"mnemonics", "Alt+r Alt+e"},
	})
}

// ---- compare merge ----

func compareMergeName(st dialog.CompareMergeDialogState) string {
	switch st.Focus {
	case dialog.CompareMergeRight:
		return "right"
	case dialog.CompareMergeMissing:
		return "missing"
	case dialog.CompareMergeModified:
		return "modified"
	case dialog.CompareMergeCopy:
		return "copy"
	case dialog.CompareMergeMove:
		return "move"
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "left" // an unset focus is the first widget
}

func compareMergeKeyDialog(name string, primary, secondary string) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			app.model.UserHomeDir = "/home/alice"
			app.model.ViewMode = ui.ViewCompare
			app.model.CompareView.Filter = comparepkg.FilterAll
			app.model.CompareSnapshot.PrimaryRoot = pathloc.MustParse(primary)
			app.model.CompareSnapshot.SecondaryRoot = pathloc.MustParse(secondary)
			app.model.CompareSnapshot.Rows = []comparepkg.Row{
				{Kind: comparepkg.KindPrimaryOnly, PrimaryRel: "lantern.txt", Size: 1 << 20},
				{Kind: comparepkg.KindSecondaryOnly, SecondaryRel: "anchor.txt", Size: 2 << 20},
				{Kind: comparepkg.KindContentDiff, PrimaryRel: "marble.txt", SecondaryRel: "marble.txt", Size: 3 << 20},
			}
			app.compareCtrl.OpenMergeDialog()
			return app
		},
		state: func(app *App) string {
			st := app.model.CompareMergeDialog
			return fmt.Sprintf("focus=%s direction=%v missing=%v modified=%v move=%v preview=%q message=%q",
				compareMergeName(st), st.Direction, st.CopyMissing, st.CopyModified, st.MoveMode, st.PreviewText, app.model.Message)
		},
		open: func(app *App) bool { return app.model.CompareMergeDialog.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("message=%q", app.model.Message)
		},
	}
}

func TestCompareMergeKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"space-all-rows", "Space Down Space Down Space Down Space Down Space Down Space Down Space Down Space"},
		{"enter-items", "Enter Down Enter Down Enter Down Enter Down Enter Down Enter"},
		{"enter-ok", "Tab Tab Tab Enter"},
		{"enter-cancel", "Tab Tab Tab Right Enter"},
		{"space-ok", "Tab Tab Tab Space"},
		{"space-cancel", "Tab Tab Tab Right Space"},
		{"mnemonics", "Alt+l Alt+r Alt+m Alt+f Alt+k Alt+d Alt+L Alt+M"},
		{"mnemonics-from-items", "Down Down Alt+d Alt+l Alt+m"},
		{"mnemonics-from-buttons", "Tab Tab Tab Alt+m Alt+d Right Alt+r"},
		{"plain-letters", "l r m f k d x"},
		{"plain-letters-on-buttons", "Tab Tab Tab o c l r"},
		{"mnemonic-unknown", "Alt+x Alt+q Down"},
		{"toggles-then-ok", "Alt+f Alt+d Alt+r Tab Tab Tab Enter"},
		{"alt-o", "Alt+m Alt+o"},
		{"alt-c", "Alt+m Alt+c"},
		{"alt-enter", "Alt+m Alt+Enter"},
		{"esc", "Alt+m Esc"},
		{"esc-from-button", "Tab Tab Tab Right Esc"},
	}
	runKeyGoldenScripts(t, compareMergeKeyDialog("compare_merge", "/home/alice/projects/harbor/orchard", "/home/alice/projects/harbor/willow/meadow"), scripts)
	runKeyGoldenScripts(t, compareMergeKeyDialog("compare_merge_plain", "/srv/orchard", "/mnt/willow"), []simpleKeyScript{
		{"toggle-and-ok", "Space Down Space Tab Tab Enter"},
	})
}
