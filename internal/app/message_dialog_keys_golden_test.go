package app

import (
	"fmt"
	"strings"
	"testing"
)

// Key-script goldens for the generic message dialog (harness of simple_dialog_keys_golden_test.go).
// The dialog has a single OK button; the transcript shows the dialog rect only, because the panels
// behind it show free disk space.

func messageKeyDialog(name, title, msg string, rx, ry, rw, rh int) keyGoldenDialog {
	return keyGoldenDialog{
		name: name, rx: rx, ry: ry, rw: rw, rh: rh,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			app.openMessageDialog(title, msg)
			return app
		},
		state: func(app *App) string {
			return fmt.Sprintf("title=%q", app.model.MessageDialog.Title)
		},
		open: func(app *App) bool { return app.model.MessageDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v message=%q", quit, app.model.Message)
		},
	}
}

func messageScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"enter", "Enter"},
		{"space", "Space Enter"},
		{"esc", "Esc"},
		{"alt-o", "Alt+o"},
		{"alt-upper-o", "Alt+O"},
		{"alt-c", "Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"alt-unknown", "Alt+x Alt+q Enter"},
		{"arrows-swallowed", "Down Up Left Right Enter"},
		{"tab-swallowed", "Tab Backtab Tab Enter"},
		{"scroll-keys-swallowed", "PgDn PgUp Home End Ctrl+Home Ctrl+End Enter"},
		{"plain-letters", "o c x y n q Enter"},
		{"function-keys", "F5 F8 Enter"},
		{"backspace-delete", "Backspace Delete Insert Enter"},
		{"tab-then-space", "Tab Space"},
		{"repeat-enter", "Enter Enter"},
	}
}

func TestMessageDialogKeyScripts(t *testing.T) {
	long := strings.Repeat("harbor lantern willow meadow ", 12)
	runKeyGoldenScripts(t, messageKeyDialog("message_dialog", "Extract", "lantern failed", 24, 8, 32, 8), messageScripts())
	runKeyGoldenScripts(t, messageKeyDialog("message_dialog_wrapped", "Theme failed", long, 2, 7, 76, 10), []simpleKeyScript{
		{"enter", "Enter"},
		{"esc", "Esc"},
		{"alt-o", "Alt+o"},
		{"swallowed", "Down PgDn Tab Space Enter"},
	})
	var tall []string
	for i := 0; i < 30; i++ {
		tall = append(tall, fmt.Sprintf("line %d willow", i))
	}
	runKeyGoldenScripts(t, messageKeyDialog("message_dialog_overflow", "Extract", strings.Join(tall, "\n"), 26, 1, 28, 22), []simpleKeyScript{
		{"enter", "Enter"},
		{"swallowed", "Down PgDn End Tab Enter"},
	})
}
