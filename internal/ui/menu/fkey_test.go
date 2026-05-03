package menu

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestFindItemByFKeyLabel(t *testing.T) {
	km, err := keymap.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	defs := BrowserDefinitions(km)
	if def, item, ok := FindItemByFKeyLabel(defs, "F5"); !ok || def.ID != TopFile || item.Action != keymap.ActionCopy {
		t.Fatalf("F5: ok=%v menu=%q item.Action=%q", ok, def.ID, item.Action)
	}
	if _, _, ok := FindItemByFKeyLabel(defs, "F9"); ok {
		t.Fatal("F9: should not match a menu KeyLabel")
	}
}
