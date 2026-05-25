package dialog_test

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func TestUserMenuEntryShortcuts_configuredAndDynamic(t *testing.T) {
	entries := []usermenu.MenuEntry{
		{Key: "g", Title: "lazygit"},
		{Title: "Print working directory"},
		{Title: "Open folder"},
	}
	got := dialog.UserMenuEntryShortcuts(entries)
	if got[0] != 'g' {
		t.Fatalf("configured g: got %q", got[0])
	}
	if got[1] != 'p' {
		t.Fatalf("dynamic print: got %q, want p", got[1])
	}
	if got[2] != 'f' {
		t.Fatalf("open folder after p taken: got %q, want f (second word)", got[2])
	}
}

func TestUserMenuEntryShortcuts_reservesCancel(t *testing.T) {
	entries := []usermenu.MenuEntry{{Title: "Copy"}}
	got := dialog.UserMenuEntryShortcuts(entries)
	if got[0] != 'p' {
		t.Fatalf("copy shortcut = %q, want p (c reserved)", got[0])
	}
	if _, ok := dialog.UserMenuEntryIndexForAltShortcut(entries, 'c'); ok {
		t.Fatal("Alt+c must stay reserved for Cancel")
	}
}
