package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// TestCopyDereferenceSymlinkToDir copies real content instead of a symlink when
// DereferenceSymlinks is set, and SummarizePlan's byte totals reflect the real content.
func TestCopyDereferenceSymlinkToDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	realDir := filepath.Join(srcDir, "thornwood")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("heron and larkspur")
	if err := os.WriteFile(filepath.Join(realDir, "heron.txt"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	linkPath := filepath.Join(srcDir, "link.dir")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	opts := PlanBuildOptions{DereferenceSymlinks: true}
	plan, err := BuildPlanCtx(context.Background(), MustPaths(linkPath), MustPath(dstDir), true, opts)
	if err != nil {
		t.Fatalf("BuildPlanCtx error = %v", err)
	}
	_, _, totalBytes := SummarizePlan(plan)
	if totalBytes != int64(len(content)) {
		t.Fatalf("totalBytes = %d, want %d", totalBytes, len(content))
	}

	execOpts := Options{CopyBufferKiB: 4, DereferenceSymlinks: true}
	done, doneBytes, err := ExecuteCopy(context.Background(), MustPaths(linkPath), MustPath(dstDir), execOpts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	if done < 1 {
		t.Fatalf("done files = %d, want at least 1", done)
	}
	if doneBytes != int64(len(content)) {
		t.Fatalf("doneBytes = %d, want %d", doneBytes, len(content))
	}

	dstLinkDir := filepath.Join(dstDir, "link.dir")
	info, err := os.Lstat(dstLinkDir)
	if err != nil {
		t.Fatalf("lstat destination %q: %v", dstLinkDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("destination %q should be a real directory, not a symlink", dstLinkDir)
	}
	gotContent, err := os.ReadFile(filepath.Join(dstLinkDir, "heron.txt"))
	if err != nil {
		t.Fatalf("read destination file: %v", err)
	}
	if string(gotContent) != string(content) {
		t.Fatalf("content = %q, want %q", gotContent, content)
	}
}

// TestCopyDereferenceSymlinkToFile copies the target file's real content and the destination
// carries no symlink mode bit.
func TestCopyDereferenceSymlinkToFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	content := []byte("mossy fern glade")
	target := filepath.Join(srcDir, "target.txt")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	linkPath := filepath.Join(srcDir, "link.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	execOpts := Options{CopyBufferKiB: 4, DereferenceSymlinks: true}
	done, doneBytes, err := ExecuteCopy(context.Background(), MustPaths(linkPath), MustPath(dstDir), execOpts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}
	if done != 1 {
		t.Fatalf("done files = %d, want 1", done)
	}
	if doneBytes != int64(len(content)) {
		t.Fatalf("doneBytes = %d, want %d", doneBytes, len(content))
	}

	dstLink := filepath.Join(dstDir, "link.txt")
	info, err := os.Lstat(dstLink)
	if err != nil {
		t.Fatalf("lstat destination: %v", err)
	}
	if localfs.IsSymlink(info) {
		t.Fatalf("destination %q should not be a symlink", dstLink)
	}
	gotContent, err := os.ReadFile(dstLink)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(gotContent) != string(content) {
		t.Fatalf("content = %q, want %q", gotContent, content)
	}
}

// TestCopyDereferenceSymlinkCycleFallsBackWithWarning: a symlink cycle under a dereferenced
// source completes the copy job without hanging or erroring, lands as a plain symlink at the
// destination, and records a warning.
func TestCopyDereferenceSymlinkCycleFallsBackWithWarning(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	dirA := filepath.Join(srcDir, "alderglen")
	dirB := filepath.Join(srcDir, "brackenmere")
	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	if err := os.Mkdir(dirB, 0o755); err != nil {
		t.Fatalf("mkdir b: %v", err)
	}
	if err := os.Symlink(dirB, filepath.Join(dirA, "loop")); err != nil {
		t.Fatalf("symlink a->b: %v", err)
	}
	if err := os.Symlink(dirA, filepath.Join(dirB, "loop")); err != nil {
		t.Fatalf("symlink b->a: %v", err)
	}

	var warnings []string
	opts := Options{CopyBufferKiB: 4, DereferenceSymlinks: true}
	planOpts := PlanBuildOptions{
		DereferenceSymlinks: true,
		OnWarning:           func(msg string) { warnings = append(warnings, msg) },
	}

	done := make(chan error, 1)
	go func() {
		plan, err := BuildPlanCtx(context.Background(), MustPaths(srcDir), MustPath(dstDir), true, planOpts)
		if err != nil {
			done <- err
			return
		}
		_, _, err = ExecuteCopyUsingPlan(context.Background(), plan, MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("copy error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("copy did not terminate (likely infinite symlink recursion)")
	}

	if len(warnings) == 0 {
		t.Fatal("expected at least one dereference-fallback warning")
	}
}

// TestCopyDereferenceDanglingSymlinkFallsBackWithWarning: a broken symlink under a
// dereferenced source completes the copy, falls back to a relinked entry, and records a warning.
func TestCopyDereferenceDanglingSymlinkFallsBackWithWarning(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	linkPath := filepath.Join(srcDir, "ghostlink")
	if err := os.Symlink(filepath.Join(srcDir, "nowhere"), linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var warnings []string
	opts := Options{CopyBufferKiB: 4, DereferenceSymlinks: true}
	planOpts := PlanBuildOptions{
		DereferenceSymlinks: true,
		OnWarning:           func(msg string) { warnings = append(warnings, msg) },
	}
	plan, err := BuildPlanCtx(context.Background(), MustPaths(srcDir), MustPath(dstDir), true, planOpts)
	if err != nil {
		t.Fatalf("BuildPlanCtx error = %v", err)
	}
	if _, _, err := ExecuteCopyUsingPlan(context.Background(), plan, MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteCopyUsingPlan error = %v", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	// srcDir is the top-level source (a directory), so its destination name is its own
	// basename under dstDir (batch-relative naming; see ops.TransferDestName).
	dstLink := filepath.Join(dstDir, filepath.Base(srcDir), "ghostlink")
	info, err := os.Lstat(dstLink)
	if err != nil {
		t.Fatalf("lstat destination: %v", err)
	}
	if !localfs.IsSymlink(info) {
		t.Fatal("dangling symlink should have been relinked, not skipped")
	}
}
