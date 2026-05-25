package dialog_test

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestMetaEntryShortcuts_noneAndNames(t *testing.T) {
	entries := []dialog.MetaEntry{
		{Name: "none", Description: "None (clear)"},
		{Name: "walnut"},
		{Name: "cedar"},
	}
	got := dialog.MetaEntryShortcuts(entries)
	want := []rune{'n', 'w', 'e'}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("shortcuts[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMetaEntryShortcuts_reservesOKCancel(t *testing.T) {
	entries := []dialog.MetaEntry{
		{Name: "copy"},
		{Name: "open"},
		{Name: "count"},
	}
	got := dialog.MetaEntryShortcuts(entries)
	// copy: c/o reserved -> p; open: o reserved, p taken -> e; count: c/o -> u
	want := []rune{'p', 'e', 'u'}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("shortcuts[%d] = %q, want %q (all %q)", i, got[i], want[i], got)
		}
	}
}

func TestMetaEntryShortcuts_diskSizeUsesSecondWord(t *testing.T) {
	entries := []dialog.MetaEntry{
		{Name: "disk-size", Description: "Disk usage"},
	}
	got := dialog.MetaEntryShortcuts(entries)
	if len(got) != 1 || got[0] != 'd' {
		t.Fatalf("shortcuts = %q, want d (first letter of disk)", got)
	}
}

func TestMetaEntryShortcuts_duplicateFirstLetters(t *testing.T) {
	entries := []dialog.MetaEntry{
		{Name: "night"},
		{Name: "noble"},
	}
	got := dialog.MetaEntryShortcuts(entries)
	if got[0] != 'n' {
		t.Fatalf("night shortcut = %q, want n", got[0])
	}
	if got[1] != 'b' {
		t.Fatalf("noble shortcut = %q, want b (n taken, o reserved for OK)", got[1])
	}
}

func TestMetaEntryIndexForAltShortcut(t *testing.T) {
	entries := []dialog.MetaEntry{
		{Name: "none"},
		{Name: "walnut"},
		{Name: "cedar"},
	}
	if i, ok := dialog.MetaEntryIndexForAltShortcut(entries, 'N'); !ok || i != 0 {
		t.Fatalf("Alt+n: i=%d ok=%v, want 0 true", i, ok)
	}
	if i, ok := dialog.MetaEntryIndexForAltShortcut(entries, 'w'); !ok || i != 1 {
		t.Fatalf("Alt+w: i=%d ok=%v, want 1 true", i, ok)
	}
	if i, ok := dialog.MetaEntryIndexForAltShortcut(entries, 'e'); !ok || i != 2 {
		t.Fatalf("Alt+e: i=%d ok=%v, want 2 true", i, ok)
	}
	if _, ok := dialog.MetaEntryIndexForAltShortcut(entries, 'z'); ok {
		t.Fatal("Alt+z should not match")
	}
	if _, ok := dialog.MetaEntryIndexForAltShortcut(entries, 'o'); ok {
		t.Fatal("Alt+o must stay reserved for OK")
	}
	if _, ok := dialog.MetaEntryIndexForAltShortcut(entries, 'c'); ok {
		t.Fatal("Alt+c must stay reserved for Cancel")
	}
}
