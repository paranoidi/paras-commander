package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestFilterArchiveEntries(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "a.zip", Path: "/a.zip", Type: localfs.EntryFile},
		{Name: "b.txt", Path: "/b.txt", Type: localfs.EntryFile},
		{Name: "c", Path: "/c", Type: localfs.EntryDirectory},
	}
	paths, skipped := FilterArchiveEntries(entries)
	if len(paths) != 1 || paths[0] != "/a.zip" || skipped != 2 {
		t.Fatalf("paths=%v skipped=%d", paths, skipped)
	}
}

func TestPlanExtractRequiresDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.zip")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := archive.Toolchain{Unzip: "/bin/unzip"}
	_, _, err := PlanExtract([]string{file}, filepath.Join(dir, "missing"), tc)
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestExecuteExtractTarGz(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not in PATH")
	}
	srcDir := t.TempDir()
	destDir := t.TempDir()
	inner := filepath.Join(srcDir, "hello.txt")
	if err := os.WriteFile(inner, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(srcDir, "pack.tar.gz")
	cmd := exec.Command("tar", "-czf", archivePath, "-C", srcDir, "hello.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	tc := archive.ProbeToolchain()
	plan, _, err := PlanExtract([]string{archivePath}, destDir, tc)
	if err != nil {
		t.Fatal(err)
	}
	done, err := ExecuteExtract(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("ExecuteExtract: %v", err)
	}
	if done != 1 {
		t.Fatalf("done = %d, want 1", done)
	}
	got := filepath.Join(destDir, "hello.txt")
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("extracted file: %v", err)
	}
}
