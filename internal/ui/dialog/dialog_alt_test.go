package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestTryDialogMnemonicsExtrasOnly(t *testing.T) {
	t.Parallel()
	ran := 0
	extras := []ExtraMnemonic{{Rune: 'p', Fn: func() { ran++ }}}
	if !TryDialogMnemonics(tcell.NewEventKey(tcell.KeyRune, 'P', tcell.ModAlt), extras) || ran != 1 {
		t.Fatal("Alt+P should run the extra")
	}
	if !TryDialogMnemonics(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModMeta), extras) || ran != 2 {
		t.Fatal("Meta+p should run the extra (Meta as Alt)")
	}
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt),
		tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt),
	} {
		if TryDialogMnemonics(ev, extras) {
			t.Fatalf("%v must not be handled (OK/Cancel are resolved centrally)", ev.Name())
		}
	}
}
