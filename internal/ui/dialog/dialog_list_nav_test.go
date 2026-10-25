package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestListOKCancelNavFocusKey(t *testing.T) {
	t.Parallel()
	type row struct {
		focus int
		key   tcell.Key
		wantF int
		wantH bool
	}
	for _, tt := range []row{
		{0, tcell.KeyTab, 1, true},
		{2, tcell.KeyTab, 0, true},
		{1, tcell.KeyBacktab, 0, true},
		{1, tcell.KeyLeft, 0, true},
		{2, tcell.KeyLeft, 1, true},
		{1, tcell.KeyRight, 2, true},
		{2, tcell.KeyRight, 2, false},
		{2, tcell.KeyUp, 0, true},
		{1, tcell.KeyDown, 2, true},
		{0, tcell.KeyDown, 0, false},
		{0, tcell.KeyUp, 0, false},
	} {
		gotF, gotH := ListOKCancelNavFocusKey(tt.focus, tt.key)
		if gotF != tt.wantF || gotH != tt.wantH {
			t.Fatalf("focus=%d key=%v: got (%d,%v) want (%d,%v)", tt.focus, tt.key, gotF, gotH, tt.wantF, tt.wantH)
		}
	}
}
