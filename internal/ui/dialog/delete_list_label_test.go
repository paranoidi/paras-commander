package dialog

import (
	"path/filepath"
	"testing"
)

func TestDeleteListEntryNameInCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	panel := filepath.Join(dir, "Some Series")
	entry := filepath.Join(panel, "Season 01")
	got := DeleteListEntryName(panel, "", entry, "Season 01")
	if got != "Season 01" {
		t.Fatalf("DeleteListEntryName = %q, want basename only", got)
	}
}

func TestDeleteListEntryNameOffPanelShowsAncestorPath(t *testing.T) {
	dir := t.TempDir()
	panel := filepath.Join(dir, "Other")
	entry := filepath.Join(dir, "Some Series", "Season 01")
	want := filepath.Join("Some Series", "Season 01")
	got := DeleteListEntryName(panel, "", entry, "Season 01")
	if got != want {
		t.Fatalf("DeleteListEntryName = %q, want %q", got, want)
	}
}

func TestDeleteListEntryNameWithHomeDirStillShowsContext(t *testing.T) {
	home := filepath.Join(t.TempDir(), "user")
	panel := filepath.Join(home, "Other")
	entry := filepath.Join(home, "Some Series", "Season 01")
	want := filepath.Join("Some Series", "Season 01")
	got := DeleteListEntryName(panel, home, entry, "Season 01")
	if got != want {
		t.Fatalf("DeleteListEntryName = %q, want %q", got, want)
	}
}

func TestDeleteListEntryNameFitsWidthShortensContextualPath(t *testing.T) {
	label := filepath.Join("Some Series", "Season 01")
	entry := filepath.Join("/tmp", label)
	narrow := DeleteListEntryNameFitsWidth(label, entry, 12)
	if narrow == label {
		t.Fatalf("expected shortened label, got %q", narrow)
	}
	if DeleteListEntryNameFitsWidth("readme.txt", "/tmp/readme.txt", 8) != "readme.txt" {
		t.Fatal("basename in current dir should not be path-fitted")
	}
}
