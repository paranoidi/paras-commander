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
		keys[ActionClipboardCopyFilenameWithoutExt] != "n" ||
		keys[ActionClipboardGitMenu] != "g" ||
		keys[ActionClipboardCopyGitFilePath] != "c" ||
		keys[ActionClipboardCopyGitDirPath] != "d" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestBuildCopyMenuEntriesOrder(t *testing.T) {
	entries := BuildCopyMenuEntries(DefaultCopyMenuKeys())
	if len(entries) != 5 {
		t.Fatalf("len = %d, want 5", len(entries))
	}
	want := []string{
		ActionClipboardCopyFileURL,
		ActionClipboardCopyDirURL,
		ActionClipboardCopyFilename,
		ActionClipboardCopyFilenameWithoutExt,
		ActionClipboardGitMenu,
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
	if len(b.CopyMenuKey) != 7 {
		t.Fatalf("CopyMenuKey len = %d, want 7", len(b.CopyMenuKey))
	}
	git := b.CopyMenuGitEntries()
	if len(git) != 2 || git[0].ActionID != ActionClipboardCopyGitFilePath || git[0].Label != "Copy file URL" ||
		git[1].ActionID != ActionClipboardCopyGitDirPath || git[1].Label != "Copy directory URL" {
		t.Fatalf("git entries = %+v", git)
	}
}

func TestValidateCopyMenuKeysPerScope(t *testing.T) {
	keys := DefaultCopyMenuKeys() // c and d appear in both scopes
	if err := validateCopyMenuKeys(keys); err != nil {
		t.Fatalf("cross-scope reuse should be valid: %v", err)
	}
	keys[ActionClipboardCopyGitDirPath] = "c"
	if err := validateCopyMenuKeys(keys); err == nil {
		t.Fatal("duplicate within git scope should error")
	}
	keys = DefaultCopyMenuKeys()
	keys[ActionClipboardGitMenu] = "c"
	if err := validateCopyMenuKeys(keys); err == nil {
		t.Fatal("duplicate within top scope should error")
	}
}
