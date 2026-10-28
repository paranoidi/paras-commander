package ops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/fschange"
)

// recSink records reported batches, one line per change: "<kind> <path> [<- from] [bytes]",
// paths relative to root; a batch of several changes is joined with " & ".
type recSink struct {
	root string
	mu   sync.Mutex
	got  []string
}

func newRecSink(t *testing.T) (*recSink, context.Context, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &recSink{root: root}
	return s, fschange.WithSink(context.Background(), s), root
}

func (s *recSink) Apply(batch []fschange.Change) {
	rel := func(p string) string {
		r, err := filepath.Rel(s.root, p)
		if err != nil {
			return p
		}
		return r
	}
	parts := make([]string, 0, len(batch))
	for _, c := range batch {
		line := [...]string{"renamed", "removed", "created", "dirty"}[c.Kind] + " " + rel(c.Path)
		if c.IsDir {
			line += "/"
		}
		if c.From != "" {
			line += " <- " + rel(c.From)
		}
		if c.Bytes != 0 {
			line += fmt.Sprintf(" %d", c.Bytes)
		}
		parts = append(parts, line)
	}
	s.mu.Lock()
	s.got = append(s.got, strings.Join(parts, " & "))
	s.mu.Unlock()
}

func (s *recSink) want(t *testing.T, want ...string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	got := slices.Clone(s.got)
	slices.Sort(got)
	want = slices.Clone(want)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("reported\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestReportWithoutSinkIsSilent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "kettle.txt"), "abc")
	plan := RenamePlan{SourcePath: filepath.Join(dir, "kettle.txt"), NewPath: filepath.Join(dir, "teapot.txt")}
	if err := ExecuteRename(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
}

func TestReportRenameMkdirAndLinks(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "kettle.txt"), "abcd")
	if err := ExecuteRename(ctx, RenamePlan{SourcePath: filepath.Join(root, "kettle.txt"), NewPath: filepath.Join(root, "teapot.txt")}); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteMkdir(ctx, MkdirPlan{Path: filepath.Join(root, "lantern")}); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteSymlink(ctx, SymlinkPlan{Target: "teapot.txt", LinkPath: filepath.Join(root, "spout")}); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteHardlink(ctx, HardlinkPlan{Source: filepath.Join(root, "teapot.txt"), NewPath: filepath.Join(root, "lid.txt")}); err != nil {
		t.Fatal(err)
	}
	s.want(t,
		"renamed teapot.txt <- kettle.txt 4",
		"created lantern/",
		"created spout 10",
		"created lid.txt 4",
	)
}

func TestReportMassRenameIsOneNetBatch(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "amber"), "a")
	writeTestFile(t, filepath.Join(root, "basil"), "bb")
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(root, "amber"), OldBase: "amber", NewBase: "basil"},
		{SourcePath: filepath.Join(root, "basil"), OldBase: "basil", NewBase: "amber"},
	}
	if err := ExecuteMassRename(ctx, rows); err != nil {
		t.Fatal(err)
	}
	s.want(t, "renamed basil <- amber 1 & renamed amber <- basil 2")
}

func TestReportMoveRenameAndCrossDeviceFallback(t *testing.T) {
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "src/kettle.txt"), "abc")
	writeTestFile(t, filepath.Join(root, "far/orchard/fig.txt"), "figs")
	if err := os.MkdirAll(filepath.Join(root, "dst"), 0o755); err != nil {
		t.Fatal(err)
	}
	refuseRenameFrom(t, filepath.Join(root, "far"))
	srcs := MustPaths(filepath.Join(root, "src/kettle.txt"), filepath.Join(root, "far/orchard"))
	if _, _, err := ExecuteMove(ctx, srcs, MustPath(filepath.Join(root, "dst")), Options{CopyBufferKiB: 4, FlatDestNames: true}, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	s.want(t,
		"renamed dst/kettle.txt <- src/kettle.txt 3",
		"created dst/orchard/",
		"created dst/orchard/fig.txt 4",
		"removed far/orchard/fig.txt",
		"removed far/orchard/",
	)
}

func TestReportMergeMoveRemovesEmptiedSource(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "src/basket/fig.txt"), "f")
	writeTestFile(t, filepath.Join(root, "dst/basket/pear.txt"), "pp")
	if _, _, err := ExecuteMove(ctx, MustPaths(filepath.Join(root, "src/basket")), MustPath(filepath.Join(root, "dst")), Options{}, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	s.want(t,
		"renamed dst/basket/fig.txt <- src/basket/fig.txt 1",
		"removed src/basket/",
	)
}

