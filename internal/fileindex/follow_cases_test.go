package fileindex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func mustRenameFile(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

// An unindexed file moved over an indexed one replaced its content, so the record's size, hash and
// tags no longer describe what is there.
func TestApplyRenamedUnindexedOverRecordedTarget(t *testing.T) {
	x, root := scanned(t)
	n := changeCounter(x)
	target := filepath.Join(root, "harbor.txt")
	if _, err := x.EditTags([]string{target}, []string{"amber"}, true, false); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "willow.txt")
	writeFile(t, root, "willow.txt", "something else entirely")
	mustRenameFile(t, source, target)
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: source, Path: target}})
	if _, ok := recorded(t, x, target); ok {
		t.Fatal("the target keeps the record of the content it lost")
	}
	if n.Load() == 0 {
		t.Fatal("no OnChange for the dropped record")
	}
}

// A directory renamed onto one with stale records: rename(2) proves the target was empty on disk,
// so its records are gone and the source's take their place.
func TestApplyRenamedDirOntoStaleClashingRecords(t *testing.T) {
	x, root := scanned(t)
	source, target := filepath.Join(root, "lantern"), filepath.Join(root, "beacon")
	writeFile(t, root, "beacon/meadow.txt", "stale, same name as the source's")
	writeFile(t, root, "beacon/only-here.txt", "stale")
	overlayFixture(t, x, root, "beacon/meadow.txt", "beacon/only-here.txt")
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	mustRenameFile(t, source, target)
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: source, Path: target, IsDir: true}})

	if got, ok := recorded(t, x, filepath.Join(target, "meadow.txt")); !ok || got != int64(len("clover")) {
		t.Errorf("moved record = %d, %v; want the source's size %d", got, ok, len("clover"))
	}
	if _, ok := recorded(t, x, filepath.Join(target, "only-here.txt")); ok {
		t.Error("a stale record of the target survived")
	}
	if _, ok := recorded(t, x, filepath.Join(source, "meadow.txt")); ok {
		t.Error("the source keeps a ghost record")
	}
	if got := ghosts(t, x, root); len(got) != 0 {
		t.Errorf("ghosts = %v", got)
	}
}

// A dir move that still fails must not leave the source as a ghost. Records below it are refused a
// move by a trigger; dropping them works.
func TestApplyRenamedDirThatCannotMoveDropsSource(t *testing.T) {
	x, root := scanned(t)
	source, target := filepath.Join(root, "lantern"), filepath.Join(root, "beacon")
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("CREATE TRIGGER refuse BEFORE UPDATE OF parent_id ON dir BEGIN SELECT RAISE(ABORT, 'refused'); END"); err != nil {
		t.Fatal(err)
	}
	mustRenameFile(t, source, target)
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: source, Path: target, IsDir: true}})
	if _, ok := recorded(t, x, filepath.Join(source, "meadow.txt")); ok {
		t.Error("the source keeps a ghost record")
	}
}

// Changes apply in the order they arrived.
func TestApplyBatchKeepsOrder(t *testing.T) {
	t.Run("renamed then source name removed", func(t *testing.T) {
		x, root := scanned(t)
		a, b := filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt")
		mustRenameFile(t, a, b)
		applyAndWait(x, []fschange.Change{
			{Kind: fschange.Renamed, From: a, Path: b},
			{Kind: fschange.Removed, Path: a},
		})
		if _, ok := recorded(t, x, b); !ok {
			t.Error("the moved file lost its record")
		}
	})
	t.Run("renamed then target removed", func(t *testing.T) {
		x, root := scanned(t)
		a, b := filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt")
		if err := os.Remove(a); err != nil {
			t.Fatal(err)
		}
		applyAndWait(x, []fschange.Change{
			{Kind: fschange.Renamed, From: a, Path: b},
			{Kind: fschange.Removed, Path: b},
		})
		if _, ok := recorded(t, x, b); ok {
			t.Error("the removed file keeps a record")
		}
	})
	t.Run("a swap beside a removal still chains", func(t *testing.T) {
		x, root := scanned(t)
		a, b := filepath.Join(root, "lantern/meadow.txt"), filepath.Join(root, "lantern/thistle.md")
		gone := filepath.Join(root, "harbor.txt")
		applyAndWait(x, []fschange.Change{
			{Kind: fschange.Removed, Path: gone},
			{Kind: fschange.Renamed, From: a, Path: b},
			{Kind: fschange.Renamed, From: b, Path: a},
		})
		if got, _ := recorded(t, x, b); got != int64(len("clover")) {
			t.Errorf("record at %s has size %d", b, got)
		}
		if _, ok := recorded(t, x, gone); ok {
			t.Error("the removed file keeps a record")
		}
	})
}

