package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// sourceOf is "base=source" per reported item, sorted, so a test states which source every item
// was attributed to.
func sourceOf(items []PlanItem) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%s=%d", it.Src.Base(), it.Source))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func collectItems(out *[]PlanItem) ItemFunc {
	return func(item PlanItem, _ ItemResult, _ string) { *out = append(*out, item) }
}

func stepSources(steps []UndoStep) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, fmt.Sprintf("%s=%d", s.Path.Base(), s.Source))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func TestCopyAttributesItemsAndJournalToSources(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	harbor, meadow := filepath.Join(base, "harbor"), filepath.Join(base, "meadow")
	writeTree(t, harbor, map[string]string{"lantern.txt": "l", "deep/acorn.bin": "a"})
	writeTree(t, meadow, map[string]string{"willow.txt": "w"})
	var items []PlanItem
	var journal []UndoStep
	opts := Options{CopyBufferKiB: 4, OnItemDone: collectItems(&items), OnJournal: func(s UndoStep) { journal = append(journal, s) }}
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(harbor, meadow), MustPath(dst), opts, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(items), "acorn.bin=0 lantern.txt=0 willow.txt=1"; got != want {
		t.Fatalf("items = %s, want %s", got, want)
	}
	if got, want := stepSources(journal), "acorn.bin=0 deep=0 harbor=0 lantern.txt=0 meadow=1 willow.txt=1"; got != want {
		t.Fatalf("journal = %s, want %s", got, want)
	}
}

// remoteTreeBackend serves a fixed remote tree: dirs maps a directory path to its child names,
// any other listed path is a file.
type remoteTreeBackend struct {
	remoteDeleteBackend
	dirs map[string][]string
}

func (b *remoteTreeBackend) entry(loc pathloc.Path) fsbackend.Entry {
	typ := fsbackend.EntryFile
	if _, ok := b.dirs[loc.String()]; ok {
		typ = fsbackend.EntryDirectory
	}
	return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: typ}
}

func (b *remoteTreeBackend) Stat(_ context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	if _, ok := b.dirs[loc.String()]; ok {
		return b.entry(loc), nil
	}
	if names, ok := b.dirs[loc.Parent().String()]; ok {
		for _, n := range names {
			if n == loc.Base() {
				return b.entry(loc), nil
			}
		}
	}
	return fsbackend.Entry{}, os.ErrNotExist
}

func (b *remoteTreeBackend) List(_ context.Context, loc pathloc.Path) ([]fsbackend.Entry, error) {
	var out []fsbackend.Entry
	for _, n := range b.dirs[loc.String()] {
		child, err := loc.Join(n)
		if err != nil {
			return nil, err
		}
		out = append(out, b.entry(child))
	}
	return out, nil
}

