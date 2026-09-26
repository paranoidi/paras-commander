package keymap

import "testing"

func TestDefaultPreviewMenuKeysUnique(t *testing.T) {
	if err := validatePreviewMenuKeys(DefaultPreviewMenuKeys()); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultPreviewMenuKeysLetters(t *testing.T) {
	keys := DefaultPreviewMenuKeys()
	want := map[string]string{
		ActionPreviewThemePicker:  "t",
		ActionPreviewToggleRaw:    "r",
		ActionPreviewReload:       "R",
		ActionPreviewSearchStart:  "s",
		ActionPreviewDiffNextHunk: "n",
		ActionPreviewDiffPrevHunk: "p",
		ActionFileEdit:            "e",
		ActionFileDelete:          "d",
		ActionAppQuit:             "q",
	}
	for action, key := range want {
		if got := keys[action]; got != key {
			t.Fatalf("keys[%q] = %q, want %q", action, got, key)
		}
	}
	if len(keys) != len(want) {
		t.Fatalf("keys len = %d, want %d (got %v)", len(keys), len(want), keys)
	}
}

func TestBuildPreviewMenuEntriesOrder(t *testing.T) {
	entries := BuildPreviewMenuEntries(DefaultPreviewMenuKeys())
	if len(entries) != 9 {
		t.Fatalf("len = %d, want 9", len(entries))
	}
	want := []string{
		ActionPreviewThemePicker,
		ActionPreviewToggleRaw,
		ActionPreviewReload,
		ActionPreviewSearchStart,
		ActionPreviewDiffNextHunk,
		ActionPreviewDiffPrevHunk,
		ActionFileEdit,
		ActionFileDelete,
		ActionAppQuit,
	}
	for i, id := range want {
		if entries[i].ActionID != id {
			t.Fatalf("[%d] = %q, want %q", i, entries[i].ActionID, id)
		}
	}
}

func TestPreviewMenuKeysMergeOverrideAndOmit(t *testing.T) {
	user := map[string]string{
		ActionPreviewThemePicker: "z",
		ActionFileDelete:         "",
	}
	merged := mergePreviewMenuKeys(DefaultPreviewMenuKeys(), user)
	if merged[ActionPreviewThemePicker] != "z" {
		t.Fatalf("theme picker key = %q, want z", merged[ActionPreviewThemePicker])
	}
	if _, ok := merged[ActionFileDelete]; ok {
		t.Fatalf("delete should be omitted, still in map: %v", merged[ActionFileDelete])
	}
}

func TestValidatePreviewMenuKeysRejectsDuplicate(t *testing.T) {
	keys := DefaultPreviewMenuKeys()
	keys[ActionFileEdit] = "d"
	if err := validatePreviewMenuKeys(keys); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestValidatePreviewMenuKeysRejectsNonLetter(t *testing.T) {
	keys := map[string]string{ActionFileEdit: "1"}
	if err := validatePreviewMenuKeys(keys); err == nil {
		t.Fatal("expected non-letter key error")
	}
}

func TestDefaultBundlePreviewMenuKey(t *testing.T) {
	b, err := DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.PreviewMenuKey) != 9 {
		t.Fatalf("PreviewMenuKey len = %d, want 9", len(b.PreviewMenuKey))
	}
	entries := b.PreviewMenuEntries()
	if len(entries) != 9 {
		t.Fatalf("PreviewMenuEntries() len = %d, want 9", len(entries))
	}
}

func TestFilePreviewOverlayMapsColonToPreviewMenu(t *testing.T) {
	keys := DefaultFilePreviewOverlayKeys()
	chords, ok := keys[ActionPreviewMenu]
	if !ok || len(chords) != 1 || chords[0] != ":" {
		t.Fatalf("DefaultFilePreviewOverlayKeys()[ActionPreviewMenu] = %v, want [\":\"]", chords)
	}
}

func TestActionForPreviewMenuKey(t *testing.T) {
	b, err := DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	if id, ok := b.ActionForPreviewMenuKey('e'); !ok || id != ActionFileEdit {
		t.Fatalf("ActionForPreviewMenuKey('e') = (%q, %v), want (%q, true)", id, ok, ActionFileEdit)
	}
	var nilBundle *Bundle
	if _, ok := nilBundle.ActionForPreviewMenuKey('e'); ok {
		t.Fatal("nil bundle ActionForPreviewMenuKey should return ok=false")
	}
}

func TestFilePreviewOverlayMapsQToClose(t *testing.T) {
	keys := DefaultFilePreviewOverlayKeys()
	chords, ok := keys[ActionPreviewClose]
	if !ok || len(chords) != 1 || chords[0] != "q" {
		t.Fatalf("DefaultFilePreviewOverlayKeys()[ActionPreviewClose] = %v, want [\"q\"]", chords)
	}
}
