package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the mass rename dialog: the main screen in all four modes, the
// save-pattern prompt and the load / history / overwrite pickers. They use the harness of
// simple_dialog_keys_golden_test.go. The panel holds
// fixed file names; the patterns file lives in the temp dir; the external editor is a fake that
// prefixes every line (installed once per test function, since the runner is a package-level
// variable and the fake is stateless, so the scripts run in parallel).

// massRenameFocusName names the focused widget: r0..r3 (mode radios), o0..o3 (the option
// checkbox of column 0..3), find, repl, cw / cp (capitalize each word / punctuation), apply, ok,
// cancel; f0 / f1 in the save prompt; list in a picker. It is the only code that knows how the
// state encodes focus.
func massRenameFocusName(st dialog.FileDialogState) string {
	pickerName := func(f dlg.ID) string {
		switch f {
		case dlg.OK:
			return "ok"
		case dlg.Cancel:
			return "cancel"
		}
		return "list"
	}
	switch st.MassRenamePhase {
	case dialog.MassRenamePhaseLoadPicker, dialog.MassRenamePhaseOverwritePicker:
		return pickerName(st.MassRenameLoadPicker.Focus)
	case dialog.MassRenamePhaseHistoryPicker:
		return pickerName(st.MassRenameHistoryPicker.Focus)
	}
	if st.MassRenamePhase == dialog.MassRenamePhaseSavePrompt {
		switch st.Focus {
		case dlg.OK:
			return "ok"
		case dlg.Cancel:
			return "cancel"
		case dialog.FileFieldID(1):
			return "f1"
		}
		return "f0"
	}
	names := map[dlg.ID]string{
		dialog.MassRenameRadioSimple: "r0", dialog.MassRenameRadioRegex: "r1", dialog.MassRenameRadioEditor: "r2", dialog.MassRenameRadioCapitalize: "r3",
		dialog.MassRenameOptShowModified: "o0", dialog.MassRenameOptTrim: "o1", dialog.MassRenameOptCase: "o2", dialog.MassRenameOptIgnoreExt: "o3",
		dialog.MassRenameOptCapEachWord: "cw", dialog.MassRenameOptCapPunct: "cp",
		dialog.FileFieldID(0): "find", dialog.FileFieldID(1): "repl", dlg.OK: "ok", dialog.MassRenameApply: "apply", dlg.Cancel: "cancel",
	}
	if st.MassRenameMode == dialog.MassRenameModeUICapitalize && st.Focus == dialog.MassRenameOptIgnoreExt {
		return "o2"
	}
	if n, ok := names[st.Focus]; ok {
		return n
	}
	return "?" + string(st.Focus)
}

func massRenamePickerState(name string, pk dialog.MassRenamePatternPickerState) string {
	return fmt.Sprintf(" %s[items=%d ranked=%v query=%q@%d sel=%d scroll=%d]", name, len(pk.Items), pk.Ranked, pk.Query, pk.QueryCursor, pk.Selected, pk.ListScroll)
}

func massRenameDialogState(app *App) string {
	st := app.model.FileDialog
	var fields []string
	for _, f := range st.Fields {
		fields = append(fields, fmt.Sprintf("%s=%q@%d/%d pending=%v invalid=%v", f.Label, f.Value, f.Cursor, f.Scroll, f.PrefillPending, f.InputInvalid))
	}
	extra := ""
	switch st.MassRenamePhase {
	case dialog.MassRenamePhaseLoadPicker, dialog.MassRenamePhaseOverwritePicker:
		extra = massRenamePickerState("picker", st.MassRenameLoadPicker)
	case dialog.MassRenamePhaseHistoryPicker:
		extra = massRenamePickerState("picker", st.MassRenameHistoryPicker)
	}
	return fmt.Sprintf("focus=%s phase=%d mode=%d fields=[%s] case=%v trim=%v ext=%v modified=%v word=%v punct=%v preview=%d scroll=%d matches=%d okEnabled=%v external=%d hint=%q%s",
		massRenameFocusName(st), st.MassRenamePhase, st.MassRenameMode, strings.Join(fields, "; "),
		st.MassRenameCaseFold, st.MassRenameStripSpaces, st.MassRenameIgnoreExt, st.MassRenameShowOnlyModified,
		st.MassRenameCapEachWord, st.MassRenameCapPunctSep, len(st.MassRenamePreviewBefore), st.MassRenamePreviewScroll,
		st.MassRenameMatchCount, dialog.FileDialogMassRenameOKEnabled(st), len(st.MassRenameExternalNames), st.MassRenamePatternCompileHint, extra)
}

