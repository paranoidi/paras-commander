package fileindex

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fschange"
)

// recorded reports whether the index holds a file instance at path and its recorded size.
func recorded(t *testing.T, x *Index, path string) (int64, bool) {
	t.Helper()
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	rec, ok, err := d.FileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Size, ok
}

// applyAndWait queues batches and returns once they are applied.
func applyAndWait(x *Index, batches ...[]fschange.Change) {
	for _, b := range batches {
		x.Apply(b)
	}
	x.WaitApplied()
}

func changeCounter(x *Index) *atomic.Int32 {
	var n atomic.Int32
	x.OnChange = func() { n.Add(1) }
	return &n
}

func TestApplyRemoved(t *testing.T) {
	x, root := scanned(t)
	n := changeCounter(x)
	file, dir := filepath.Join(root, "harbor.txt"), filepath.Join(root, "lantern")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Removed, Path: file}})
	if _, ok := recorded(t, x, file); ok {
		t.Fatal("a removed file keeps its record")
	}
	if n.Load() != 1 {
		t.Fatalf("OnChange calls = %d, want 1", n.Load())
	}
	// IsDir is not trusted: the index decides whether it is a file or a directory.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Removed, Path: dir, IsDir: false}})
	if _, ok := recorded(t, x, filepath.Join(dir, "meadow.txt")); ok {
		t.Fatal("a removed directory keeps the records below it")
	}
}

func TestApplyRenamed(t *testing.T) {
	x, root := scanned(t)
	n := changeCounter(x)
	oldFile, newFile := filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt")
	size := int64(len("anchor and rope"))
	if err := os.Rename(oldFile, newFile); err != nil {
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: oldFile, Path: newFile}})
	if got, ok := recorded(t, x, newFile); !ok || got != size {
		t.Fatalf("renamed file record = %d, %v; want %d, true", got, ok, size)
	}
	if _, ok := recorded(t, x, oldFile); ok {
		t.Fatal("the old name keeps a record")
	}
	if n.Load() != 1 {
		t.Fatalf("OnChange calls = %d, want 1", n.Load())
	}

	// A directory moves with everything under it.
	oldDir, newDir := filepath.Join(root, "lantern"), filepath.Join(root, "beacon")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: oldDir, Path: newDir, IsDir: true}})
	if _, ok := recorded(t, x, filepath.Join(newDir, "meadow.txt")); !ok {
		t.Fatal("a renamed directory's records did not follow")
	}
}

func TestApplyRenamedOverRecordedTarget(t *testing.T) {
	x, root := scanned(t)
	meadow, thistle := filepath.Join(root, "lantern/meadow.txt"), filepath.Join(root, "lantern/thistle.md")
	if err := os.Rename(meadow, thistle); err != nil { // replaces the recorded thistle.md
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: meadow, Path: thistle}})
	if got, ok := recorded(t, x, thistle); !ok || got != int64(len("clover")) {
		t.Fatalf("target record = %d, %v; want the moved file's size %d", got, ok, len("clover"))
	}
	if _, ok := recorded(t, x, meadow); ok {
		t.Fatal("the source name keeps a record")
	}
}

func TestApplyRenamedSwap(t *testing.T) {
	x, root := scanned(t)
	a, b := filepath.Join(root, "lantern/meadow.txt"), filepath.Join(root, "lantern/thistle.md")
	sizeA, sizeB := int64(len("clover")), int64(len("bramble patch"))
	tmp := filepath.Join(root, "lantern/swap.tmp")
	for _, mv := range [][2]string{{a, tmp}, {b, a}, {tmp, b}} {
		if err := os.Rename(mv[0], mv[1]); err != nil {
			t.Fatal(err)
		}
	}
	// A mass rename reports its net renames as one batch.
	applyAndWait(x, []fschange.Change{
		{Kind: fschange.Renamed, From: a, Path: b},
		{Kind: fschange.Renamed, From: b, Path: a},
	})
	if got, _ := recorded(t, x, b); got != sizeA {
		t.Errorf("record at %s has size %d, want %d (the file that moved there)", b, got, sizeA)
	}
	if got, _ := recorded(t, x, a); got != sizeB {
		t.Errorf("record at %s has size %d, want %d (the file that moved there)", a, got, sizeB)
	}
}

func TestApplyThroughSymlinkedDirectory(t *testing.T) {
	x, root := scanned(t)
	link := filepath.Join(t.TempDir(), "shortcut")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	// The change carries the path the user acted on; the index records the resolved one.
	if err := os.Remove(filepath.Join(link, "harbor.txt")); err != nil {
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Removed, Path: filepath.Join(link, "harbor.txt")}})
	if _, ok := recorded(t, x, filepath.Join(root, "harbor.txt")); ok {
		t.Fatal("a removal reported through a symlinked directory missed its record")
	}
}

func TestApplyIgnoresWhatTheIndexDoesNotHold(t *testing.T) {
	x, root := scanned(t)
	n := changeCounter(x)
	other := filepath.Join(root, "willow.txt")
	applyAndWait(x,
		[]fschange.Change{{Kind: fschange.Removed, Path: other}},
		[]fschange.Change{{Kind: fschange.Renamed, From: other, Path: filepath.Join(root, "reed.txt")}},
		[]fschange.Change{{Kind: fschange.Created, Path: other, Bytes: 4}},
		[]fschange.Change{{Kind: fschange.Dirty, Path: root, IsDir: true}},
	)
	if n.Load() != 0 {
		t.Fatalf("OnChange calls = %d, want none when no record changed", n.Load())
	}
	if _, ok := recorded(t, x, filepath.Join(root, "harbor.txt")); !ok {
		t.Fatal("an unrelated change dropped a record")
	}
}

func TestApplyWithoutDatabaseCreatesNothing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "none.sqlite")
	x := testIndex(t, dbPath)
	n := changeCounter(x)
	var asked atomic.Int32
	x.OnOpenError = func(error) { asked.Add(1) }
	applyAndWait(x, []fschange.Change{{Kind: fschange.Removed, Path: "/orchard/harbor.txt"}})
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatal("Apply created a database")
	}
	if n.Load() != 0 || asked.Load() != 0 {
		t.Fatalf("OnChange %d, OnOpenError %d; want neither", n.Load(), asked.Load())
	}
}

func TestApplyNilIndexAndCloseInFlight(t *testing.T) {
	var none *Index
	none.Apply([]fschange.Change{{Kind: fschange.Removed, Path: "/orchard"}}) // must not panic

	x, root := scanned(t)
	for range 50 {
		x.Apply([]fschange.Change{{Kind: fschange.Removed, Path: filepath.Join(root, "harbor.txt")}})
	}
	if err := x.Close(); err != nil { // safe with a drain in flight
		t.Fatal(err)
	}
	x.Apply([]fschange.Change{{Kind: fschange.Removed, Path: filepath.Join(root, "harbor.txt")}}) // closed: ignored
	x.WaitApplied()
}
