package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestRenameFileWithTrailingSpace(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "orchard ")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanRename(localfs.Entry{Name: "orchard ", Path: src, Type: localfs.EntryFile}, "harbor ", dir)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourcePath != src {
		t.Fatalf("source %q, want %q", plan.SourcePath, src)
	}
	if err := ExecuteRename(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "harbor ")); err != nil {
		t.Fatal(err)
	}
}