// massRenameSeedNames are the files in the panel; all of them are selected when the dialog opens.
var massRenameSeedNames = []string{"lantern-willow.txt", "lantern-meadow.txt", "orchard-ribbon.txt", "velvet.txt", "anchor.txt", "marble-otter.txt", "timber"}

func massRenameKeyDialog(name string, savedPatterns bool) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			harbor := filepath.Join(dir, "harbor")
			if err := os.MkdirAll(filepath.Join(harbor, "timber"), 0o755); err != nil {
				t.Fatal(err)
			}
			app := newApp(t, newScreen(t, 100, 40), harbor)
			app.model.Message = ""
			app.model.UserHomeDir = dir
			app.config.MassRename.File = filepath.Join(dir, "patterns.toml")
			if savedPatterns {
				pats := []ops.MassRenamePattern{
					{Name: "lantern-to-harbor", Description: "rename the lantern files", Mode: "simple", Find: "lantern", Replace: "harbor", CaseFold: true, StripSpaces: true, IgnoreExt: true},
					{Name: "orchard-to-meadow", Description: "rename the orchard files", Mode: "regex", Find: "orch(ard)", Replace: "meadow$1", CaseFold: true, StripSpaces: true, IgnoreExt: true},
					{Name: "title-case", Description: "capitalize words", Mode: "capitalize", CapEachWord: true, CapPunctSep: true, StripSpaces: true, IgnoreExt: true},
				}
				if err := ops.SaveMassRenamePatterns(app.config.MassRename.File, pats); err != nil {
					t.Fatal(err)
				}
			}
			for _, n := range massRenameSeedNames {
				if n != "timber" {
					writeFile(t, filepath.Join(harbor, n))
				}
				app.activePanel().AddSelection(filepath.Join(harbor, n))
			}
			app.dispatch(keymap.ActionFileRename)
			if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogMassRename {
				t.Fatal("mass rename dialog should be open")
			}
			return app
		},
		state: massRenameDialogState,
		open:  func(app *App) bool { return app.model.FileDialog.Open },
		outcome: func(app *App, _ bool) string {
			entries, _ := os.ReadDir(filepath.Join(app.model.UserHomeDir, "harbor"))
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			sort.Strings(names)
			pats, _ := os.ReadFile(app.config.MassRename.File)
			return fmt.Sprintf("message=%q files=%s patterns=%q", strings.TrimSpace(app.model.Message), strings.Join(names, ","), strings.ReplaceAll(string(pats), app.model.UserHomeDir, "<tmp>"))
		},
	}
}

// useFakeMassRenameEditor replaces the external editor with one that prefixes every line of the
// file, until the calling test (and its parallel scripts) finishes.
func useFakeMassRenameEditor(t *testing.T) {
	t.Helper()
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string, _ bool) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		for i := range lines {
			lines[i] = "e-" + lines[i]
		}
		return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })
}

// typeWord is the script tokens that type s one rune at a time.
func typeWord(s string) string {
	var toks []string
	for _, r := range s {
		if r == ' ' {
			toks = append(toks, "Space")
			continue
		}
		toks = append(toks, string(r))
	}
	return strings.Join(toks, " ")
}

