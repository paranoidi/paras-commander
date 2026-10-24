package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the preview settings dialog, run through App.handleKey with the
// harness of simple_dialog_keys_golden_test.go. previewKeyFocusNames is the only code that knows
// how the state encodes focus.

func previewKeyFocusName(app *App) string {
	switch id := app.model.PreviewSettingsDialog.Focus; id {
	case "": // an unset focus is the first widget
		return "sixel-auto"
	case dialog.PreviewVideoMetadata:
		return "video"
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	default:
		row, value, _ := strings.Cut(string(id), ":")
		return row + "-" + value
	}
}

// previewKeyDialog opens the dialog on a screen tall enough that the dialog and the
// shorter engine one share the same top row.
func previewKeyDialog(name string, env map[string]string) keyGoldenDialog {
	return keyGoldenDialog{
		name: name, rx: 17, ry: 4, rw: 46, rh: 22,
		setup: func(t *testing.T, dir string) *App {
			for _, k := range []string{"TMUX", "TERM_PROGRAM", "TERM"} {
				t.Setenv(k, env[k])
			}
			app := newApp(t, newScreen(t, 80, 31), dir)
			app.model.Message = ""
			app.openPreviewSettingsDialog()
			return app
		},
		state: func(app *App) string {
			st := app.model.PreviewSettingsDialog
			return fmt.Sprintf("focus=%s sixel=%s kitty=%s placeholder=%s protocol=%s metadata=%s video=%v",
				previewKeyFocusName(app), st.Sixel, st.Kitty, st.KittyPlaceholder, st.Protocol, st.ImageMetadata, st.VideoMetadata)
		},
		open: func(app *App) bool { return app.model.PreviewSettingsDialog.Open },
		outcome: func(app *App, _ bool) string {
			p := app.config.Preview
			return fmt.Sprintf("saved=%v sixel=%s kitty=%s placeholder=%s protocol=%s metadata=%s video=%v",
				strings.HasPrefix(app.model.Message, "Preview settings saved"),
				p.TerminalSixel, p.TerminalKitty, p.TerminalKittyPlaceholder, p.ImageProtocol, p.ImageMetadata, p.VideoMetadata)
		},
	}
}

func dn(n int) string { return strings.Repeat("Down ", n) }

