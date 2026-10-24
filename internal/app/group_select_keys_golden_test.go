package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Key-script goldens for the Select / Unselect group dialog (harness:
// simple_dialog_keys_golden_test.go; fixtures: filter_dialog_keys_golden_test.go).
// groupSelectKeyFocusName is the only code that knows how the state encodes focus.

func groupSelectKeyState(app *App) string {
	st := app.model.GroupSelect
	var types []string
	for i, on := range st.Types {
		if on {
			types = append(types, strings.ToLower(dialog.FilterTypeLabels[i]))
		}
	}
	return fmt.Sprintf("focus=%s text=%q cursor=%d scroll=%d mode=%d files=%v dirs=%v case=%v fullpath=%v meta=%v/%v types=%v preview=%v/%d/%d",
		patternKeyFocusName(st.Focus), st.Text, st.TextCursor, st.TextScroll, st.PatternMode, st.FilesOnly, st.DirsOnly,
		st.CaseSensitive, st.FullPath, st.IncludeMetaColumns, st.OnlyMetaColumns, types, st.PreviewShow, st.PreviewFiles, st.PreviewFolders)
}

// groupSelectKeyDialog opens the dialog in mode ("select" / "unselect") and context. With
// preselected, two fixture files are selected first so unselect has something to flip.
func groupSelectKeyDialog(name, mode, context string, metas int, preselected bool) keyGoldenDialog {
	d := keyGoldenDialog{
		name: name, rx: 10, ry: 5, rw: 60, rh: 13,
		setup: func(t *testing.T, dir string) *App {
			seedPatternFixtures(t, dir)
			h := 24
			if metas > 0 {
				h = 25
			}
			app := newApp(t, newScreen(t, 80, h), dir)
			app.model.Message = ""
			app.model.ActivePanel = ui.PrimaryPanel
			titles := []string{"duration", "bitrate"}
			var cols []ui.MetaColumnState
			for _, title := range titles[:metas] {
				cols = append(cols, ui.MetaColumnState{EntryName: title, ColumnTitle: title})
			}
			app.model.MetaResults[ui.PrimaryPanel] = cols
			if preselected {
				for _, n := range []string{"orchard.mp4", "willow.mkv"} {
					app.model.Primary.TogglePathSelection(filepath.Join(dir, n))
				}
			}
			app.openGroupSelect(mode, context)
			return app
		},
		state: groupSelectKeyState,
		open:  func(app *App) bool { return app.model.GroupSelect.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("message=%q selected=%d", app.model.Message, len(app.model.Primary.SelectedPaths))
		},
	}
	if metas > 0 {
		d.rh = 14
	}
	return d
}

func TestGroupSelectKeyScripts(t *testing.T) {
	panelScripts := []simpleKeyScript{
		// Focus movement.
		{"down-walk", repeatTok("Down", 9)},
		{"down-walk-from-top", "Up Up Up " + repeatTok("Down", 9)},
		{"up-walk", repeatTok("Up", 6)},
		{"up-walk-from-buttons", repeatTok("Down", 5) + " " + repeatTok("Up", 8)},
		{"grid-right-left", "Up Left Right Right Right Left Left Left"},
		{"grid-columns", "Up Up Up Right Down Down Right Down Down Left Down Up"},
		{"grid-types-col2", "Up Up Up Right Right Down Down Right Up Up"},
		{"grid-down-from-short-column", "Up Up Up Right Right Down Down Down"},
		{"grid-up-from-pattern-to-radio", "Up Up Right Up Down"},
		{"grid-down-from-types-to-pattern", "Up Up Up Right Down Down Down Up"},
		{"options-rows", "Down Right Down Left Down Down Up Up Up"},
		{"options-rows-right-edges", "Down Right Right Down Right Right Up Right"},
		{"options-from-column-two", "Down Right Down Down Up Left"},
		{"buttons-left-right", "Down Down Down Left Right Right Right Left Left Left"},
		{"buttons-up-from-ok", "Down Down Down Up Down Down Up"},
		{"buttons-up-from-cancel", "Down Down Down Right Up Right Up"},
		{"buttons-down-stays", "Down Down Down Down Down Right Down Down"},
		{"tab-walk", repeatTok("Tab", 7)},
		{"backtab-walk", repeatTok("Backtab", 7)},
		{"tab-from-radios", "Up Up Up Tab Tab Tab Tab Tab"},
		{"tab-from-types", "Up Up Up Right Down Tab Tab Backtab Backtab"},
		{"tab-from-types-col2", "Up Up Up Right Right Down Tab Backtab"},
		{"tab-from-options", "Down Right Tab Tab Backtab Backtab Backtab"},
		{"tab-from-buttons", "Down Down Down Right Tab Backtab Backtab"},
		// Activation.
		{"space-radios", "Up Up Up Space Down Space Down Space"},
		{"enter-radios", "Up Up Up Enter Down Enter Down Enter"},
		{"enter-radio-with-text", typeTokens("*.mp4") + " Up Up Enter Up Enter"},
		{"space-types", "Up Up Up Right Space Down Space Down Space Right Space Down Space"},
		{"enter-type-applies-group", typeTokens("*.mp4") + " Up Up Up Right Enter"},
		{"space-options", "Down Space Right Space Down Space"},
		{"enter-option-applies-group", typeTokens("*.zip") + " Down Enter"},
		{"space-ok", typeTokens("*.zip") + " Down Down Down Space"},
		{"enter-ok", typeTokens("*.zip") + " Down Down Down Enter"},
		{"space-ok-empty-pattern", "Down Down Down Space"},
		{"space-cancel", typeTokens("*.zip") + " Down Down Down Right Space"},
		{"enter-cancel", typeTokens("*.zip") + " Down Down Down Right Enter"},
		{"enter-in-pattern", typeTokens("*.mp3") + " Enter"},
		{"enter-in-pattern-empty", "Enter"},
		{"enter-bad-pattern", "[ Enter"},
		{"enter-bad-regex", "Alt+r ( Enter"},
		{"esc", typeTokens("abc") + " Esc"},
		{"esc-from-checkbox", "Down Space Esc"},
		{"alt-o-applies", typeTokens("*.mkv") + " Alt+o"},
		{"alt-c-cancels", typeTokens("*.mkv") + " Alt+c"},
		{"alt-enter-applies", typeTokens("*.mkv") + " Alt+Enter"},
		{"alt-o-from-checkbox", typeTokens("*.mkv") + " Down Space Alt+o"},
		{"alt-o-from-radio", typeTokens("*.mkv") + " Up Up Alt+o"},
		{"alt-o-from-type", typeTokens("*.mkv") + " Up Up Up Right Alt+o"},
		{"alt-o-from-ok", typeTokens("*.mkv") + " Down Down Down Alt+o"},
		{"alt-o-from-cancel", typeTokens("*.mkv") + " Down Down Down Right Alt+o"},
		{"alt-c-from-ok", typeTokens("*.mkv") + " Down Down Down Alt+c"},
		{"alt-c-from-radio", typeTokens("*.mkv") + " Up Alt+c"},
		{"alt-o-radio-after-change", typeTokens("*.mkv") + " Up Up Alt+l Alt+o"},
		{"alt-enter-from-radio", typeTokens("*.mkv") + " Up Up Alt+Enter"},
		// Typing and editing in the pattern input.
		{"type-edit", typeTokens("willow") + " Left Left Backspace x Home Delete End 4 2"},
		{"type-space", "a Space b Space"},
		{"type-long-scroll", typeTokens(strings.Repeat("meadow-", 12)) + " Home End Left Left"},
		{"home-end", "a b c Home Home End Left Left Right"},
		{"edit-empty", "Backspace Delete Left Right Home End"},
		{"readline-kill-word", typeTokens("willow notes txt") + " Ctrl+W Ctrl+W"},
		{"readline-kill-line", typeTokens("willow notes txt") + " Left Left Ctrl+U Ctrl+Y"},
		{"readline-kill-to-end", typeTokens("willow notes txt") + " Left Left Left Ctrl+K Ctrl+Y"},
		{"readline-kill-all", typeTokens("willow notes") + " Ctrl+L"},
		{"readline-alt-backspace", typeTokens("willow notes") + " Alt+Backspace"},
		{"readline-alt-d-is-dirs-only", typeTokens("willow notes") + " Home Alt+d"},
		{"plain-letters-ignored-on-options", "Down o c x s r l Space"},
		{"plain-letters-ignored-on-radio", "Up Up Up o c x s"},
		{"plain-letters-ignored-on-buttons", "Down Down Down o c x"},
		{"unknown-keys", "Insert F3 PgUp PgDn"},
		// Live preview and applying.
		{"preview-some", typeTokens("*.mp4")},
		{"preview-many", "*"},
		{"preview-zero", typeTokens("zzz")},
		{"preview-dirs", typeTokens("a") + " Alt+d"},
		{"preview-files-only", typeTokens("*") + " Alt+y"},
		{"preview-types", "Alt+v Alt+u"},
		{"preview-types-with-pattern", typeTokens("w") + " Alt+v"},
		{"preview-case", typeTokens("WILLOW") + " Alt+l Alt+e"},
		{"preview-simple", typeTokens("notes") + " Alt+l"},
		{"preview-regex", typeTokens("^w.*txt$") + " Alt+r"},
		{"preview-regex-bad", typeTokens("(") + " Alt+r"},
		{"preview-shell-bad", typeTokens("[") + " Alt+s"},
		{"preview-after-backspace", typeTokens("zzz") + " Backspace Backspace Backspace"},
		{"apply-select", typeTokens("*.mp4") + " Enter"},
		{"apply-select-nothing", typeTokens("zzz") + " Enter"},
		{"apply-select-types", "Alt+v Down Down Space"},
		// Mnemonics.
		{"alt-mnemonics-in-pattern", "Alt+s Alt+r Alt+l Alt+y Alt+d Alt+y Alt+e Alt+v Alt+p Alt+a Alt+u Alt+t"},
		{"alt-mnemonics-upper", "Alt+S Alt+R Alt+L Alt+Y Alt+D Alt+E Alt+V Alt+P"},
		{"alt-radios-from-checkbox", "Down Alt+r Alt+l Alt+s"},
		{"alt-radios-from-button", "Down Down Down Alt+r"},
		{"alt-radios-from-type", "Up Up Up Right Alt+l"},
		{"alt-checkbox-from-radio", "Up Up Up Alt+e Alt+y"},
		{"alt-type-from-other-row", "Down Alt+v Alt+t"},
		{"alt-checkbox-from-button", "Down Down Down Alt+e"},
		{"alt-checkbox-from-type", "Up Up Up Right Alt+e"},
		{"alt-unknown-in-pattern", "a Alt+x Alt+q Alt+g b"},
		{"alt-invert-is-unknown", "Alt+i Alt+i"},
		{"alt-full-path-without-find", "Alt+h Alt+h"},
		{"alt-meta-without-meta", "Alt+m Alt+n"},
		{"alt-dirs-only-disables-types", "Alt+v Alt+d Alt+v Alt+d Alt+v"},
		{"alt-dirs-only-then-nav-types", "Alt+v Alt+d Up Up Up Right Down Down Right Left Left"},
		{"alt-dirs-only-then-space-type", "Alt+d Up Up Up Right Space Down Space"},
		{"alt-dirs-only-focus-type-then-toggle", "Up Up Up Right Alt+d"},
		{"alt-files-only-excludes-dirs-only", "Alt+y Alt+d Alt+y Alt+y"},
		{"alt-dirs-only-from-options", "Down Alt+d Alt+y"},
	}
	runKeyGoldenScripts(t, groupSelectKeyDialog("group_select", "select", "panel", 0, false), panelScripts)
	runKeyGoldenScripts(t, groupSelectKeyDialog("group_select_unselect", "unselect", "panel", 0, true), []simpleKeyScript{
		{"preview-some", typeTokens("*.mp4")},
		{"preview-all", "*"},
		{"apply-unselect", typeTokens("*.mp4") + " Enter"},
		{"apply-unselect-nothing", typeTokens("*.zip") + " Enter"},
		{"apply-unselect-types", "Alt+v Down Down Space"},
	})
	findScripts := []simpleKeyScript{
		// The find context never has meta columns and adds the Match full path checkbox. No script
		// restricts anything, so no asynchronous count starts.
		{"down-walk", repeatTok("Down", 9)},
		{"up-walk-from-buttons", repeatTok("Down", 5) + " " + repeatTok("Up", 8)},
		{"up-from-ok", "Down Down Down Up"},
		{"up-from-cancel", "Down Down Down Right Up"},
		{"options-rows", "Down Right Down Left Down Down Up Up Up"},
		{"options-rows-right-edges", "Down Right Right Down Right Right Up Right"},
		{"options-from-column-two", "Down Right Down Down Up Left"},
		{"tab-walk", repeatTok("Tab", 7)},
		{"backtab-walk", repeatTok("Backtab", 7)},
		{"tab-from-options", "Down Right Tab Tab Backtab Backtab Backtab"},
		{"space-full-path", "Down Right Down Right Space Space"},
		{"enter-full-path", "Down Right Down Right Enter"},
		{"alt-h-in-pattern", "Alt+h Alt+h Alt+H"},
		{"alt-h-from-radio", "Up Alt+h"},
		{"alt-h-from-button", "Down Down Down Alt+h"},
		{"alt-mnemonics-in-pattern", "Alt+h Alt+e Alt+y Alt+d Alt+y"},
		{"alt-meta-is-unknown", "Alt+m Alt+n"},
		{"space-ok-empty", "Down Down Down Space"},
		{"esc", "Alt+h Esc"},
	}
	runKeyGoldenScripts(t, groupSelectKeyDialog("group_select_find", "select", "find", 0, false), findScripts)
	metaScripts := []simpleKeyScript{
		{"down-walk", repeatTok("Down", 10)},
		{"up-walk-from-buttons", repeatTok("Down", 6) + " " + repeatTok("Up", 8)},
		{"meta-row-nav", "Down Down Down Right Left Down Up Up"},
		{"meta-row-right-edges", "Down Down Down Right Right Up Right"},
		{"tab-walk", repeatTok("Tab", 7)},
		{"backtab-walk", repeatTok("Backtab", 7)},
		{"tab-from-meta-row", "Down Down Down Right Tab Backtab Backtab"},
		{"up-from-ok", repeatTok("Down", 4) + " Up"},
		{"up-from-cancel", repeatTok("Down", 4) + " Right Up"},
		{"space-meta", "Down Down Down Space Right Space Left Space"},
		{"enter-meta-applies", typeTokens("*") + " Down Down Down Enter"},
		{"alt-meta-in-pattern", "Alt+m Alt+n Alt+m Alt+n Alt+n"},
		{"alt-meta-from-options", "Down Alt+m Alt+n"},
		{"alt-meta-from-button", "Down Down Down Down Alt+n"},
		{"alt-only-meta-implies-include", "Alt+m Alt+n Alt+m"},
		{"preview-meta", typeTokens("lantern") + " Alt+n"},
		{"space-ok", typeTokens("*.zip") + " Down Down Down Down Space"},
		{"alt-o", typeTokens("*.zip") + " Alt+m Alt+o"},
	}
	runKeyGoldenScripts(t, groupSelectKeyDialog("group_select_meta", "select", "panel", 2, false), metaScripts)
}
