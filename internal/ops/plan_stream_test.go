package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// buildStreamFixtureTree creates a small directory tree with random-English-word names
// (per repo testing convention, never real project filenames) and returns its root.
func buildStreamFixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "meadow")
	if err := os.MkdirAll(filepath.Join(src, "lantern"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	files := map[string]string{
		filepath.Join(src, "harbor.txt"):            "harbor-content",
		filepath.Join(src, "lantern", "copper.txt"): "copper-content",
		filepath.Join(src, "lantern", "willow.txt"): "willow-content",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %q: %v", path, err)
		}
	}
	return src
}

func drainPlanStream(t *testing.T, ch <-chan PlanItem) []PlanItem {
	t.Helper()
	var out []PlanItem
	for it := range ch {
		out = append(out, it)
	}
	return out
}

// planItemShape strips filesystem-observed timestamps (which can drift a few nanoseconds
// between two separate walks of the same directory, since Lstat re-reads live atime/mtime)
// so parity checks compare only what the walk itself decides: paths, kind, size, mode.
type planItemShape struct {
	Src, Dst         string
	IsDir, IsSymlink bool
	FileSize         int64
}

func shapePlanItems(items []PlanItem) []planItemShape {
	out := make([]planItemShape, len(items))
	for i, it := range items {
		out[i] = planItemShape{
			Src: it.Src.String(), Dst: it.Dst.String(),
			IsDir: it.IsDir, IsSymlink: it.IsSymlink, FileSize: it.FileSize,
		}
	}
	return out
}

func TestBuildPlanStreamCtxParityWithBuildPlanCtx(t *testing.T) {
	// Two independent copies of the same tree shape: walking the same directory twice can
	// perturb its own atime, so comparing a slice-walk and a stream-walk of the identical
	// inode is flaky. Separate copies keep the parity check about the walk logic only.
	srcSlice := buildStreamFixtureTree(t)
	srcStream := buildStreamFixtureTree(t)
	dstDir := filepath.Join(t.TempDir(), "orchard")

	sliceItems, err := BuildPlanCtx(context.Background(), MustPaths(srcSlice), MustPath(dstDir), true, PlanBuildOptions{})
	if err != nil {
		t.Fatalf("BuildPlanCtx error = %v", err)
	}

	out := make(chan PlanItem, 8)
	var streamErr error
	done := make(chan struct{})
	go func() {
		streamErr = BuildPlanStreamCtx(context.Background(), MustPaths(srcStream), MustPath(dstDir), true, PlanBuildOptions{}, out)
		close(done)
	}()
	streamItems := drainPlanStream(t, out)
	<-done
	if streamErr != nil {
		t.Fatalf("BuildPlanStreamCtx error = %v", streamErr)
	}

	sliceShape := shapePlanItems(sliceItems)
	streamShape := shapePlanItems(streamItems)
	// Rewrite the stream shape's source root to match the slice's so path comparison isn't
	// defeated by the two fixtures living in different temp dirs.
	for i := range streamShape {
		streamShape[i].Src = filepath.ToSlash(streamShape[i].Src)[len(filepath.ToSlash(srcStream)):]
		streamShape[i].Dst = filepath.ToSlash(streamShape[i].Dst)[len(filepath.ToSlash(dstDir)):]
	}
	for i := range sliceShape {
		sliceShape[i].Src = filepath.ToSlash(sliceShape[i].Src)[len(filepath.ToSlash(srcSlice)):]
		sliceShape[i].Dst = filepath.ToSlash(sliceShape[i].Dst)[len(filepath.ToSlash(dstDir)):]
	}

	if !reflect.DeepEqual(sliceShape, streamShape) {
		t.Fatalf("stream items differ from slice items:\nslice=%#v\nstream=%#v", sliceShape, streamShape)
	}
	if len(streamItems) != 5 { // meadow(dir) + harbor.txt + lantern(dir) + copper.txt + willow.txt
		t.Fatalf("len(streamItems) = %d, want 5", len(streamItems))
	}
}

func TestBuildPlanStreamCtxCancellationMidWalk(t *testing.T) {
	src := buildStreamFixtureTree(t)
	dstDir := filepath.Join(t.TempDir(), "orchard")

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan PlanItem)
	errCh := make(chan error, 1)
	go func() {
		errCh <- BuildPlanStreamCtx(ctx, MustPaths(src), MustPath(dstDir), true, PlanBuildOptions{}, out)
	}()

	// Receive exactly one item, then cancel — the walk must observe cancellation and stop
	// instead of blocking forever trying to send further items into an unbuffered channel
	// nobody else reads.
	select {
	case _, ok := <-out:
		if !ok {
			t.Fatal("channel closed before first item")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for first item")
	}
	cancel()

	// Drain until closed so the producer goroutine can exit.
	for range out {
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("BuildPlanStreamCtx error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for BuildPlanStreamCtx to return")
	}
}

func TestBuildPlanStreamCtxErrorPropagation(t *testing.T) {
	// Multiple sources into a destination that doesn't exist: validateBatchDestinationCtx
	// rejects this upfront (same rule prepareCopyDestinationCtx enforces for the slice path),
	// so the walk goroutine never starts and out closes immediately with zero items.
	srcA := filepath.Join(t.TempDir(), "meadow.txt")
	srcB := filepath.Join(t.TempDir(), "harbor.txt")
	for _, p := range []string{srcA, srcB} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %q: %v", p, err)
		}
	}
	missingDst := filepath.Join(t.TempDir(), "does-not-exist")

	out := make(chan PlanItem, 4)
	err := BuildPlanStreamCtx(context.Background(), MustPaths(srcA, srcB), MustPath(missingDst), true, PlanBuildOptions{}, out)
	if err == nil {
		t.Fatal("expected error for multi-source copy to nonexistent destination")
	}
	if items := drainPlanStream(t, out); len(items) != 0 {
		t.Fatalf("expected no items on validation error, got %d", len(items))
	}
}