func TestMassRenameKeyScripts(t *testing.T) {
	useFakeMassRenameEditor(t)
	find := typeWord("lantern")
	repl := typeWord("harbor")
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename", false), []simpleKeyScript{
		// The Find / Replace inputs.
		{"type-find", find},
		{"type-find-replace", find + " Tab " + repl},
		{"type-replace-via-down", find + " Down " + repl},
		{"edit-cursor", "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-empty", "a Backspace Backspace Delete Left Right Home End"},
		{"edit-keys", "x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"space-in-find", "a Space b"},
		{"letters-in-find", "a s r e z i m t x w p o c n O C"},
		{"no-match", typeWord("zzz")},
		{"conflict", typeWord("lantern-willow") + " Tab " + typeWord("velvet")},
		{"find-then-clear", find + " Ctrl+U"},
		// Mode radios: Space selects and keeps focus, Enter selects and moves on.
		{"radio-space-regex", "Up Right Space"},
		{"radio-enter-regex", "Up Right Enter"},
		{"radio-space-capitalize", "Up Right Right Right Space"},
		{"radio-enter-capitalize", "Up Right Right Right Enter"},
		{"radio-space-external", "Up Right Right Space"},
		{"radio-enter-external", "Up Right Right Enter"},
		{"radio-enter-simple-again", "Up Enter"},
		{"radio-plain-letters", "Up s r e z x"},
		// Mode and option mnemonics, also from the inputs.
		{"alt-s", "Alt+r Alt+s"},
		{"alt-r", "Alt+r"},
		{"alt-e", "Alt+e"},
		{"alt-z", "Alt+z"},
		{"alt-r-in-find", find + " Alt+r"},
		{"alt-e-in-find", find + " Alt+e Tab"},
		{"alt-z-in-replace", "Tab Alt+z Tab"},
		{"alt-m", "Alt+m Alt+m"},
		{"alt-t", "Alt+t"},
		{"alt-i", "Alt+i"},
		{"alt-x", "Alt+x"},
		{"alt-m-from-button", "Down Down Down Alt+m"},
		{"alt-m-from-radio", "Up Alt+t"},
		{"alt-options-in-find", find + " Alt+m Alt+t Alt+i Alt+x"},
		{"alt-w-simple-ignored", "Alt+w Alt+p"},
		{"alt-unknown", "a Alt+q Alt+y b"},
		{"alt-upper", "Alt+R Alt+M"},
		{"alt-i-external", "Alt+e Alt+i Alt+x"},
		{"alt-w-capitalize", "Alt+z Alt+w Alt+p"},
		{"alt-p-capitalize", "Alt+z Alt+p"},
		{"alt-p-then-w", "Alt+z Alt+p Alt+w"},
		{"alt-i-capitalize", "Alt+z Alt+i"},
		// Checkboxes.
		{"checkbox-space", "Up Down Space Right Space Right Space Right Space"},
		{"checkbox-enter", "Up Down Enter Right Enter"},
		{"checkbox-plain-letters", "Up Down m t i x Space"},
		// Regex mode.
		{"regex-valid", "Alt+r " + typeWord("orch(ard)") + " Tab " + typeWord("meadow$1")},
		{"regex-invalid", "Alt+r " + typeWord("orch(ard")},
		{"regex-then-fix", "Alt+r " + typeWord("orch(ard") + " Right )"},
		{"regex-to-simple-keeps-text", "Alt+r " + typeWord("orch(ard") + " Alt+s"},
		{"regex-find-focus-then-capitalize", "Alt+r " + find + " Alt+z"},
		{"regex-replace-focus-then-external", "Alt+r Tab Alt+e"},
		{"regex-case-toggle", "Alt+r " + typeWord("LANTERN") + " Alt+i"},
		// Capitalize mode.
		{"cap-walk", "Alt+z Tab Tab Tab Tab Tab"},
		{"cap-down", "Alt+z Down Down Down Down Down Down Down"},
		{"cap-up", "Alt+z Down Down Down Up Up Up Up Up"},
		{"cap-backtab", "Alt+z Backtab Backtab Backtab Backtab"},
		{"cap-checkboxes", "Alt+z Tab Space Down Space"},
		{"cap-checkboxes-enter", "Alt+z Tab Enter Down Enter"},
		{"cap-options-row", "Alt+z Up Down Right Right Right Right"},
		{"cap-radio-down", "Alt+z Up Right Right Right Down Up"},
		{"cap-focus-on-find-then-cap", find + " Alt+z Down"},
		// External editor mode.
		{"external-walk", "Alt+e Tab Tab Tab Tab"},
		{"external-down", "Alt+e Down Down Down Down"},
		{"external-up", "Alt+e Down Down Down Up Up Up Up"},
		{"external-backtab", "Alt+e Backtab Backtab Backtab"},
		{"external-radio-enter-launches", "Alt+e Up Right Right Enter"},
		{"external-radio-space-no-launch", "Alt+e Up Right Right Space"},
		{"external-f4", "Alt+e F4"},
		{"f4-from-simple", "F4"},
		{"f4-in-find", find + " F4"},
		{"external-ok-before-editor", "Alt+e Down Down Down Enter"},
		{"external-ok-after-editor", "Alt+e F4 Down Down Down Enter"},
		{"external-apply-after-editor", "Alt+e F4 Alt+a"},
		{"external-modified-toggle", "Alt+e F4 Alt+m Alt+t"},
		{"external-mode-to-simple", "Alt+e F4 Alt+s"},
		// Preview scrolling.
		{"preview-in-find", find + " PgDn PgUp"},
		// OK, Apply, Cancel and the chords.
		{"enter-in-find-renames", find + " Tab " + repl + " Enter"},
		{"enter-in-find-nothing", "Enter"},
		{"enter-in-find-no-match", typeWord("zzz") + " Enter"},
		{"enter-in-find-conflict", typeWord("lantern-willow") + " Tab " + typeWord("velvet") + " Enter"},
		{"enter-ok", find + " Tab " + repl + " Down Enter"},
		{"enter-ok-disabled", "Down Down Down Enter"},
		{"enter-apply", find + " Tab " + repl + " Down Right Enter"},
		{"enter-apply-twice", find + " Tab " + repl + " Down Right Enter Tab " + typeWord("harbor") + " Tab " + typeWord("lantern") + " Down Right Enter"},
		{"enter-apply-disabled", "Down Down Right Enter"},
		{"enter-cancel", "Down Down Right Right Enter"},
		{"space-ok", find + " Tab " + repl + " Down Space"},
		{"space-apply", find + " Tab " + repl + " Down Right Space"},
		{"space-cancel", "Down Down Right Right Space"},
		{"plain-letters-on-buttons", "Down Down o a c x"},
		{"alt-a", find + " Tab " + repl + " Alt+a"},
		{"alt-a-disabled", "Alt+a"},
		{"alt-a-then-keys", find + " Tab " + repl + " Alt+a x"},
		{"alt-o", find + " Tab " + repl + " Alt+o"},
		{"alt-o-disabled", "Alt+o"},
		{"alt-o-conflict", typeWord("lantern-willow") + " Tab " + typeWord("velvet") + " Alt+o"},
		{"alt-c", "a Alt+c"},
		{"alt-enter", find + " Tab " + repl + " Alt+Enter"},
		{"esc", "a Esc"},
		{"esc-from-radio", "Up Esc"},
		{"esc-from-button", "Down Down Right Right Esc"},
		{"show-only-modified", find + " Alt+m"},
		{"show-only-modified-nothing-changes", "Alt+m"},
		{"ignore-ext-off", typeWord("txt") + " Alt+x"},
		{"trim-off", typeWord("lantern") + " Alt+t"},
		// Sub-screen openers from every focus position.
		{"f2-opens-load", "F2"},
		{"f3-opens-history", "F3"},
		{"f5-opens-save", find + " F5"},
		{"f5-external-ignored", "Alt+e F5"},
		{"f2-from-radio", "Up F2"},
		{"f2-from-button", "Down Down Down F2"},
		{"f-keys-ignored", "F7 a"},
	})
}

