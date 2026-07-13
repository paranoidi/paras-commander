package subshell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestEncodeKey(t *testing.T) {
	tests := []struct {
		name      string
		key       tcell.Key
		ch        rune
		mod       tcell.ModMask
		appCursor bool
		want      string
	}{
		{"plain rune", tcell.KeyRune, 'w', tcell.ModNone, false, "w"},
		{"alt rune", tcell.KeyRune, 'w', tcell.ModAlt, false, "\x1bw"},
		{"unicode rune", tcell.KeyRune, '日', tcell.ModNone, false, "日"},

		{"ctrl c", tcell.KeyCtrlC, 0, tcell.ModCtrl, false, "\x03"},
		{"alt ctrl letter", tcell.KeyCtrlC, 0, tcell.ModCtrl | tcell.ModAlt, false, "\x1b\x03"},

		{"enter", tcell.KeyEnter, 0, tcell.ModNone, false, "\r"},
		{"tab", tcell.KeyTab, 0, tcell.ModNone, false, "\t"},
		{"backspace", tcell.KeyBackspace, 0, tcell.ModNone, false, "\x7f"},
		{"backspace2", tcell.KeyBackspace2, 0, tcell.ModNone, false, "\x7f"},
		{"esc", tcell.KeyEsc, 0, tcell.ModNone, false, "\x1b"},

		{"up normal", tcell.KeyUp, 0, tcell.ModNone, false, "\x1b[A"},
		{"up appcursor", tcell.KeyUp, 0, tcell.ModNone, true, "\x1bOA"},
		{"down normal", tcell.KeyDown, 0, tcell.ModNone, false, "\x1b[B"},
		{"down appcursor", tcell.KeyDown, 0, tcell.ModNone, true, "\x1bOB"},
		{"right normal", tcell.KeyRight, 0, tcell.ModNone, false, "\x1b[C"},
		{"right appcursor", tcell.KeyRight, 0, tcell.ModNone, true, "\x1bOC"},
		{"left normal", tcell.KeyLeft, 0, tcell.ModNone, false, "\x1b[D"},
		{"left appcursor", tcell.KeyLeft, 0, tcell.ModNone, true, "\x1bOD"},

		{"ctrl right", tcell.KeyRight, 0, tcell.ModCtrl, false, "\x1b[1;5C"},

		{"home normal", tcell.KeyHome, 0, tcell.ModNone, false, "\x1b[H"},
		{"home appcursor", tcell.KeyHome, 0, tcell.ModNone, true, "\x1bOH"},
		{"end normal", tcell.KeyEnd, 0, tcell.ModNone, false, "\x1b[F"},
		{"end appcursor", tcell.KeyEnd, 0, tcell.ModNone, true, "\x1bOF"},

		{"delete", tcell.KeyDelete, 0, tcell.ModNone, false, "\x1b[3~"},
		{"shift delete", tcell.KeyDelete, 0, tcell.ModShift, false, "\x1b[3;2~"},
		{"pgup", tcell.KeyPgUp, 0, tcell.ModNone, false, "\x1b[5~"},
		{"shift pgup", tcell.KeyPgUp, 0, tcell.ModShift, false, "\x1b[5;2~"},

		{"f1", tcell.KeyF1, 0, tcell.ModNone, false, "\x1bOP"},
		{"f5", tcell.KeyF5, 0, tcell.ModNone, false, "\x1b[15~"},
		{"f12", tcell.KeyF12, 0, tcell.ModNone, false, "\x1b[24~"},
		{"shift f5", tcell.KeyF5, 0, tcell.ModShift, false, "\x1b[15;2~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := tcell.NewEventKey(tt.key, tt.ch, tt.mod)
			got := EncodeKey(ev, tt.appCursor)
			if string(got) != tt.want {
				t.Errorf("EncodeKey(%v, appCursor=%v) = %q, want %q", tt.key, tt.appCursor, got, tt.want)
			}
		})
	}
}

func TestEncodeKeyNoEncoding(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyF60, 0, tcell.ModNone)
	if got := EncodeKey(ev, false); got != nil {
		t.Errorf("EncodeKey(F60) = %q, want nil", got)
	}
}