func TestRemotePlanAttributesItemsToSources(t *testing.T) {
	const thicket, quarry = remoteDeleteRoot, "sftp://alice@example.com/quarry"
	installOpsBackend(t, &remoteTreeBackend{dirs: map[string][]string{
		thicket:                    {"bramble.txt", "copper"},
		thicket + "/copper":        {"thistle.log"},
		quarry:                     {"pebble.txt"},
		"sftp://alice@example.com": {"thicket", "quarry"},
	}})
	plan, err := BuildPlanCtx(context.Background(), MustPaths(thicket, quarry), MustPath(t.TempDir()), true, PlanBuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(plan), "bramble.txt=0 copper=0 pebble.txt=1 quarry=1 thicket=0 thistle.log=0"; got != want {
		t.Fatalf("plan = %s, want %s", got, want)
	}
}

func TestPlanItemErrorsCarrySource(t *testing.T) {
	var failed []PlanItem
	opts := PlanBuildOptions{OnItemError: func(it PlanItem, _ error) { failed = append(failed, it) }}
	sink := func(PlanItem) error { return nil }
	stamped, stampedOpts := sourceStamped(2, sink, opts)
	_ = stamped(PlanItem{})
	if err := skipOrFail(stampedOpts, PlanItem{Src: MustPath("/orchard/socket")}, os.ErrInvalid); err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 || failed[0].Source != 2 {
		t.Fatalf("failed = %+v, want source 2", failed)
	}
}

func TestMoveAttributesRenamesAndCopyFallbackToSources(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	lantern, meadow, ember := filepath.Join(base, "lantern"), filepath.Join(base, "meadow"), filepath.Join(base, "ember.txt")
	writeTree(t, lantern, map[string]string{"quill.txt": "q"})
	writeTree(t, meadow, map[string]string{"tulip.txt": "t", "moss/fern.txt": "f"})
	writeTestFile(t, ember, "e")
	refuseRenameFrom(t, meadow) // source 1 goes through the copy fallback, planned on its own

	var items []PlanItem
	var journal []UndoStep
	opts := Options{CopyBufferKiB: 4, FlatDestNames: true, OnItemDone: collectItems(&items), OnJournal: func(s UndoStep) { journal = append(journal, s) }}
	if _, _, err := ExecuteMove(context.Background(), MustPaths(lantern, meadow, ember), MustPath(dst), opts, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(items), "ember.txt=2 fern.txt=1 lantern=0 tulip.txt=1"; got != want {
		t.Fatalf("items = %s, want %s", got, want)
	}
	if got, want := stepSources(journal), "ember.txt=2 lantern=0 meadow=1"; got != want {
		t.Fatalf("journal = %s, want %s", got, want)
	}
}

func TestFlattenAttributesItemsToRoots(t *testing.T) {
	base := t.TempDir()
	forest, grove, dest := filepath.Join(base, "forest"), filepath.Join(base, "grove"), filepath.Join(base, "clearing")
	writeTree(t, forest, map[string]string{"birch.txt": "b", "moss/fern.txt": "f"})
	writeTree(t, grove, map[string]string{"cedar.txt": "c"})
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	var items []PlanItem
	opts := Options{CopyBufferKiB: 4, OnItemDone: collectItems(&items)}
	if _, _, err := ExecuteFlatten(context.Background(), MustPaths(forest, grove), MustPath(dest), true, false, opts, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(items), "birch.txt=0 cedar.txt=1 fern.txt=0"; got != want {
		t.Fatalf("items = %s, want %s", got, want)
	}
}

func TestDeleteAttributesPathsToSources(t *testing.T) {
	base := t.TempDir()
	quarry, pebble := filepath.Join(base, "quarry"), filepath.Join(base, "pebble.txt")
	writeTree(t, quarry, map[string]string{"slate.txt": "s"})
	writeTestFile(t, pebble, "p")
	var items []PlanItem
	if _, _, err := ExecuteDeletePaths(context.Background(), []string{MustPath(quarry).String(), MustPath(pebble).String()}, nil, collectItems(&items)); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(items), "pebble.txt=1 quarry=0"; got != want {
		t.Fatalf("items = %s, want %s", got, want)
	}
}

func TestExtractAttributesArchivesToSourcePositions(t *testing.T) {
	tc := requireGzipToolchain(t)
	srcDir, destDir := t.TempDir(), t.TempDir()
	notes, acorn, heron := filepath.Join(srcDir, "notes.txt"), filepath.Join(srcDir, "acorn.gz"), filepath.Join(srcDir, "heron.gz")
	writeTestFile(t, notes, "not an archive")
	writeGzipFile(t, acorn, "a")
	writeGzipFile(t, heron, "h")
	plan, _, err := PlanExtract([]string{acorn, notes, heron}, destDir, tc)
	if err != nil {
		t.Fatal(err)
	}
	var items []PlanItem
	plan.OnItem = collectItems(&items)
	if _, err := ExecuteExtract(context.Background(), plan, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(items), "acorn.gz=0 heron.gz=2"; got != want { // the skipped notes.txt keeps its index
		t.Fatalf("items = %s, want %s", got, want)
	}
}

func TestUndoAttributesStepsToTheirSources(t *testing.T) {
	base, dst := t.TempDir(), t.TempDir()
	harbor, meadow := filepath.Join(base, "harbor"), filepath.Join(base, "meadow")
	writeTree(t, harbor, map[string]string{"lantern.txt": "l"})
	writeTree(t, meadow, map[string]string{"willow.txt": "w"})
	var journal []UndoStep
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(harbor, meadow), MustPath(dst), journalOpts(&journal), ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	// One created file changed since the copy: its failure is attributed too.
	writeTestFile(t, filepath.Join(dst, "meadow", "willow.txt"), "grown")
	var done, failed []PlanItem
	opts := Options{CopyBufferKiB: 4, OnItemDone: collectItems(&done), OnItemError: func(it PlanItem, _ error) { failed = append(failed, it) }}
	if _, _, err := ExecuteUndo(context.Background(), journal, opts, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(done), "harbor=0 lantern.txt=0"; got != want {
		t.Fatalf("done = %s, want %s", got, want)
	}
	if got, want := sourceOf(failed), "meadow=1 willow.txt=1"; got != want {
		t.Fatalf("failed = %s, want %s", got, want)
	}
}

func TestUndoMassRenameAttributesRowsToSteps(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"amber.txt", "basil.txt"} {
		writeTestFile(t, filepath.Join(dir, n), n)
	}
	steps := []UndoStep{
		{Kind: UndoMoveBack, Path: MustPath(filepath.Join(dir, "amber.txt")), From: MustPath(filepath.Join(dir, "cobalt.txt")), Source: 0},
		{Kind: UndoMoveBack, Path: MustPath(filepath.Join(dir, "basil.txt")), From: MustPath(filepath.Join(dir, "dahlia.txt")), Source: 1},
	}
	var done []PlanItem
	if _, _, err := ExecuteUndo(context.Background(), steps, Options{OnItemDone: collectItems(&done)}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := sourceOf(done), "amber.txt=0 basil.txt=1"; got != want {
		t.Fatalf("done = %s, want %s", got, want)
	}
}
