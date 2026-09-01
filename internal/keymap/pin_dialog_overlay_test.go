package keymap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestAllowedInPinDialogOverlayRejectsForeignActions(t *testing.T) {
	if AllowedInPinDialogOverlay(ActionBookmarkDelete) {
		t.Fatal("AllowedInPinDialogOverlay should reject foreign actions")
	}
	if !AllowedInPinDialogOverlay(ActionPinOpenInPrimary) {
		t.Fatal("AllowedInPinDialogOverlay should accept pin.open-primary")
	}
	if !AllowedInPinDialogOverlay(ActionPinOpenInSecondary) {
		t.Fatal("AllowedInPinDialogOverlay should accept pin.open-secondary")
	}
	if !AllowedInPinDialogOverlay(ActionPinRemove) {
		t.Fatal("AllowedInPinDialogOverlay should accept pin.remove")
	}
}

func TestPinDialogOverlayRejectsNonPinActions(t *testing.T) {
	dir := t.TempDir()
	keybindings := filepath.Join(dir, "keybindings.toml")
	body := "[dialog.pin]\n" +
		"jobs.cancel = [\"C-r\"]\n"
	if err := os.WriteFile(keybindings, []byte(body), 0o600); err != nil {
		t.Fatalf("write keybindings: %v", err)
	}
	_, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: keybindings})
	if err == nil {
		t.Fatal("LoadFromPaths: want error for invalid action in [dialog.pin]")
	}
}

func TestDefaultBundlePinDialogOverlayKeys(t *testing.T) {
	bundle, err := DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	tests := []struct {
		ev   *tcell.EventKey
		want string
	}{
		{tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift), ActionPinOpenInPrimary},
		{tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift), ActionPinOpenInSecondary},
		{tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone), ActionPinRemove},
	}
	for _, tt := range tests {
		id, ok := bundle.PinDialog.Lookup(tt.ev)
		if !ok || id != tt.want {
			t.Fatalf("PinDialog lookup %v = %q %v, want %q", tt.ev, id, ok, tt.want)
		}
	}
}

func TestPinDialogOverlayRoundTripFromKeybindingsFile(t *testing.T) {
	dir := t.TempDir()
	keybindings := filepath.Join(dir, "keybindings.toml")
	body := "[dialog.pin]\n" +
		"\"pin.remove\" = [\"delete\"]\n"
	if err := os.WriteFile(keybindings, []byte(body), 0o600); err != nil {
		t.Fatalf("write keybindings: %v", err)
	}
	bundle, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: keybindings})
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	id, ok := bundle.PinDialog.Lookup(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if !ok || id != ActionPinRemove {
		t.Fatalf("PinDialog delete = %q %v, want %q", id, ok, ActionPinRemove)
	}
	// F8 default is replaced, not merged, by the explicit override.
	if _, ok := bundle.PinDialog.Lookup(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)); ok {
		t.Fatal("F8 should no longer bind pin.remove after explicit [dialog.pin] override")
	}
}
