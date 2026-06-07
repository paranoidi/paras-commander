package ops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestValidateCopyHereSourceRequiresSingleDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &panel.State{
		Entries: []localfs.Entry{
			{Name: "sub", Path: sub, Type: localfs.EntryDirectory},
			{Name: "file.txt", Path: file, Type: localfs.EntryFile},
		},
	}
	p.SelectedPaths = map[string]bool{sub: true, file: true}

	if _, err := ValidateCopyHereSource(p); err == nil {
		t.Fatal("ValidateCopyHereSource() error = nil, want error for multiple selections")
	}

	p.SelectedPaths = map[string]bool{file: true}
	if _, err := ValidateCopyHereSource(p); err == nil {
		t.Fatal("ValidateCopyHereSource() error = nil, want error for file")
	}

	p.SelectedPaths = map[string]bool{sub: true}
	entry, err := ValidateCopyHereSource(p)
	if err != nil {
		t.Fatalf("ValidateCopyHereSource() error = %v", err)
	}
	if entry.Path != sub {
		t.Fatalf("entry path = %q, want %q", entry.Path, sub)
	}
}

func TestPlanCopyHereRejectsSameName(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := localfs.Entry{Name: "sub", Path: sub, Type: localfs.EntryDirectory}
	if _, err := PlanCopyHere(entry, "sub", dir); err == nil {
		t.Fatal("PlanCopyHere() error = nil, want error for unchanged name")
	}
}