func TestPreviewSettingsKeyScripts(t *testing.T) {
	plain := previewKeyDialog("preview_settings", nil)
	runKeyGoldenScripts(t, plain, []simpleKeyScript{
		{"down-walk", dn(15)},
		{"up-walk", dn(12) + strings.Repeat("Up ", 15)},
		{"capability-left-right", "Right Right Right Left Left Left Left"},
		{"capability-down-keeps-column", "Right Down Down Down Up Up Up Up"},
		{"capability-down-keeps-column-right", "Right Right Down Down Down Up Up Up Left Down"},
		{"capability-up-from-first-row", "Right Up Up Right Up"},
		{"down-out-of-capabilities", "Down Down Down Down"},
		{"down-out-of-capabilities-from-no", "Right Right Down Down Down Up"},
		{"up-into-capabilities", "Down Down Down Up Up"},
		{"up-into-capabilities-from-protocol-last", "Down Down Down Down Down Down Down Up Up Up Up Up Up"},
		{"left-right-protocol", "Down Down Down Down Left Right Right Down Left"},
		{"left-right-video", dn(11) + "Left Right Left Right"},
		{"left-right-buttons", dn(12) + "Left Right Right Right Left Left Left"},
		{"up-from-buttons", dn(12) + "Right Up Down Down"},
		{"up-from-ok", dn(12) + "Up"},
		{"down-from-cancel", dn(12) + "Right Down Down"},
		{"tab-walk", strings.Repeat("Tab ", 8)},
		{"backtab-walk", strings.Repeat("Backtab ", 8)},
		{"tab-from-middle-of-groups", "Right Right Tab Right Right Tab Down Tab Down Down Tab Down Tab"},
		{"backtab-from-middle-of-groups", "Right Tab Down Down Backtab Backtab Backtab"},
		{"tab-from-buttons", dn(12) + "Tab Tab Backtab"},
		{"space-capabilities", "Right Space Down Right Space Down Right Space"},
		{"enter-capabilities", "Right Enter Down Right Enter Down Right Enter"},
		{"space-kitty-yes-implies-nothing", "Down Right Space Down Right Space Up Left Space"},
		{"space-placeholder-yes-sets-kitty", "Down Down Right Space Up Left Space"},
		{"space-kitty-no-drops-placeholder-yes", "Down Down Right Space Up Right Right Space"},
		{"space-protocol", "Down Down Down Down Space Down Space Down Space Down Space"},
		{"enter-protocol", "Down Down Down Down Enter Down Enter Down Enter Down Enter"},
		{"space-metadata", dn(7) + "Space Down Space Down Space Down Space Down Space"},
		{"enter-metadata", dn(7) + "Enter Down Enter Down Enter Down Enter"},
		{"space-video", dn(11) + "Space Space Space"},
		{"enter-video", dn(11) + "Enter Enter"},
		{"alt-capability-letters", "Alt+s Alt+s Alt+s Alt+s Alt+k Alt+k Alt+k Alt+p Alt+p Alt+p"},
		{"alt-capability-letters-upper", "Alt+S Alt+K Alt+P Alt+K Alt+P"},
		{"alt-kitty-after-placeholder-yes", "Alt+p Alt+k Alt+k Alt+k"},
		{"alt-protocol-letters", "Alt+a Alt+i Alt+t Alt+z Alt+a"},
		{"alt-metadata-letters", "Alt+f Alt+b Alt+e Alt+u Alt+f"},
		{"alt-video", "Alt+v Alt+v Alt+V"},
		{"alt-then-nav", "Alt+z Up Alt+u Down Alt+s Left"},
		{"alt-unknown", "Alt+x Alt+q Alt+g Down"},
		{"alt-mnemonic-from-buttons", dn(12) + "Right Alt+t Alt+s"},
		{"plain-letters-ignored", "s k p a i t z f b e u v x q"},
		{"plain-o-applies", "Right Space o"},
		{"plain-c-cancels", "Right Space c"},
		{"plain-upper-o", "O"},
		{"plain-upper-c", "C"},
		{"plain-o-after-radio-edit", "Down Down Down Down Down Space o"},
		{"alt-o-applies", "Alt+z Alt+o"},
		{"alt-c-cancels", "Alt+z Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"enter-ok", dn(12) + "Enter"},
		{"enter-cancel", dn(12) + "Right Enter"},
		{"space-ok", "Alt+z " + dn(12) + "Space"},
		{"space-cancel", "Alt+z " + dn(12) + "Right Space"},
		{"esc", "Alt+z Esc"},
		{"esc-from-button", dn(12) + "Right Esc"},
		{"apply-all-changes", "Right Space Down Right Space Down Down Down Down Down Space Down Down Down Down Space Down Space Down Enter"},
		{"f5-detect-plain", "Right Space F5"},
		{"unknown-keys", "Home End PgUp PgDn Insert Delete Backspace"},
	})
	kitty := previewKeyDialog("preview_settings_kitty", map[string]string{"TERM_PROGRAM": "kitty"})
	runKeyGoldenScripts(t, kitty, []simpleKeyScript{
		{"f5-detect", "Right Space Down Down F5"},
		{"f5-then-protocol-untouched", "Alt+z Alt+u Alt+v F5 Down"},
		{"f5-then-apply", "F5 Alt+o"},
	})
	ghostty := previewKeyDialog("preview_settings_wezterm", map[string]string{"TERM_PROGRAM": "wezterm"})
	runKeyGoldenScripts(t, ghostty, []simpleKeyScript{
		{"f5-detect", "F5"},
	})
}
