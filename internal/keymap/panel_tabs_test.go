package keymap

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/config"
)

func TestPanelTabActionsCoverEverySlot(t *testing.T) {
	if len(PanelTabActions) != config.TabSlots {
		t.Fatalf("%d tab actions, want config.TabSlots = %d", len(PanelTabActions), config.TabSlots)
	}
}

// Extended-key terminals report Ctrl+digit as the digit rune with Ctrl held; Ctrl+W is a control
// key in every encoding.
func TestPanelTabKeysResolveFromExtendedKeyEvents(t *testing.T) {
	m, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range PanelTabActions {
		ev := tcell.NewEventKey(tcell.KeyRune, rune('1'+i), tcell.ModCtrl)
		if id, ok := m.Lookup(ev); !ok || id != want {
			t.Errorf("C-%d = %q %v, want %q", i+1, id, ok, want)
		}
	}
	for name, ev := range map[string]*tcell.EventKey{
		"KeyCtrlW":       tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone),
		"KeyCtrlW+ctrl":  tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModCtrl),
		"rune w + ctrl":  tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModCtrl),
		"rune 0x17 ctrl": tcell.NewEventKey(tcell.KeyRune, 0x17, tcell.ModCtrl),
	} {
		if id, ok := m.Lookup(ev); !ok || id != ActionPanelCloseTab {
			t.Errorf("%s = %q %v, want %q", name, id, ok, ActionPanelCloseTab)
		}
	}
}

func TestPanelTabKeysDoNotShadowEachOther(t *testing.T) {
	m, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	// Plain digits and Alt+digits keep their own meaning.
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModAlt),
	} {
		if id, ok := m.Lookup(ev); ok && (id == PanelTabActions[0] || id == ActionPanelCloseTab) {
			t.Errorf("%v resolves to %q", ev.Modifiers(), id)
		}
	}
}

func TestCtrlWStillDeletesWordInDialogInputs(t *testing.T) {
	m, err := Build(DefaultDialogInputOverlayKeys())
	if err != nil {
		t.Fatal(err)
	}
	ev := tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone)
	if id, ok := m.Lookup(ev); !ok || id != ActionDialogInputKillWordBackward {
		t.Fatalf("dialog C-w = %q %v, want %q", id, ok, ActionDialogInputKillWordBackward)
	}
}