// A move across file systems is a copy and a delete: the record goes, and the tag returns when a
// scan hashes the destination.
func TestCrossDeviceMoveGetsTagBackFromScan(t *testing.T) {
	x, root := scanned(t)
	source := filepath.Join(root, "harbor.txt")
	if _, err := x.EditTags([]string{source}, []string{"amber"}, true, false); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "lantern", "harbor-copy.txt")
	mustRenameFile(t, source, other)
	applyAndWait(x, []fschange.Change{
		{Kind: fschange.Created, Path: other, Bytes: 15},
		{Kind: fschange.Removed, Path: source},
	})
	if _, ok := recorded(t, x, source); ok {
		t.Fatal("the source keeps its record")
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	state, err := x.TagState([]string{other})
	if err != nil {
		t.Fatal(err)
	}
	if state.Counts["amber"] != 1 {
		t.Fatalf("tag counts at the destination = %v", state.Counts)
	}
}

// Ordinary copies report nothing to follow: they must not even queue.
func TestApplyQueuesNothingForCreatedAndDirty(t *testing.T) {
	x, root := scanned(t)
	x.writeMu.Lock()
	defer x.writeMu.Unlock()
	x.Apply([]fschange.Change{{Kind: fschange.Created, Path: filepath.Join(root, "new.txt")}, {Kind: fschange.Dirty, Path: root, IsDir: true}})
	x.applyMu.Lock()
	defer x.applyMu.Unlock()
	if len(x.applyPending) != 0 || x.applyDone != nil {
		t.Fatalf("queued %d batches, drain running %v", len(x.applyPending), x.applyDone != nil)
	}
}

func TestApplyQueuesNothingWithoutDatabase(t *testing.T) {
	x := testIndex(t, filepath.Join(t.TempDir(), "none.sqlite"))
	x.Apply([]fschange.Change{{Kind: fschange.Removed, Path: "/orchard/harbor.txt"}})
	x.applyMu.Lock()
	defer x.applyMu.Unlock()
	if len(x.applyPending) != 0 || x.applyDone != nil || len(x.gone) != 0 {
		t.Fatalf("queued %d batches, drain running %v, gone %v", len(x.applyPending), x.applyDone != nil, x.gone)
	}
}

// Tagging reports the empty files it skipped and the copies it also reached.
func TestEditTagsReportsSkippedAndOtherCopies(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	writeFile(t, root, "empty", "")
	writeFile(t, root, "copy.txt", "anchor and rope")
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	res, err := x.EditTags([]string{filepath.Join(root, "empty")}, []string{"amber"}, true, false)
	if err != nil || res.Changed != 0 || res.SkippedEmpty != 1 {
		t.Fatalf("empty file: %+v, %v", res, err)
	}
	res, err = x.EditTags([]string{filepath.Join(root, "harbor.txt")}, []string{"amber"}, true, false)
	if err != nil || res.Changed != 1 || res.OtherCopies != 1 {
		t.Fatalf("file with a copy: %+v, %v", res, err)
	}
	res, err = x.EditTags([]string{filepath.Join(root, "copy.txt")}, []string{"amber"}, false, false)
	if err != nil || res.Changed != 1 || res.OtherCopies != 1 {
		t.Fatalf("removal: %+v, %v", res, err)
	}
}

