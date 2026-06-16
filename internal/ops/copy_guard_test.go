package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestDestinationUnderSource(t *testing.T) {
	t.Parallel()
	root := MustPath("/tmp/example")
	child := MustPath("/tmp/example/build/out")
	if !DestinationUnderSource(root, child) {
		t.Fatal("expected child under root")
	}
	if DestinationUnderSource(root, root) {
		t.Fatal("same path should not count as under")
	}
	if DestinationUnderSource(child, root) {
		t.Fatal("ancestor should not be under descendant")
	}
}

func TestBuildPlanRejectsCopyIntoSubdirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "proj")
	sub := filepath.Join(src, "build")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := BuildPlan(MustPaths(src), MustPath(sub), true)
	if err == nil {
		t.Fatal("BuildPlan() error = nil, want error for copy into subdirectory of itself")
	}
	if !strings.Contains(err.Error(), "subdirectory of itself") {
		t.Fatalf("error = %v, want subdirectory guard message", err)
	}
}

func TestCopyPreservesDirectoryPermissionsAndTimestamps(t *testing.T) {
	t.Parallel()
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	nested := filepath.Join(srcRoot, "tree")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "leaf.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mod := time.Date(2019, 1, 2, 3, 4, 5, 0, time.UTC)
	atime := time.Date(2018, 6, 7, 8, 9, 0, 0, time.UTC)
	if err := os.Chtimes(nested, atime, mod); err != nil {
		t.Fatal(err)
	}

	opts := Options{PreservePermissions: true, PreserveTimestamps: true, CopyBufferKiB: 4}
	_, _, err := ExecuteCopy(context.Background(), MustPaths(nested), MustPath(dstRoot), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}

	destNested := filepath.Join(dstRoot, "tree")
	info, err := os.Stat(destNested)
	if err != nil {
		t.Fatalf("stat dest dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dest dir mode = %o, want 0700", info.Mode().Perm())
	}
	gotAtime, gotMtime := localfs.FileTimes(info)
	if !gotMtime.Equal(mod) {
		t.Fatalf("dest mtime = %v, want %v", gotMtime, mod)
	}
	if !gotAtime.Equal(atime) {
		t.Fatalf("dest atime = %v, want %v", gotAtime, atime)
	}
}

func TestCopyPreservesFileAccessTime(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "timed.txt")
	if err := os.WriteFile(srcFile, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	mod := time.Date(2021, 5, 10, 12, 0, 0, 0, time.UTC)
	atime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(srcFile, atime, mod); err != nil {
		t.Fatal(err)
	}

	opts := Options{PreserveTimestamps: true, CopyBufferKiB: 4}
	_, _, err := ExecuteCopy(context.Background(), MustPaths(srcFile), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteCopy error = %v", err)
	}

	dstFile := filepath.Join(dstDir, "timed.txt")
	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	gotAtime, gotMtime := localfs.FileTimes(info)
	if !gotMtime.Equal(mod) {
		t.Fatalf("dest mtime = %v, want %v", gotMtime, mod)
	}
	if !gotAtime.Equal(atime) {
		t.Fatalf("dest atime = %v, want %v", gotAtime, atime)
	}
}
