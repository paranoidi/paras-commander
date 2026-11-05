package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSoftStopFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stopAfterFirstFile returns options plus a soft-stop context that requests the stop once one
// file has been reported done.
func stopAfterFirstFile(opts Options) (context.Context, Options) {
	done := 0
	opts.OnItemDone = func(_ PlanItem, res ItemResult, _ string) {
		if res == ItemDone {
			done++
		}
	}
	return WithSoftStop(context.Background(), func() bool { return done >= 1 }), opts
}

func TestSoftStopCopyFinishesFileAndAppliesDirMetadata(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "meadow")
	dst := filepath.Join(root, "orchard")
	writeSoftStopFile(t, filepath.Join(src, "amber.txt"), "amber")
	writeSoftStopFile(t, filepath.Join(src, "cobalt.txt"), "cobalt")
	writeSoftStopFile(t, filepath.Join(src, "dune.txt"), "dune")
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(src, old, old); err != nil {
		t.Fatal(err)
	}

	ctx, opts := stopAfterFirstFile(Options{CopyBufferKiB: 4, PreserveTimestamps: true})
	_, _, err := ExecuteCopy(ctx, MustPaths(src), MustPath(dst), opts, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, errSoftStopped) {
		t.Fatalf("err = %v, want soft stop wrapping context.Canceled", err)
	}
	if got := readFileContent(t, filepath.Join(dst, "amber.txt")); got != "amber" {
		t.Fatalf("amber.txt = %q", got)
	}
	if exists(filepath.Join(dst, "cobalt.txt")) || exists(filepath.Join(dst, "dune.txt")) {
		t.Fatal("files after the stop were copied")
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(old) {
		t.Fatalf("directory mtime = %v, want deferred metadata applied (%v)", fi.ModTime(), old)
	}
}

func TestSoftStopMoveCopyPhaseKeepsUncopiedSources(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "meadow")
	dst := filepath.Join(root, "orchard")
	writeSoftStopFile(t, filepath.Join(src, "dune", "pebble.txt"), "pebble")
	writeSoftStopFile(t, filepath.Join(src, "tulip.txt"), "tulip")
	if err := os.MkdirAll(filepath.Join(src, "zephyr"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, opts := stopAfterFirstFile(Options{CopyBufferKiB: 4})
	var steps []UndoStep
	opts.OnJournal = func(s UndoStep) { steps = append(steps, s) }
	r := moveRun{opts: opts, resolver: overwriteAllResolver()}
	_, _, err := r.copyTask(ctx, moveTask{src: MustPath(src), dst: MustPath(dst), destination: MustPath(dst)}, 0, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := readFileContent(t, filepath.Join(dst, "dune", "pebble.txt")); got != "pebble" {
		t.Fatalf("destination pebble.txt = %q", got)
	}
	if exists(filepath.Join(src, "dune", "pebble.txt")) || exists(filepath.Join(src, "dune")) {
		t.Fatal("moved file's source (and its now-empty directory) should be gone")
	}
	for _, keep := range []string{"tulip.txt", "zephyr"} {
		if !exists(filepath.Join(src, keep)) {
			t.Fatalf("uncopied %s must stay in the source", keep)
		}
	}
	if exists(filepath.Join(dst, "tulip.txt")) {
		t.Fatal("tulip.txt was copied after the stop")
	}
	if len(steps) != 1 || steps[0].Kind != UndoMoveBack ||
		steps[0].From.String() != filepath.Join(src, "dune", "pebble.txt") ||
		steps[0].Path.String() != filepath.Join(dst, "dune", "pebble.txt") {
		t.Fatalf("journal = %+v, want one move-back step for pebble.txt", steps)
	}
}

func TestSoftStopDeleteStopsBetweenChildren(t *testing.T) {
	root := t.TempDir()
	tree := filepath.Join(root, "meadow")
	for _, name := range []string{"amber.txt", "cobalt.txt", "dune.txt"} {
		writeSoftStopFile(t, filepath.Join(tree, name), name)
	}
	removed := 0
	ctx := WithSoftStop(context.Background(), func() bool {
		entries, _ := os.ReadDir(tree)
		removed = 3 - len(entries)
		return removed >= 1
	})
	_, _, err := ExecuteDeletePaths(ctx, []string{tree}, nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	entries, _ := os.ReadDir(tree)
	if len(entries) != 2 || removed != 1 {
		t.Fatalf("%d entries left (removed %d), want the delete to stop after one file", len(entries), removed)
	}
}
