package keymap

import "testing"

func TestDefaultCopyMenuKeysUnique(t *testing.T) {
	if err := validateCopyMenuKeys(DefaultCopyMenuKeys()); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultCopyMenuKeysLetters(t *testing.T) {
	keys := DefaultCopyMenuKeys()
	if keys[ActionClipboardCopyFileURL] != "c" ||
		keys[ActionClipboardCopyDirURL] != "d" ||
		keys[ActionClipboardCopyFilename] != "f" ||
		keys[ActionClipboardCopyFilenameWithoutExt] != "n" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestBuildCopyMenuEntriesOrder(t *testing.T) {
	entries := BuildCopyMenuEntries(DefaultCopyMenuKeys())
	if len(entries) != 4 {
		t.Fatalf("len = %d, want 4", len(entries))
	}
	want := []string{
		ActionClipboardCopyFileURL,
		ActionClipboardCopyDirURL,
		ActionClipboardCopyFilename,
		ActionClipboardCopyFilenameWithoutExt,
	}
	for i, id := range want {
		if entries[i].ActionID != id {
			t.Fatalf("[%d] = %q, want %q", i, entries[i].ActionID, id)
		}
	}
}

func TestValidateCopyMenuKeysRejectsDuplicate(t *testing.T) {
	keys := DefaultCopyMenuKeys()
	keys[ActionClipboardCopyDirURL] = "c"
	if err := validateCopyMenuKeys(keys); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestDefaultBundleCopyMenuKey(t *testing.T) {
	b, err := DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.CopyMenuKey) != 4 {
		t.Fatalf("CopyMenuKey len = %d, want 4", len(b.CopyMenuKey))
	}
}
