package dialog_test

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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

func TestMetaDialogView_noticeReplacesEmptyList(t *testing.T) {
	notice := `meta: ignoring untrusted file "/home/walnut/.config/pc/meta.toml": group-writable by a shared group (chmod g-w)`
	spec := dialog.MetaDialogView(dialog.MetaDialogState{Open: true, Notice: notice})
	items := spec.Groups[0].Columns[0]
	if len(items) < 2 {
		t.Fatalf("notice rows = %d, want it wrapped over several rows", len(items))
	}
	var joined []string
	for _, it := range items {
		if it.Kind != dlg.KindLabel || it.Style != dlg.TextWarn {
			t.Fatalf("row %+v, want a TextWarn label", it)
		}
		joined = append(joined, it.Label)
	}
	if got := strings.Join(joined, " "); !strings.Contains(got, "chmod g-w") {
		t.Fatalf("wrapped notice %q lost its tail", got)
	}

	spec = dialog.MetaDialogView(dialog.MetaDialogState{
		Open: true, Notice: notice, Entries: []dialog.MetaEntry{{Name: "cedar"}}, Checked: []bool{false},
	})
	if items := spec.Groups[0].Columns[0]; len(items) != 1 || items[0].Kind != dlg.KindCheckbox {
		t.Fatalf("with entries the notice must not render, got %+v", items)
	}
}