func TestReportCopyCreatesAndOverwrites(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "src/basket/fig.txt"), "figs")
	writeTestFile(t, filepath.Join(root, "src/pear.txt"), "pear!")
	writeTestFile(t, filepath.Join(root, "dst/pear.txt"), "old")
	srcs := MustPaths(filepath.Join(root, "src/basket"), filepath.Join(root, "src/pear.txt"))
	if _, _, err := ExecuteCopy(ctx, srcs, MustPath(filepath.Join(root, "dst")), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, actionResolver(ActionOverwrite), nil); err != nil {
		t.Fatal(err)
	}
	s.want(t,
		"created dst/basket/",
		"created dst/basket/fig.txt 4",
		"created dst/pear.txt 5",
	)
}

func TestReportUndoOfCopyRemovesCreated(t *testing.T) {
	t.Parallel()
	_, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "src/basket/fig.txt"), "figs")
	if err := os.MkdirAll(filepath.Join(root, "dst"), 0o755); err != nil {
		t.Fatal(err)
	}
	var journal []UndoStep
	opts := Options{CopyBufferKiB: 4, OnJournal: func(st UndoStep) { journal = append(journal, st) }}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(filepath.Join(root, "src/basket")), MustPath(filepath.Join(root, "dst")), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	s := &recSink{root: root}
	if _, _, err := ExecuteUndo(fschange.WithSink(ctx, s), journal, Options{}, nil); err != nil {
		t.Fatal(err)
	}
	s.want(t, "removed dst/basket/fig.txt", "removed dst/basket/")
}

func TestReportDeleteAndRemoveEmpty(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "attic/trunk/quilt.txt"), "q")
	writeTestFile(t, filepath.Join(root, "attic/lamp.txt"), "l")
	if err := os.MkdirAll(filepath.Join(root, "attic/empty/hollow"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(root, "attic/trunk"), filepath.Join(root, "attic/lamp.txt")}
	if _, _, err := ExecuteDeletePaths(ctx, paths, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := RemoveEmptyDirsUnder(ctx, MustPaths(filepath.Join(root, "attic"))); err != nil {
		t.Fatal(err)
	}
	s.want(t,
		"removed attic/trunk/",
		"removed attic/lamp.txt",
		"removed attic/empty/hollow/",
		"removed attic/empty/",
		"removed attic/",
	)
}

func TestReportDeleteCanceledMidTreeIsDirty(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "attic/trunk/quilt.txt"), "q")
	ctx, cancel := context.WithCancel(ctx)
	_, _, err := ExecuteDeletePaths(ctx, []string{filepath.Join(root, "attic")}, nil, nil, func(int32) {
		cancel() // canceled once the source starts: the tree walk stops part way
	})
	if err == nil {
		t.Fatal("want cancel error")
	}
	s.want(t, "dirty attic/")
}

func TestReportMoveDeleteSourceRenamesNothing(t *testing.T) {
	t.Parallel()
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "src/harbor.txt"), "fresh")
	writeTestFile(t, filepath.Join(root, "dst/harbor.txt"), "stale")
	if _, _, err := ExecuteMove(ctx, MustPaths(filepath.Join(root, "src/harbor.txt")), MustPath(filepath.Join(root, "dst")), Options{}, ProgressEmitThrottle{}, nil, actionResolver(ActionDeleteSource), nil); err != nil {
		t.Fatal(err)
	}
	s.want(t, "removed src/harbor.txt")
}

func TestReportArchiveExtractMarksNewAndChangedTopLevelDirty(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not in PATH")
	}
	s, ctx, root := newRecSink(t)
	writeTestFile(t, filepath.Join(root, "pack/basket/fig.txt"), "fig")
	writeTestFile(t, filepath.Join(root, "pack/kettle.txt"), "k")
	archivePath := filepath.Join(root, "pack.tar")
	if out, err := exec.Command("tar", "-cf", archivePath, "-C", filepath.Join(root, "pack"), "basket", "kettle.txt").CombinedOutput(); err != nil {
		t.Fatalf("tar: %v %s", err, out)
	}
	dest := filepath.Join(root, "dest")
	writeTestFile(t, filepath.Join(dest, "untouched.txt"), "u")
	// basket exists already: tar merges into it and its mtime moves.
	if err := os.MkdirAll(filepath.Join(dest, "basket"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dest, "basket"), old, old); err != nil {
		t.Fatal(err)
	}
	plan, _, err := PlanExtract([]string{archivePath}, dest, archive.ProbeToolchain())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteExtract(ctx, plan, nil); err != nil {
		t.Fatal(err)
	}
	s.want(t, "dirty dest/basket/", "dirty dest/kettle.txt")
}
