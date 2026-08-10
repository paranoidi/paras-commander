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
		ActionFileViewThemePicker:  "t",
		ActionFileViewToggleRaw:    "r",
		ActionFileViewReload:       "R",
		ActionFileViewSearchStart:  "s",
		ActionFileViewDiffNextHunk: "n",
		ActionFileViewDiffPrevHunk: "p",
		ActionFileEdit:             "e",
		ActionFileDelete:           "d",
		ActionAppQuit:              "q",
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
		ActionFileViewThemePicker,
		ActionFileViewToggleRaw,
		ActionFileViewReload,
		ActionFileViewSearchStart,
		ActionFileViewDiffNextHunk,
		ActionFileViewDiffPrevHunk,
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
		ActionFileViewThemePicker: "z",
		ActionFileDelete:          "",
	}
	merged := mergePreviewMenuKeys(DefaultPreviewMenuKeys(), user)
	if merged[ActionFileViewThemePicker] != "z" {
		t.Fatalf("theme picker key = %q, want z", merged[ActionFileViewThemePicker])
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
	chords, ok := keys[ActionFileViewMenu]
	if !ok || len(chords) != 1 || chords[0] != ":" {
		t.Fatalf("DefaultFilePreviewOverlayKeys()[ActionFileViewMenu] = %v, want [\":\"]", chords)
	}
}

func TestFilePreviewOverlayMapsQToClose(t *testing.T) {
	keys := DefaultFilePreviewOverlayKeys()
	chords, ok := keys[ActionFileViewClose]
	if !ok || len(chords) != 1 || chords[0] != "q" {
		t.Fatalf("DefaultFilePreviewOverlayKeys()[ActionFileViewClose] = %v, want [\"q\"]", chords)
	}
}
