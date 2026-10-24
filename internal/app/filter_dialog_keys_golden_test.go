package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the Filter dialog (harness: simple_dialog_keys_golden_test.go). The
// panel holds fixture entries so the live preview counts something. filterKeyFocusName is the only
// code that knows how the state encodes focus.

// typeTokens turns text into script tokens (one key per rune).
func typeTokens(text string) string {
	var toks []string
	for _, r := range text {
		if r == ' ' {
			toks = append(toks, "Space")
			continue
		}
		toks = append(toks, string(r))
	}
	return strings.Join(toks, " ")
}

func repeatTok(tok string, n int) string { return strings.TrimSpace(strings.Repeat(tok+" ", n)) }

// seedPatternFixtures fills dir with deterministic entries: seven files and two directories.
func seedPatternFixtures(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"orchard.mp4", "willow.mkv", "meadow.jpg", "harbor.zip", "lantern.txt", "quartz.mp3", "willow-notes.txt"} {
		writeFile(t, filepath.Join(dir, name))
	}
	for _, name := range []string{"anchor", "beacon"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// patternKeyFocusName names the focused widget of the Filter and Select group dialogs.
func patternKeyFocusName(id dlg.ID) string {
	names := map[dlg.ID]string{
		dialog.PatternShellRadio: "shell", dialog.PatternRegexRadio: "regex", dialog.PatternSimpleRadio: "simple",
		dialog.PatternInput: "pattern", dialog.PatternFilesOnly: "filesonly", dialog.PatternDirsOnly: "dirsonly",
		dialog.PatternInvert: "invert", dialog.PatternCase: "case", dialog.PatternFullPath: "fullpath",
		dialog.PatternIncludeMeta: "includemeta", dialog.PatternOnlyMeta: "onlymeta", dlg.OK: "ok", dlg.Cancel: "cancel",
	}
	if n, ok := names[id]; ok {
		return n
	}
	for i, l := range dialog.FilterTypeLabels {
		if id == dialog.PatternTypeID(i) {
			return strings.ToLower(l)
		}
	}
	return "?" + string(id)
}

func filterKeyState(app *App) string {
	st := app.model.FilterDialog
	var types []string
	for i, on := range st.Types {
		if on {
			types = append(types, strings.ToLower(dialog.FilterTypeLabels[i]))
		}
	}
	return fmt.Sprintf("focus=%s text=%q cursor=%d scroll=%d mode=%d files=%v dirs=%v invert=%v case=%v meta=%v/%v types=%v preview=%v/%d/%d",
		patternKeyFocusName(st.Focus), st.Text, st.TextCursor, st.TextScroll, st.PatternMode, st.FilesOnly, st.DirsOnly, st.Invert,
		st.CaseSensitive, st.IncludeMetaColumns, st.OnlyMetaColumns, types, st.PreviewShow, st.PreviewFiles, st.PreviewFolders)
}

func filterKeyDialog(name string, metas int) keyGoldenDialog {
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
			app.dialogCtrl.OpenFilterDialog()
			return app
		},
		state: filterKeyState,
		open:  func(app *App) bool { return app.model.FilterDialog.Open },
		outcome: func(app *App, _ bool) string {
			return fmt.Sprintf("message=%q visible=%d", app.model.Message, app.model.Primary.VisibleEntryCount())
		},
	}
	if metas > 0 {
		d.rh = 14
	}
	return d
}

func TestFilterDialogKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
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
		{"grid-up-into-types", "Up Up Up Right Right Down Up"},
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
		{"enter-type-applies-filter", typeTokens("*.mp4") + " Up Up Up Right Enter"},
		{"space-options", "Down Space Right Space Down Space Right Space"},
		{"enter-option-applies-filter", typeTokens("*.zip") + " Down Enter"},
		{"space-ok", typeTokens("*.zip") + " Down Down Down Space"},
		{"enter-ok", typeTokens("*.zip") + " Down Down Down Enter"},
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
		{"type-star-dot", typeTokens("*.mp4")},
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
		// Live preview.
		{"preview-some", typeTokens("*.mp4")},
		{"preview-many", "*"},
		{"preview-zero", typeTokens("zzz")},
		{"preview-dirs", typeTokens("a") + " Alt+d"},
		{"preview-files-only", typeTokens("*") + " Alt+y"},
		{"preview-types", "Alt+v Alt+u"},
		{"preview-types-with-pattern", typeTokens("w") + " Alt+v"},
		{"preview-invert", typeTokens("*.mp4") + " Alt+i"},
		{"preview-case", typeTokens("WILLOW") + " Alt+l Alt+e"},
		{"preview-simple", typeTokens("notes") + " Alt+l"},
		{"preview-regex", typeTokens("^w.*txt$") + " Alt+r"},
		{"preview-regex-bad", typeTokens("(") + " Alt+r"},
		{"preview-shell-bad", typeTokens("[") + " Alt+s"},
		{"preview-after-backspace", typeTokens("zzz") + " Backspace Backspace Backspace"},
		// Mnemonics.
		{"alt-mnemonics-in-pattern", "Alt+s Alt+r Alt+l Alt+y Alt+d Alt+y Alt+i Alt+e Alt+v Alt+p Alt+a Alt+u Alt+t"},
		{"alt-mnemonics-upper", "Alt+S Alt+R Alt+L Alt+Y Alt+D Alt+I Alt+E Alt+V Alt+P"},
		{"alt-radios-from-checkbox", "Down Alt+r Alt+l Alt+s"},
		{"alt-radios-from-button", "Down Down Down Alt+r"},
		{"alt-radios-from-type", "Up Up Up Right Alt+l"},
		{"alt-checkbox-from-radio", "Up Up Up Alt+e Alt+i"},
		{"alt-type-from-other-row", "Down Alt+v Alt+t"},
		{"alt-checkbox-from-button", "Down Down Down Alt+e"},
		{"alt-checkbox-from-type", "Up Up Up Right Alt+i"},
		{"alt-unknown-in-pattern", "a Alt+x Alt+q Alt+g b"},
		{"alt-meta-without-meta", "Alt+m Alt+n"},
		{"alt-dirs-only-disables-types", "Alt+v Alt+d Alt+v Alt+d Alt+v"},
		{"alt-dirs-only-then-nav-types", "Alt+v Alt+d Up Up Up Right Down Down Right Left Left"},
		{"alt-dirs-only-then-space-type", "Alt+d Up Up Up Right Space Down Space"},
		{"alt-dirs-only-focus-type-then-toggle", "Up Up Up Right Alt+d"},
		{"alt-files-only-excludes-dirs-only", "Alt+y Alt+d Alt+y Alt+y"},
		{"alt-dirs-only-from-options", "Down Alt+d Alt+y"},
	}
	runKeyGoldenScripts(t, filterKeyDialog("filter", 0), scripts)
	meta := []simpleKeyScript{
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
	runKeyGoldenScripts(t, filterKeyDialog("filter_meta", 2), meta)
}
