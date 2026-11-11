package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exactRunner runs one single-source transfer to an exact destination.
type exactRunner struct {
	name string
	run  func(src, dst string, r ConflictResolver) error
}

func exactRunners(extra func(*Options)) []exactRunner {
	opts := Options{CopyBufferKiB: 4, ExactDest: true}
	if extra != nil {
		extra(&opts)
	}
	return []exactRunner{
		{"copy", func(src, dst string, r ConflictResolver) error {
			_, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dst), opts, ProgressEmitThrottle{}, nil, r, nil)
			return err
		}},
		{"move", func(src, dst string, r ConflictResolver) error {
			_, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dst), opts, ProgressEmitThrottle{}, nil, r, nil)
			return err
		}},
	}
}

func TestExactDestWritesMissingTargetAndParents(t *testing.T) {
	for _, rn := range exactRunners(nil) {
		t.Run(rn.name, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "willow.txt")
			dst := filepath.Join(root, "out", "marsh", "reed.txt") // name differs from the source's
			writeMismatchTree(t, map[string]string{src: "payload"})
			if err := rn.run(src, dst, nil); err != nil {
				t.Fatal(err)
			}
			if got := readFileContent(t, dst); got != "payload" {
				t.Fatalf("target = %q, want payload", got)
			}
		})
	}
}

func TestExactDestMoveCopyFallbackWritesMissingTargetAndParents(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in", "willow.txt")
	dst := filepath.Join(root, "out", "marsh", "reed.txt")
	writeMismatchTree(t, map[string]string{src: "payload"})
	refuseRenameFrom(t, src)
	if err := exactRunners(nil)[1].run(src, dst, nil); err != nil {
		t.Fatal(err)
	}
	if got := readFileContent(t, dst); got != "payload" {
		t.Fatalf("target = %q, want payload", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source must be gone after the copy fallback: %v", err)
	}
}

func TestExactDestDirectoryAtTargetIsMismatchConflict(t *testing.T) {
	for _, rn := range exactRunners(nil) {
		t.Run(rn.name, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "orchard.txt")
			dst := filepath.Join(root, "out", "orchard.txt") // a directory, same name as the source
			writeMismatchTree(t, map[string]string{src: "new", filepath.Join(dst, "pear.txt"): "keep"})
			var seen []FileConflictFacts
			resolver := func(_ context.Context, _, d string, f FileConflictFacts) (ConflictResolution, error) {
				if d != dst {
					t.Errorf("conflict at %q, want the exact target %q", d, dst)
				}
				seen = append(seen, f)
				return ConflictResolution{Action: ActionSkip}, nil
			}
			if err := rn.run(src, dst, resolver); err != nil {
				t.Fatal(err)
			}
			if len(seen) != 1 || !seen[0].OverwriteUnsafe() || !seen[0].DestIsDir() {
				t.Fatalf("resolver facts = %+v, want one directory mismatch", seen)
			}
			if _, err := os.Stat(filepath.Join(dst, "orchard.txt")); !os.IsNotExist(err) {
				t.Fatalf("source landed inside the directory: %v", err)
			}
			if got := readFileContent(t, filepath.Join(dst, "pear.txt")); got != "keep" {
				t.Fatalf("directory contents changed: %q", got)
			}
			if got := readFileContent(t, src); got != "new" {
				t.Fatalf("skipped source changed: %q", got)
			}
		})
	}
}

func TestExactDestMoveCopyFallbackDirectoryAtTargetKeepsBothAsSibling(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in", "orchard.txt")
	dst := filepath.Join(root, "out", "orchard.txt")
	writeMismatchTree(t, map[string]string{src: "new", filepath.Join(dst, "pear.txt"): "keep"})
	refuseRenameFrom(t, src)
	rename := func(context.Context, string, string, FileConflictFacts) (ConflictResolution, error) {
		return ConflictResolution{Action: ActionRename}, nil
	}
	if err := exactRunners(nil)[1].run(src, dst, rename); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "orchard.txt")); !os.IsNotExist(err) {
		t.Fatalf("source landed inside the directory: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("out has %d entries, want the directory plus the renamed sibling", len(entries))
	}
}

func TestExactDestRejectsSeveralSources(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "in", "heron.txt"), filepath.Join(root, "in", "egret.txt")
	writeMismatchTree(t, map[string]string{a: "a", b: "b"})
	dst := MustPath(filepath.Join(root, "out", "stork.txt"))
	opts := Options{CopyBufferKiB: 4, ExactDest: true}
	srcs := MustPaths(a, b)
	if _, _, err := ExecuteCopy(context.Background(), srcs, dst, opts, ProgressEmitThrottle{}, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "exactly one source") {
		t.Fatalf("copy err = %v", err)
	}
	if _, _, err := ExecuteMove(context.Background(), srcs, dst, opts, ProgressEmitThrottle{}, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "exactly one source") {
		t.Fatalf("move err = %v", err)
	}
	out := make(chan PlanItem, 4)
	if err := BuildPlanStreamCtx(context.Background(), srcs, dst, true, PlanBuildOptions{ExactDest: true}, out); err == nil {
		t.Fatal("streamed plan accepted several sources")
	}
	if _, err := os.Stat(filepath.Join(root, "out")); !os.IsNotExist(err) {
		t.Fatalf("rejected transfer created the destination: %v", err)
	}
}

func TestExactDestPlanTargetsDestinationItself(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in", "willow.txt")
	dst := filepath.Join(root, "out", "reed.txt")
	writeMismatchTree(t, map[string]string{src: "x", filepath.Join(dst, "pear.txt"): "y"}) // dst is a directory
	plan, err := BuildPlanCtx(context.Background(), MustPaths(src), MustPath(dst), true, PlanBuildOptions{ExactDest: true})
	if err != nil || len(plan) != 1 || plan[0].Dst.String() != dst {
		t.Fatalf("plan = %+v, err = %v; want one item at %q", plan, err, dst)
	}
}

func TestExactDestStreamedPlanTargetsDestinationItself(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in", "willow.txt")
	dst := filepath.Join(root, "out", "reed.txt")
	writeMismatchTree(t, map[string]string{src: "x", filepath.Join(dst, "pear.txt"): "y"})
	out := make(chan PlanItem, 4)
	if err := BuildPlanStreamCtx(context.Background(), MustPaths(src), MustPath(dst), true, PlanBuildOptions{ExactDest: true}, out); err != nil {
		t.Fatal(err)
	}
	var got []PlanItem
	for it := range out {
		got = append(got, it)
	}
	if len(got) != 1 || got[0].Dst.String() != dst {
		t.Fatalf("streamed plan = %+v, want one item at %q", got, dst)
	}
}