// massRenameFind and massRenameRepl are the tokens that type the Find / Replace values the
// sub-screen scripts start from.
var (
	massRenameFind = typeWord("lantern")
	massRenameRepl = typeWord("harbor")
)

func TestMassRenameSaveKeyScripts(t *testing.T) {
	useFakeMassRenameEditor(t)
	open := massRenameFind + " Tab " + massRenameRepl + " F5 "
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_save", false), []simpleKeyScript{
		{"open", open + "a"},
		{"type-name", open + typeWord("pattern")},
		{"type-both", open + typeWord("pattern") + " Tab " + typeWord("rename lantern files")},
		{"edit-cursor", open + "x y z Left Left Backspace Home Delete End q Right Right"},
		{"edit-keys", open + "x y z Ctrl+W a b Ctrl+U c d Ctrl+K e"},
		{"space-in-name", open + "a Space b"},
		{"letters-in-name", open + "a t p r o c y n O C"},
		{"tab-walk", open + "Tab Tab Tab Tab Tab"},
		{"backtab-walk", open + "Backtab Backtab Backtab Backtab Backtab"},
		{"down-walk", open + "Down Down Down Down"},
		{"up-walk", open + "Down Down Down Up Up Up Up"},
		{"buttons-left-right", open + "Down Down Left Right Right Left Left"},
		{"mnemonic-in-name", open + "a Alt+q Alt+x b"},
		{"alt-on-button", open + "Down Down Alt+q"},
		{"plain-letters-on-buttons", open + "Down Down o c x"},
		{"enter-name-saves", open + typeWord("pattern") + " Enter"},
		{"enter-description-saves", open + typeWord("pattern") + " Tab " + typeWord("some words") + " Enter"},
		{"enter-empty-name", open + "Enter"},
		{"enter-blank-name", open + "Space Enter"},
		{"enter-ok", open + typeWord("pattern") + " Down Down Enter"},
		{"enter-cancel", open + "Down Down Right Enter"},
		{"space-ok", open + typeWord("pattern") + " Down Down Space"},
		{"space-cancel", open + "Down Down Right Space"},
		{"alt-o", open + typeWord("pattern") + " Alt+o"},
		{"alt-o-empty", open + "Alt+o"},
		{"alt-c", open + "a Alt+c"},
		{"alt-enter", open + typeWord("pattern") + " Alt+Enter"},
		{"esc", open + "a Esc"},
		{"esc-from-button", open + "Down Down Right Esc"},
		{"esc-then-main", open + "a Esc Tab"},
		{"restore-default", open + "x y z Alt+Backspace"},
		{"f5-opens-overwrite-empty", open + "F5"},
		{"f2-ignored", open + "F2 F3 F4 a"},
		{"save-then-reopen", open + typeWord("pattern") + " Enter F5 a"},
	})
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_save_saved", true), []simpleKeyScript{
		{"overwrite-opens", open + "F5"},
		{"overwrite-keeps-typed-description", open + typeWord("pattern") + " Tab " + typeWord("some words") + " F5"},
	})
}

