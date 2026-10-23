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

func TestFindDialogNavFocusKey(t *testing.T) {
	t.Parallel()
	type row struct {
		focus                 int
		hasSelectionsCheckbox bool
		key                   tcell.Key
		wantF                 int
		wantH                 bool
	}
	for _, tt := range []row{
		{0, false, tcell.KeyTab, 1, true},
		{1, false, tcell.KeyTab, 2, true},
		{7, false, tcell.KeyTab, 0, true},
		{0, false, tcell.KeyDown, 0, false},
		{1, false, tcell.KeyLeft, 0, true},
		{1, false, tcell.KeyRight, 1, false},
		{1, false, tcell.KeyUp, 0, true},
		{1, false, tcell.KeyDown, 2, true},
		{2, false, tcell.KeyRight, 3, true},
		{3, false, tcell.KeyRight, 4, true},
		{4, false, tcell.KeyRight, 5, true},
		{5, false, tcell.KeyLeft, 4, true},
		{3, false, tcell.KeyLeft, 2, true},
		{2, false, tcell.KeyLeft, 2, false},
		{2, false, tcell.KeyUp, 1, true},
		{5, false, tcell.KeyUp, 1, true},
		{2, false, tcell.KeyDown, 6, true},
		{5, false, tcell.KeyDown, 6, true},
		{6, false, tcell.KeyUp, 5, true},
		{7, false, tcell.KeyUp, 5, true},
		{0, true, tcell.KeyTab, 1, true},
		{8, true, tcell.KeyTab, 0, true},
		{2, true, tcell.KeyDown, 6, true},
		{5, true, tcell.KeyDown, 6, true},
		{6, true, tcell.KeyDown, 7, true},
		{6, true, tcell.KeyUp, 5, true},
		{7, true, tcell.KeyUp, 6, true},
	} {
		gotF, gotH := FindDialogNavFocusKey(tt.focus, tt.hasSelectionsCheckbox, tt.key)
		if gotF != tt.wantF || gotH != tt.wantH {
			t.Fatalf("focus=%d sel=%v key=%v: got (%d,%v) want (%d,%v)",
				tt.focus, tt.hasSelectionsCheckbox, tt.key, gotF, gotH, tt.wantF, tt.wantH)
		}
	}
}

func TestListClampedSelectionDelta(t *testing.T) {
	t.Parallel()
	if g := ListClampedSelectionDelta(3, 10, 1); g != 4 {
		t.Fatalf("increment: got %d", g)
	}
	if g := ListClampedSelectionDelta(9, 10, 5); g != 9 {
		t.Fatalf("clamp high: got %d", g)
	}
	if g := ListClampedSelectionDelta(0, 10, -1); g != 0 {
		t.Fatalf("clamp low: got %d", g)
	}
	if g := ListClampedSelectionDelta(5, 0, 1); g != 0 {
		t.Fatalf("empty list: got %d", g)
	}
}

func TestFindDialogFocusHelpersMatchNav(t *testing.T) {
	t.Parallel()
	s := FindDialogState{ShowSearchSelectionsOption: true}
	if s.FindDialogDepthFocus() != 1 || s.FindDialogOnlyDirsFocus() != 2 || s.FindDialogSelectionsFocus() != 6 ||
		s.FindDialogOKFocus() != 7 || s.FindDialogCancelFocus() != 8 {
		t.Fatalf("unexpected focus order: %d %d %d %d %d", s.FindDialogDepthFocus(), s.FindDialogOnlyDirsFocus(),
			s.FindDialogSelectionsFocus(), s.FindDialogOKFocus(), s.FindDialogCancelFocus())
	}
}
