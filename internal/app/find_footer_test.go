package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestFindDialogFooterPairsUnselectAsShiftWord(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][3]string{}
	for _, fk := range findDialogMarkingFooterKeys(bundle.FindDialog) {
		got[fk.KeyLabel] = [3]string{fk.Hint, fk.HintShiftPrefix, fk.ShiftKeyLabel}
	}
	want := map[string][3]string{
		"F4": {"Edit", "", ""},
		"F5": {"Select all", "Unselect", "S-F5"},
		"F6": {"Select group", "Unselect", "S-F6"},
	}
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s = %q, want %q", key, got[key], w)
		}
	}
	for key, g := range got {
		if g[0] == "Unselect all" || g[0] == "Unselect group" {
			t.Errorf("%s: unselect still has an entry of its own", key)
		}
	}
}
