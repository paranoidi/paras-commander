package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func journalOpts(journal *[]UndoStep) Options {
	return Options{CopyBufferKiB: 4, OnJournal: func(s UndoStep) { *journal = append(*journal, s) }}
}

func readTestFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// undoAll runs ExecuteUndo and returns the per-step failures it reported.
func undoAll(t *testing.T, steps []UndoStep) (done int, failed []string) {
	t.Helper()
	opts := Options{CopyBufferKiB: 4, OnItemError: func(item PlanItem, err error) { failed = append(failed, item.Src.String()+": "+err.Error()) }}
	done, _, err := ExecuteUndo(context.Background(), steps, opts, nil)
	if err != nil {
		t.Fatalf("ExecuteUndo: %v", err)
	}
	return done, failed
}

func copyTree(t *testing.T, src, dstParent string) []UndoStep {
	t.Helper()
	var journal []UndoStep
	if _, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dstParent), journalOpts(&journal), ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteCopy: %v", err)
	}
	return journal
}

func TestUndoCopyRestoresTree(t *testing.T) {
	base, dstParent := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "meadow")
	writeTree(t, src, map[string]string{"orchard/apple": "a", "river/stone": "s"})
	steps := copyTree(t, src, dstParent)
	if len(steps) != 5 { // meadow, orchard, apple, river, stone
		t.Fatalf("journal has %d steps: %+v", len(steps), steps)
	}
	done, failed := undoAll(t, steps)
	if done != 5 || len(failed) != 0 {
		t.Fatalf("done=%d failed=%v", done, failed)
	}
	if exists(filepath.Join(dstParent, "meadow")) {
		t.Fatal("copied tree should be gone")
	}
	if !exists(filepath.Join(src, "orchard", "apple")) {
		t.Fatal("source must be untouched")
	}
}

func TestUndoCopySkipsModifiedFileAndKeepsGrownDir(t *testing.T) {
	base, dstParent := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "meadow")
	writeTree(t, src, map[string]string{"orchard/apple": "a", "river/stone": "s"})
	steps := copyTree(t, src, dstParent)

	apple := filepath.Join(dstParent, "meadow", "orchard", "apple")
	if err := os.WriteFile(apple, []byte("edited later"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(apple, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	writeTree(t, filepath.Join(dstParent, "meadow", "river"), map[string]string{"willow": "w"})

	_, failed := undoAll(t, steps)
	// apple (modified) is kept, so orchard and meadow stay non-empty; river keeps willow; stone goes.
	if len(failed) != 4 {
		t.Fatalf("want 4 reported steps (apple, orchard, river, meadow), got %v", failed)
	}
	if readTestFile(t, apple) != "edited later" {
		t.Fatal("modified file must be kept")
	}
	if exists(filepath.Join(dstParent, "meadow", "river", "stone")) {
		t.Fatal("untouched copy should be removed")
	}
	if !exists(filepath.Join(dstParent, "meadow", "river", "willow")) {
		t.Fatal("user file lost")
	}
}

func TestUndoMoveRestoresSourceAndMergeKeepsExisting(t *testing.T) {
	base, dstParent := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "meadow")
	writeTree(t, src, map[string]string{"orchard/apple": "a", "orchard/pear": "p", "river/stone": "s"})
	writeTree(t, dstParent, map[string]string{"meadow/orchard/plum": "z"})
	var journal []UndoStep
	if _, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstParent), journalOpts(&journal), ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if exists(src) {
		t.Fatal("source should have moved")
	}
	done, failed := undoAll(t, journal)
	if len(failed) != 0 || done != len(journal) {
		t.Fatalf("done=%d failed=%v journal=%+v", done, failed, journal)
	}
	for _, rel := range []string{"orchard/apple", "orchard/pear", "river/stone"} {
		if !exists(filepath.Join(src, rel)) {
			t.Fatalf("%s not restored", rel)
		}
	}
	if !exists(filepath.Join(dstParent, "meadow", "orchard", "plum")) {
		t.Fatal("pre-existing destination file must stay")
	}
	if exists(filepath.Join(dstParent, "meadow", "river")) {
		t.Fatal("moved dir should have left the destination")
	}
}

func TestUndoMassRenameSwap(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"apple": "was-apple", "pear": "was-pear"})
	apple, pear := filepath.Join(dir, "apple"), filepath.Join(dir, "pear")
	rows := []MassRenameRow{
		{SourcePath: apple, OldBase: "apple", NewBase: "pear"},
		{SourcePath: pear, OldBase: "pear", NewBase: "apple"},
	}
	if err := ExecuteMassRename(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	steps := []UndoStep{
		{Kind: UndoMoveBack, Path: MustPath(pear), From: MustPath(apple)},
		{Kind: UndoMoveBack, Path: MustPath(apple), From: MustPath(pear)},
	}
	done, failed := undoAll(t, steps)
	if done != 2 || len(failed) != 0 {
		t.Fatalf("done=%d failed=%v", done, failed)
	}
	if readTestFile(t, apple) != "was-apple" || readTestFile(t, pear) != "was-pear" {
		t.Fatal("swap not reversed")
	}
}

func TestUndoMoveBackSkipsOccupiedOrigin(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"lantern": "moved", "harbor": "squatter"})
	steps := []UndoStep{{Kind: UndoMoveBack, Path: MustPath(filepath.Join(dir, "lantern")), From: MustPath(filepath.Join(dir, "harbor"))}}
	done, failed := undoAll(t, steps)
	if done != 0 || len(failed) != 1 {
		t.Fatalf("done=%d failed=%v", done, failed)
	}
	if readTestFile(t, filepath.Join(dir, "harbor")) != "squatter" || !exists(filepath.Join(dir, "lantern")) {
		t.Fatal("nothing may be overwritten")
	}
}

func TestUndoMassRenameSkipsOccupiedOrigin(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"lantern": "1", "willow": "2", "harbor": "squatter"})
	steps := []UndoStep{
		{Kind: UndoMoveBack, Path: MustPath(filepath.Join(dir, "lantern")), From: MustPath(filepath.Join(dir, "harbor"))},
		{Kind: UndoMoveBack, Path: MustPath(filepath.Join(dir, "willow")), From: MustPath(filepath.Join(dir, "river"))},
	}
	done, failed := undoAll(t, steps)
	if done != 1 || len(failed) != 1 {
		t.Fatalf("done=%d failed=%v", done, failed)
	}
	if readTestFile(t, filepath.Join(dir, "harbor")) != "squatter" || !exists(filepath.Join(dir, "river")) {
		t.Fatal("occupied origin kept, free origin restored")
	}
}
