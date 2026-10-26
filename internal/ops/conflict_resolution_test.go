package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func actionResolver(a ConflictAction) ConflictResolver {
	return func(context.Context, string, string, FileConflictFacts) (ConflictResolution, error) {
		return ConflictResolution{Action: a}, nil
	}
}

func writeConflictPair(t *testing.T, name, srcData, dstData string) (src, dstDir string) {
	t.Helper()
	src = filepath.Join(t.TempDir(), name)
	dstDir = t.TempDir()
	if err := os.WriteFile(src, []byte(srcData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, name), []byte(dstData), 0o644); err != nil {
		t.Fatal(err)
	}
	return src, dstDir
}

func TestCopyIdenticalLeavesBothFiles(t *testing.T) {
	src, dstDir := writeConflictPair(t, "lantern.txt", "same", "same")
	done, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, actionResolver(ActionIdentical), nil)
	if err != nil {
		t.Fatal(err)
	}
	if done != 0 {
		t.Fatalf("done = %d, want 0 (nothing copied)", done)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must remain after copy: %v", err)
	}
}

func TestMoveIdenticalKeepsSource(t *testing.T) {
	src, dstDir := writeConflictPair(t, "orchard.txt", "same", "same")
	var reason string
	opts := Options{CopyBufferKiB: 4, OnItemDone: func(_ PlanItem, _ ItemResult, r string) { reason = r }}
	if _, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, actionResolver(ActionIdentical), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must remain: %v", err)
	}
	if reason != ReasonIdentical {
		t.Fatalf("reason = %q, want %q", reason, ReasonIdentical)
	}
}

func TestMoveDeleteSourceKeepsDestination(t *testing.T) {
	src, dstDir := writeConflictPair(t, "harbor.txt", "fresh", "stale")
	var reason string
	var renamed int
	opts := Options{
		CopyBufferKiB: 4,
		OnItemDone:    func(_ PlanItem, _ ItemResult, r string) { reason = r },
		OnRenamed:     func(_, _ pathloc.Path) { renamed++ },
	}
	if _, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, actionResolver(ActionDeleteSource), nil); err != nil {
		t.Fatal(err)
	}
	if renamed != 0 {
		t.Fatalf("OnRenamed called %d times; a deleted source renames nothing", renamed)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source must be removed, stat err = %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dstDir, "harbor.txt")); string(b) != "stale" {
		t.Fatalf("dest = %q, want the kept destination", b)
	}
	if reason != ReasonSourceDeleted {
		t.Fatalf("reason = %q, want %q", reason, ReasonSourceDeleted)
	}
}

func TestDeleteSourceNeverCrossesDirectoryBoundary(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "willow")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dstDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dstDir, "willow"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, actionResolver(ActionDeleteSource), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source directory must remain: %v", err)
	}
}

func TestRenameUsesFirstFreeSiblingName(t *testing.T) {
	for _, run := range []struct {
		name string
		exec func(src, dstDir string) error
	}{
		{"copy", func(src, dstDir string) error {
			_, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, actionResolver(ActionRename), nil)
			return err
		}},
		{"move", func(src, dstDir string) error {
			_, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, actionResolver(ActionRename), nil)
			return err
		}},
	} {
		t.Run(run.name, func(t *testing.T) {
			src, dstDir := writeConflictPair(t, "meadow.txt", "fresh", "stale")
			for i, want := range []string{"meadow (1).txt", "meadow (2).txt"} {
				if i == 1 {
					// Second round: "(1)" now exists, the next free name is "(2)".
					if err := os.WriteFile(src, []byte("fresh2"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				if err := run.exec(src, dstDir); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(dstDir, want)); err != nil {
					t.Fatalf("round %d: %s missing: %v", i, want, err)
				}
			}
			if b, _ := os.ReadFile(filepath.Join(dstDir, "meadow.txt")); string(b) != "stale" {
				t.Fatalf("existing = %q, want untouched", b)
			}
		})
	}
}