// A pass names the directories whose listings it changed.
func TestApplyScopesOnChange(t *testing.T) {
	x, root := scanned(t)
	var got [][]string
	x.OnChange = func(dirs []string) { got = append(got, dirs) }
	lantern := filepath.Join(root, "lantern")
	meadow, moved := filepath.Join(lantern, "meadow.txt"), filepath.Join(root, "meadow.txt")
	mustRenameFile(t, meadow, moved)
	applyAndWait(x, []fschange.Change{{Kind: fschange.Renamed, From: meadow, Path: moved}})
	if len(got) != 1 || !slices.Equal(slices.Sorted(slices.Values(got[0])), []string{root, lantern}) {
		t.Fatalf("OnChange scopes = %v, want one naming %s and %s", got, root, lantern)
	}
}

func TestScanReportsCommittedDirs(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	var mu sync.Mutex
	seen := map[string]bool{}
	x.OnScanCommit = func(dirs []string) {
		mu.Lock()
		defer mu.Unlock()
		for _, d := range dirs {
			seen[d] = true
		}
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	if !seen[root] || !seen[filepath.Join(root, "lantern")] {
		t.Fatalf("reported %v", seen)
	}
}

// A panel can show a directory through a symlink, while scopes name it as the index records it.
func TestShownDirFollowsSymlinkAfterOverlay(t *testing.T) {
	x, root := scanned(t)
	link := filepath.Join(t.TempDir(), "shortcut")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	loc, err := pathloc.File(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x.Overlay(context.Background(), loc, listLocal(t, link)); err != nil {
		t.Fatal(err)
	}
	if got := x.ShownDir(loc); got != root {
		t.Fatalf("ShownDir = %q, want %q", got, root)
	}
	plain, _ := pathloc.File(root)
	if got := x.ShownDir(plain); got != root {
		t.Fatalf("ShownDir = %q, want %q", got, root)
	}
	indexed, err := pathloc.FileIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := x.ShownDir(indexed); got != root {
		t.Fatalf("ShownDir of a fileindex:// path = %q, want %q", got, root)
	}
}

// A tag scan records and tags an unknown tree without fully hashing it, even with hashing on: it
// only samples the files, so their pending tags survive a move before a full hash.
func TestScanTagTaskSamplesInsteadOfHashing(t *testing.T) {
	x := testIndex(t, initIndex(t)) // Hash: true
	root := t.TempDir()
	reel := filepath.Join(root, "cinema", "harbor-lights.mkv")
	if err := os.MkdirAll(filepath.Dir(reel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reel, make([]byte, 100_000), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(reel)
	if err := x.ScanTagTask([]string{dir}, []string{"watched"}, true).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	var full, sampled bool
	if err := d.QueryRow("SELECT file_id IS NOT NULL, sha256_sample IS NOT NULL FROM instance WHERE filename = ?", filepath.Base(reel)).Scan(&full, &sampled); err != nil {
		t.Fatal(err)
	}
	if full || !sampled {
		t.Fatalf("after a tag scan: full hash %v, sample %v", full, sampled)
	}
	tags, err := d.EntryTags(dir, []string{filepath.Base(reel)})
	if err != nil || !slices.Equal(tags[filepath.Base(reel)], []string{"watched"}) {
		t.Fatalf("tags = %v, %v", tags, err)
	}
}

// Tagging a known tree reports it when some of its files have neither a full hash nor a sample.
func TestEditTagsReportsUnsampled(t *testing.T) {
	x := testIndex(t, initIndex(t))
	x.cfg.Hash = false
	root := tree(t)
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	res, err := x.EditTags([]string{root}, []string{"amber"}, true, true)
	if err != nil || !slices.Equal(res.Unsampled, []string{root}) || len(res.Unknown) != 0 {
		t.Fatalf("walked only: %+v, %v", res, err)
	}
	if err := x.ScanTagTask(res.Unsampled, []string{"amber"}, true).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	if res, err = x.EditTags([]string{root}, []string{"amber"}, true, true); err != nil || len(res.Unsampled) != 0 {
		t.Fatalf("after the tag scan: %+v, %v", res, err)
	}
	if res, err = x.EditTags([]string{root}, []string{"amber"}, false, true); err != nil || len(res.Unsampled) != 0 {
		t.Fatalf("a removal: %+v, %v", res, err)
	}
}
