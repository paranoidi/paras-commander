package ops

import (
	"context"
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

func TestPlanCopyHereSiblingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "project")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := localfs.Entry{Name: "project", Path: src, Type: localfs.EntryDirectory}
	plan, err := PlanCopyHere(entry, "project-copy", dir)
	if err != nil {
		t.Fatalf("PlanCopyHere() error = %v", err)
	}
	wantDest := filepath.Join(dir, "project-copy")
	if plan.DestPath != wantDest {
		t.Fatalf("DestPath = %q, want %q", plan.DestPath, wantDest)
	}
	if plan.SourcePath != src {
		t.Fatalf("SourcePath = %q, want %q", plan.SourcePath, src)
	}
}

func TestBuildCopyPlanCopyHereNoExtraNesting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "project-copy")
	plan, _, _, err := BuildCopyPlanWithTotals(MustPaths(src), MustPath(dest))
	if err != nil {
		t.Fatalf("BuildCopyPlanWithTotals() error = %v", err)
	}
	if len(plan) == 0 {
		t.Fatal("plan is empty")
	}
	nestedExtra := filepath.Join(dest, "project", "nested", "child.txt")
	nestedWant := filepath.Join(dest, "nested", "child.txt")
	for _, item := range plan {
		dst, err := item.Dst.FilePath()
		if err != nil {
			t.Fatalf("item.Dst.FilePath() error = %v", err)
		}
		if dst == nestedExtra {
			t.Fatalf("plan nests source basename under destination: %q", dst)
		}
	}
	var sawNestedChild bool
	for _, item := range plan {
		dst, err := item.Dst.FilePath()
		if err != nil {
			t.Fatal(err)
		}
		if dst == nestedWant {
			sawNestedChild = true
		}
	}
	if !sawNestedChild {
		t.Fatalf("plan missing %q; got destinations from first items", nestedWant)
	}
}

func TestExecuteCopyCopyHereSiblingSemantics(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "child.txt"), []byte("child"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "project-copy")
	opts := Options{CopyBufferKiB: 4}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dest), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteCopy() error = %v", err)
	}

	nestedExtra := filepath.Join(dest, "project", "nested", "child.txt")
	nestedWant := filepath.Join(dest, "nested", "child.txt")
	if _, err := os.Stat(nestedExtra); err == nil {
		t.Fatalf("unexpected nested copy at %q", nestedExtra)
	}
	if _, err := os.Stat(nestedWant); err != nil {
		t.Fatalf("missing copied child at %q: %v", nestedWant, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "root.txt")); err != nil {
		t.Fatalf("missing copied root file: %v", err)
	}
}

func TestExecuteCopyExistingDestinationDirStillNestsBasename(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "project")
	dstParent := filepath.Join(dir, "out")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dstParent, 0o755); err != nil {
		t.Fatal(err)
	}

	opts := Options{CopyBufferKiB: 4}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dstParent), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteCopy() error = %v", err)
	}
	want := filepath.Join(dstParent, "project", "nested")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("missing nested copy under destination dir at %q: %v", want, err)
	}
}
