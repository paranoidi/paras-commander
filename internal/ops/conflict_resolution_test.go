package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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

func TestMoveIdenticalRemovesSource(t *testing.T) {
	src, dstDir := writeConflictPair(t, "orchard.txt", "same", "same")
	if _, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, actionResolver(ActionIdentical), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source must be removed, stat err = %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dstDir, "orchard.txt")); string(b) != "same" {
		t.Fatalf("dest = %q", b)
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
