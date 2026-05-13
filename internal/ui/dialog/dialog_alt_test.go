package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestAltDialogOKCancel(t *testing.T) {
	t.Parallel()
	if !AltDialogOK(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt)) {
		t.Fatal("Alt+o")
	}
	if !AltDialogOK(tcell.NewEventKey(tcell.KeyRune, 'O', tcell.ModAlt)) {
		t.Fatal("Alt+O")
	}
	if AltDialogOK(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone)) {
		t.Fatal("plain o")
	}
	if !AltDialogCancel(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt)) {
		t.Fatal("Alt+c")
	}
}
