package ops

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestPlanExtractSkipsExistingStreamOutput(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	archivePath := filepath.Join(srcDir, "meadow.gz")
	writeGzipFile(t, archivePath, "incoming-thicket")
	existing := filepath.Join(destDir, "meadow")
	if err := os.WriteFile(existing, []byte("keep-harbor"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, skipped, err := PlanExtract([]string{archivePath}, destDir, archive.Toolchain{Gzip: "/bin/gzip"})
	if err == nil {
		t.Fatal("expected plan error when the only stream target already exists")
	}
	if len(plan.Items) != 0 {
		t.Fatalf("items = %d, want none planned over an existing stream target", len(plan.Items))
	}
	if !skipMentions(skipped, "already exists") {
		t.Fatalf("skipped = %v, want already-exists reason", skipped)
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep-harbor" {
		t.Fatalf("existing output replaced: %q", got)
	}
}

func TestPlanExtractSkipsDuplicateStreamBasenames(t *testing.T) {
	srcA := t.TempDir()
	srcB := t.TempDir()
	destDir := t.TempDir()
	first := filepath.Join(srcA, "meadow.gz")
	second := filepath.Join(srcB, "meadow.gz")
	writeGzipFile(t, first, "first-harbor")
	writeGzipFile(t, second, "second-thicket")

	plan, skipped, err := PlanExtract([]string{first, second}, destDir, archive.Toolchain{Gzip: "/bin/gzip"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Path != first {
		t.Fatalf("items = %+v, want only first archive", plan.Items)
	}
	if !skipMentions(skipped, "collides") {
		t.Fatalf("skipped = %v, want colliding-basename reason", skipped)
	}
}

func TestExecuteExtractExistingStreamOutputPreserved(t *testing.T) {
	tc := requireGzipToolchain(t)
	srcDir := t.TempDir()
	destDir := t.TempDir()
	archivePath := filepath.Join(srcDir, "meadow.gz")
	writeGzipFile(t, archivePath, "incoming-thicket")
	existing := filepath.Join(destDir, "meadow")
	if err := os.WriteFile(existing, []byte("keep-harbor"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := ExtractPlan{
		Items:       []ExtractItem{{Path: archivePath, Format: archive.FormatGz}},
		Destination: destDir,
		Toolchain:   tc,
	}
	done, err := ExecuteExtract(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("expected error when stream destination already exists")
	}
	if done != 0 {
		t.Fatalf("done = %d, want 0", done)
	}
	got, readErr := os.ReadFile(existing)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "keep-harbor" {
		t.Fatalf("existing output replaced: %q", got)
	}
}

func TestExecuteExtractDuplicateStreamBasenames(t *testing.T) {
	tc := requireGzipToolchain(t)
	srcA := t.TempDir()
	srcB := t.TempDir()
	destDir := t.TempDir()
	first := filepath.Join(srcA, "meadow.gz")
	second := filepath.Join(srcB, "meadow.gz")
	writeGzipFile(t, first, "first-harbor")
	writeGzipFile(t, second, "second-thicket")

	plan := ExtractPlan{
		Items: []ExtractItem{
			{Path: first, Format: archive.FormatGz},
			{Path: second, Format: archive.FormatGz},
		},
		Destination: destDir,
		Toolchain:   tc,
	}
	done, err := ExecuteExtract(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("expected error for colliding stream basename")
	}
	if done != 1 {
		t.Fatalf("done = %d, want 1 (first archive only)", done)
	}
	got, readErr := os.ReadFile(filepath.Join(destDir, "meadow"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "first-harbor" {
		t.Fatalf("output = %q, want first-harbor (second must not replace)", got)
	}
}

func TestExecuteExtractStreamSkipAndCancel(t *testing.T) {
	tc := requireGzipToolchain(t)

	t.Run("skip", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()
		blocked := filepath.Join(srcDir, "meadow.gz")
		other := filepath.Join(srcDir, "tulip.gz")
		writeGzipFile(t, blocked, "incoming-thicket")
		writeGzipFile(t, other, "fresh-tulip")
		existing := filepath.Join(destDir, "meadow")
		if err := os.WriteFile(existing, []byte("keep-harbor"), 0o644); err != nil {
			t.Fatal(err)
		}

		plan := ExtractPlan{
			Items: []ExtractItem{
				{Path: blocked, Format: archive.FormatGz},
				{Path: other, Format: archive.FormatGz},
			},
			Destination: destDir,
			Toolchain:   tc,
			Conflict: func(src, dest string, facts FileConflictFacts) (bool, error) {
				_ = src
				_ = facts
				if dest == existing {
					return false, nil
				}
				return false, fmt.Errorf("unexpected conflict for %q", dest)
			},
		}
		done, err := ExecuteExtract(context.Background(), plan, nil)
		if err != nil {
			t.Fatalf("skip should not fail the job: %v", err)
		}
		if done != 1 {
			t.Fatalf("done = %d, want 1 (second archive)", done)
		}
		got, readErr := os.ReadFile(existing)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != "keep-harbor" {
			t.Fatalf("skipped dest replaced: %q", got)
		}
		otherOut, readErr := os.ReadFile(filepath.Join(destDir, "tulip"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(otherOut) != "fresh-tulip" {
			t.Fatalf("other archive output = %q, want fresh-tulip", otherOut)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		srcDir := t.TempDir()
		destDir := t.TempDir()
		blocked := filepath.Join(srcDir, "meadow.gz")
		other := filepath.Join(srcDir, "tulip.gz")
		writeGzipFile(t, blocked, "incoming-thicket")
		writeGzipFile(t, other, "fresh-tulip")
		existing := filepath.Join(destDir, "meadow")
		if err := os.WriteFile(existing, []byte("keep-harbor"), 0o644); err != nil {
			t.Fatal(err)
		}
		cancelErr := errors.New("canceled by user")

		plan := ExtractPlan{
			Items: []ExtractItem{
				{Path: blocked, Format: archive.FormatGz},
				{Path: other, Format: archive.FormatGz},
			},
			Destination: destDir,
			Toolchain:   tc,
			Conflict: func(src, dest string, facts FileConflictFacts) (bool, error) {
				_ = src
				_ = dest
				_ = facts
				return false, cancelErr
			},
		}
		done, err := ExecuteExtract(context.Background(), plan, nil)
		if !errors.Is(err, cancelErr) {
			t.Fatalf("err = %v, want canceled by user", err)
		}
		if done != 0 {
			t.Fatalf("done = %d, want 0", done)
		}
		got, readErr := os.ReadFile(existing)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != "keep-harbor" {
			t.Fatalf("canceled dest replaced: %q", got)
		}
		if _, statErr := os.Stat(filepath.Join(destDir, "tulip")); !os.IsNotExist(statErr) {
			t.Fatal("cancel must not extract remaining archives")
		}
	})
}

func TestExecuteExtractTarExistingMemberPreserved(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not in PATH")
	}
	srcDir := t.TempDir()
	destDir := t.TempDir()
	inner := filepath.Join(srcDir, "meadow.txt")
	if err := os.WriteFile(inner, []byte("incoming-thicket"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(srcDir, "harbor.tar.gz")
	cmd := exec.Command("tar", "-czf", archivePath, "-C", srcDir, "meadow.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destDir, "meadow.txt")
	if err := os.WriteFile(existing, []byte("keep-harbor"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := ExtractPlan{
		Items:       []ExtractItem{{Path: archivePath, Format: archive.FormatTarGz}},
		Destination: destDir,
		Toolchain:   archive.ProbeToolchain(),
	}
	_, _ = ExecuteExtract(context.Background(), plan, nil)
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep-harbor" {
		t.Fatalf("tar replaced existing member: %q", got)
	}
}

func writeGzipFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := gzip.NewWriter(f)
	if _, err := w.Write([]byte(content)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func requireGzipToolchain(t *testing.T) archive.Toolchain {
	t.Helper()
	tc := archive.ProbeToolchain()
	if tc.Gzip == "" {
		t.Skip("gzip not in PATH")
	}
	return tc
}

func skipMentions(skipped []string, needle string) bool {
	for _, s := range skipped {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
