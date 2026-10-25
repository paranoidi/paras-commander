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
		name: name, serial: true,
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