func TestMassRenameLoadKeyScripts(t *testing.T) {
	useFakeMassRenameEditor(t)
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_load", true), massRenamePickerScripts("F2 "))
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_load_empty", false), []simpleKeyScript{
		{"defaults", "F2"},
	})
}

func TestMassRenameHistoryKeyScripts(t *testing.T) {
	useFakeMassRenameEditor(t)
	// The history holds the patterns applied so far in the session.
	seed := massRenameFind + " Tab " + massRenameRepl + " Down Right Enter " +
		"Tab " + typeWord("harbor") + " Tab " + typeWord("lantern") + " Down Right Enter "
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_history", false), massRenamePickerScripts(seed+"F3 "))
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_history_empty", false), []simpleKeyScript{
		{"empty", "F3"},
		{"empty-enter", "F3 Enter"},
		{"empty-f8", "F3 F8"},
	})
}

func TestMassRenameOverwriteKeyScripts(t *testing.T) {
	useFakeMassRenameEditor(t)
	open := massRenameFind + " Tab " + massRenameRepl + " F5 F5 "
	runKeyGoldenScripts(t, massRenameKeyDialog("mass_rename_overwrite", true), massRenamePickerScripts(open))
}

// massRenamePickerScripts are the key scripts every pattern picker runs; open reaches the picker.
func massRenamePickerScripts(open string) []simpleKeyScript {
	return []simpleKeyScript{
		{"open", open + "x"},
		{"type-filter", open + typeWord("or")},
		{"type-no-match", open + typeWord("zzz")},
		{"type-edit", open + "o r Backspace Backspace Left Home Delete End"},
		{"type-space", open + "o Space r"},
		{"down-walk", open + "Down Down Down Down"},
		{"up-walk", open + "Down Down Up Up Up"},
		{"page-keys", open + "PgDn PgDn PgUp PgUp"},
		{"home-end", open + "End Home End Up"},
		{"ctrl-home-end", open + "Ctrl+End Ctrl+Home"},
		{"tab", open + "Tab Tab Tab Tab"},
		{"backtab", open + "Backtab Backtab Backtab"},
		{"left-right-buttons", open + "Tab Left Right Right Right Left Left Left"},
		{"up-from-ok", open + "Down Tab Up"},
		{"up-from-cancel", open + "Down Tab Tab Up"},
		{"down-from-ok", open + "Tab Down Down"},
		{"enter-selects", open + "Down Enter"},
		{"enter-selects-first", open + "Enter"},
		{"enter-selects-filtered", open + typeWord("or") + " Down Enter"},
		{"enter-ok", open + "Down Tab Enter"},
		{"enter-cancel", open + "Tab Tab Enter"},
		{"enter-no-match", open + typeWord("zzz") + " Enter"},
		{"space-on-ok", open + "Down Tab Space"},
		{"space-on-cancel", open + "Tab Tab Space"},
		{"space-in-list", open + "Down Space"},
		{"plain-o-on-buttons", open + "Down Tab o"},
		{"plain-c-on-buttons", open + "Down Tab c"},
		{"plain-other-on-buttons", open + "Tab x y"},
		{"alt-o", open + "Down Alt+o"},
		{"alt-c", open + "Down Alt+c"},
		{"alt-enter", open + "Down Alt+Enter"},
		{"esc", open + "Down Esc"},
		{"esc-from-button", open + "Tab Esc"},
		{"esc-then-main", open + "Down Esc Down"},
		{"delete-selected", open + "Down F8"},
		{"delete-filtered", open + typeWord("or") + " F8"},
		{"delete-all", open + "F8 F8 F8 F8"},
		{"delete-from-button", open + "Tab F8"},
		{"enter-then-edit", open + "Down Enter x"},
		{"f-keys-ignored", open + "F4 F7 e"},
	}
}
