package ops

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// shortTempDir returns a temp dir with a short path: unix socket paths are limited to ~100 bytes.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pc")
	if err != nil {
		t.Skipf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestPlanWalkSkipsUnsupportedFileType: with OnItemError set a unix socket is reported and the
// other files are still planned; without it the walk aborts.
func TestPlanWalkSkipsUnsupportedFileType(t *testing.T) {
	srcDir := shortTempDir(t)
	dstDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "pebble.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	sockPath := filepath.Join(srcDir, "s")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	defer func() { _ = l.Close() }()

	if _, err := BuildPlanCtx(context.Background(), MustPaths(srcDir), MustPath(dstDir), true, PlanBuildOptions{}); err == nil {
		t.Fatal("walk without OnItemError succeeded, want unsupported file type error")
	}

	var failed []PlanItem
	plan, err := BuildPlanCtx(context.Background(), MustPaths(srcDir), MustPath(dstDir), true, PlanBuildOptions{
		OnItemError: func(item PlanItem, err error) { failed = append(failed, item) },
	})
	if err != nil {
		t.Fatalf("walk with OnItemError = %v", err)
	}
	if len(failed) != 1 || failed[0].Src.String() != sockPath {
		t.Fatalf("failed = %+v, want only the socket", failed)
	}
	var sawFile bool
	for _, it := range plan {
		if filepath.Base(it.Src.String()) == "pebble.txt" {
			sawFile = true
		}
		if it.Src.String() == sockPath {
			t.Fatal("socket was planned")
		}
	}
	if !sawFile {
		t.Fatalf("regular file missing from plan %+v", plan)
	}
}

// TestExecuteCopySkipsUnreadableSource: an unreadable source file is reported through
// OnItemError and the rest of the plan is still copied; without the callback the copy aborts.
func TestExecuteCopySkipsUnreadableSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 000 does not block root")
	}
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	locked := filepath.Join(srcDir, "locked.txt")
	for name, mode := range map[string]os.FileMode{"locked.txt": 0, "meadow.txt": 0o644} {
		p := filepath.Join(srcDir, name)
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := BuildPlanCtx(context.Background(), MustPaths(srcDir), MustPath(dstDir), true, PlanBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{CopyBufferKiB: 4}
	if _, _, err := ExecuteCopyUsingPlan(context.Background(), plan, MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil); err == nil || !localfs.IsSourceError(err) {
		t.Fatalf("copy without OnItemError = %v, want a source error", err)
	}

	var failed []PlanItem
	opts.OnItemError = func(item PlanItem, err error) { failed = append(failed, item) }
	if _, _, err := ExecuteCopyUsingPlan(context.Background(), plan, MustPaths(srcDir), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("copy with OnItemError = %v", err)
	}
	if len(failed) != 1 || failed[0].Src.String() != locked {
		t.Fatalf("failed = %+v, want only %s", failed, locked)
	}
	if _, err := os.Stat(filepath.Join(dstDir, filepath.Base(srcDir), "meadow.txt")); err != nil {
		if _, err2 := os.Stat(filepath.Join(dstDir, "meadow.txt")); err2 != nil {
			t.Fatalf("readable file not copied: %v / %v", err, err2)
		}
	}
}
