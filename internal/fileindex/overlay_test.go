package fileindex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// overlayFixture indexes files of root (by relative path) with their current size and mtime.
func overlayFixture(t *testing.T, x *Index, root string, rels ...string) {
	t.Helper()
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	var batch []db.ScannedFile
	for i, rel := range rels {
		fi, err := os.Stat(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		sum := strings.Repeat(string(rune('a'+i)), 32)
		batch = append(batch, db.ScannedFile{Dir: filepath.Dir(filepath.Join(root, rel)), Name: filepath.Base(rel),
			Size: fi.Size(), Mtime: fi.ModTime(), MD5: sum, SHA256: sum + sum})
	}
	if _, err := d.CommitScanBatch(batch); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// listLocal lists dir like the file backend does, unfiltered.
func listLocal(t *testing.T, dir string) []fsbackend.Entry {
	t.Helper()
	loc, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fsbackend.Default().Backend(loc)
	if err != nil {
		t.Fatal(err)
	}
	list, err := entries.List(context.Background(), loc)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestOverlayStatesAndGhosts(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "thistle.txt", "short")
	writeFile(t, root, "clover.txt", "field")
	writeFile(t, root, "bramble.txt", "gone soon")
	overlayFixture(t, x, root, "harbor.txt", "thistle.txt", "clover.txt", "bramble.txt", "lantern/meadow.txt")
	writeFile(t, root, "thistle.txt", "now a much longer body") // changed
	writeFile(t, root, "willow.txt", "fresh")                   // new
	writeFile(t, root, "Thumbs.db", "junk")                     // excluded by the default rules
	writeFile(t, root, "node_modules/pebble.txt", "x")          // excluded dir
	if err := os.Remove(filepath.Join(root, "bramble.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := x.EditTags([]string{filepath.Join(root, "clover.txt")}, []string{"field"}, true, false); err != nil {
		t.Fatal(err)
	}

	dir, _ := pathloc.File(root)
	out, err := x.Overlay(context.Background(), dir, listLocal(t, root))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range out {
		got[e.Name] = e.IndexStatus
	}
	want := map[string]string{
		"harbor.txt":   StatusIndexed,
		"thistle.txt":  StatusChanged,
		"clover.txt":   StatusTagged,
		"willow.txt":   StatusNew,
		"Thumbs.db":    StatusExcluded,
		"node_modules": StatusExcluded,
		"lantern":      StatusIndexed,
		"bramble.txt":  StatusMissing,
	}
	for name, st := range want {
		if got[name] != st {
			t.Errorf("%s status = %q, want %q", name, got[name], st)
		}
	}
	for _, e := range out {
		if e.Name == "bramble.txt" {
			if e.Loc.Scheme() != pathloc.SchemeFileIndex || e.Size != int64(len("gone soon")) {
				t.Errorf("ghost = %+v, want a fileindex:// row with the indexed size", e)
			}
		}
	}
}

func TestOverlayGhostDirAndIndexUnavailable(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	overlayFixture(t, x, root, "harbor.txt", "lantern/meadow.txt")
	if err := os.RemoveAll(filepath.Join(root, "lantern")); err != nil {
		t.Fatal(err)
	}
	dir, _ := pathloc.File(root)
	out, err := x.Overlay(context.Background(), dir, listLocal(t, root))
	if err != nil {
		t.Fatal(err)
	}
	var ghost *fsbackend.Entry
	for i, e := range out {
		if e.Name == "lantern" {
			ghost = &out[i]
		}
	}
	if ghost == nil || ghost.Type != fsbackend.EntryDirectory || ghost.IndexStatus != StatusMissing {
		t.Fatalf("lantern = %+v, want a missing ghost directory", ghost)
	}

	absent := testIndex(t, filepath.Join(t.TempDir(), "none.sqlite"))
	plain := listLocal(t, root)
	same, err := absent.Overlay(context.Background(), dir, plain)
	if err != nil || len(same) != len(plain) || same[0].IndexStatus != "" {
		t.Fatalf("without an index entries must come back unchanged: %v %+v", err, same)
	}
}

func TestStatusColumnSharesExcluded(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "Thumbs.db", "junk")
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	vals, err := x.statusValues(context.Background(), d, root, []localfs.Entry{{Name: "Thumbs.db", Type: localfs.EntryFile}}, true)
	if err != nil || vals["Thumbs.db"] != StatusExcluded {
		t.Fatalf("status column = %v, %v", vals, err)
	}
}

func TestDirDrift(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "orchard/apple/pear.txt", "pear")
	writeFile(t, root, "pasture/clover/hay.txt", "hay")
	writeFile(t, root, "marsh/reed/heron.txt", "heron")
	writeFile(t, root, "quarry/stone/slate.txt", "slate")
	writeFile(t, root, "meadow/brook/trout.txt", "trout")
	overlayFixture(t, x, root, "orchard/apple/pear.txt", "pasture/clover/hay.txt", "marsh/reed/heron.txt", "quarry/stone/slate.txt", "meadow/brook/trout.txt")

	writeFile(t, root, "orchard/apple/plum.txt", "new below")     // new file
	writeFile(t, root, "pasture/clover/hay.txt", "hay, but more") // size differs
	if err := os.Remove(filepath.Join(root, "marsh/reed/heron.txt")); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour) // same size, new mtime
	if err := os.Chtimes(filepath.Join(root, "meadow/brook/trout.txt"), later, later); err != nil {
		t.Fatal(err)
	}

	got, err := dirDriftResults(x, context.Background(), root, []string{"orchard", "pasture", "marsh", "quarry", "meadow"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"orchard": StatusDiffers, "pasture": StatusDiffers, "marsh": StatusDiffers, "quarry": StatusIndexed, "meadow": StatusDiffers} {
		if got[name] != want {
			t.Errorf("drift[%s] = %q, want %q", name, got[name], want)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := dirDriftResults(x, ctx, root, []string{"orchard"}, 0); err == nil {
		t.Fatal("a cancelled drift pass must return the context error")
	}
}

func TestDirDriftDepthLimit(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "willow/shallow/reed.txt", "reed")
	writeFile(t, root, "willow/a/b/c/lily.txt", "lily")
	writeFile(t, root, "thistle/a/b/c/d/moss.txt", "moss")
	writeFile(t, root, "bramble/a/b/c/d/fern.txt", "fern")
	overlayFixture(t, x, root, "willow/shallow/reed.txt", "willow/a/b/c/lily.txt", "thistle/a/b/c/d/moss.txt", "bramble/a/b/c/d/fern.txt")

	writeFile(t, root, "willow/shallow/sedge.txt", "new within depth") // differs within depth 3
	writeFile(t, root, "bramble/a/b/c/d/rush.txt", "new below depth")  // only visible unlimited
	names := []string{"willow", "thistle", "bramble"}

	got, err := dirDriftResults(x, context.Background(), root, names, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"willow": StatusDiffers, "thistle": StatusUnchecked, "bramble": StatusUnchecked}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("depth 3: drift[%s] = %q, want %q", n, got[n], w)
		}
	}

	// A deep clean tree is indexed once the depth is unlimited; a difference below the limit
	// shows only then.
	got, err = dirDriftResults(x, context.Background(), root, names, 0)
	if err != nil {
		t.Fatal(err)
	}
	want = map[string]string{"willow": StatusDiffers, "thistle": StatusIndexed, "bramble": StatusDiffers}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("unlimited: drift[%s] = %q, want %q", n, got[n], w)
		}
	}
}

func TestStatusOfSymlinksFollowsTheTarget(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "targets/reed.txt", "reed")
	writeFile(t, root, "targets/moss.txt", "moss")
	writeFile(t, root, "targets/fern.txt", "fern")
	writeFile(t, root, "targets/sedge.txt", "sedge")
	link := func(name, target string) {
		t.Helper()
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(root, "links", name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "links"), 0o755); err != nil {
		t.Fatal(err)
	}
	link("same.txt", "targets/reed.txt")
	link("grown.txt", "targets/moss.txt")
	link("unrecorded.txt", "targets/fern.txt")
	link("dangling-recorded.txt", "targets/sedge.txt")
	link("dangling-unrecorded.txt", "targets/nowhere.txt")
	link("to-dir", "targets")
	overlayFixture(t, x, root, "links/same.txt", "links/grown.txt", "links/dangling-recorded.txt")
	writeFile(t, root, "targets/moss.txt", "moss, grown")
	if err := os.Remove(filepath.Join(root, "targets/sedge.txt")); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"same.txt":              StatusIndexed,
		"grown.txt":             StatusChanged,
		"unrecorded.txt":        StatusNew,
		"dangling-recorded.txt": StatusMissing,
	}
	linksDir := filepath.Join(root, "links")
	dir, _ := pathloc.File(linksDir)
	out, err := x.Overlay(context.Background(), dir, listLocal(t, linksDir))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range out {
		got[e.Name] = e.IndexStatus
	}
	for name, st := range want {
		if got[name] != st {
			t.Errorf("local panel: %s = %q, want %q", name, got[name], st)
		}
	}
	for _, name := range []string{"dangling-unrecorded.txt", "to-dir"} {
		if got[name] != "" {
			t.Errorf("local panel: %s = %q, want no status", name, got[name])
		}
	}

	// A fileindex:// panel lists the index's own entries as plain files: the row is stat'ed.
	d, _ := x.DB()
	var entries []localfs.Entry
	for name := range want {
		entries = append(entries, localfs.Entry{Name: name, Type: localfs.EntryFile})
	}
	entries = append(entries, localfs.Entry{Name: "dangling-unrecorded.txt", Type: localfs.EntryFile}, localfs.Entry{Name: "to-dir", Type: localfs.EntryDirectory})
	rules, _, _ := x.scanSettings()
	byName, err := entryStatuses(context.Background(), d, linksDir, entries, false, rules)
	if err != nil {
		t.Fatal(err)
	}
	for name, st := range want {
		if byName[name] != st {
			t.Errorf("index panel: %s = %q, want %q", name, byName[name], st)
		}
	}
	for _, name := range []string{"dangling-unrecorded.txt", "to-dir"} {
		if byName[name] != "" {
			t.Errorf("index panel: %s = %q, want no status", name, byName[name])
		}
	}
}

// dirDriftResults collects what DirDrift yields.
func dirDriftResults(x *Index, ctx context.Context, host string, subdirs []string, maxDepth int) (map[string]string, error) {
	out := map[string]string{}
	err := x.DirDrift(ctx, host, subdirs, maxDepth, func(name, status string) { out[name] = status })
	return out, err
}

// DirDrift hands over each directory when its walk ends, so a cancel keeps what came before it.
func TestDirDriftYieldsEachDirectoryAsItFinishes(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "orchard/apple/pear.txt", "pear")
	writeFile(t, root, "quarry/stone/slate.txt", "slate")
	overlayFixture(t, x, root, "orchard/apple/pear.txt", "quarry/stone/slate.txt")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []string
	err := x.DirDrift(ctx, root, []string{"orchard", "quarry"}, 0, func(name, status string) {
		got = append(got, name+"="+status)
		cancel()
	})
	if err == nil {
		t.Fatal("a cancelled drift pass must return the context error")
	}
	if len(got) != 1 || got[0] != "orchard="+StatusIndexed {
		t.Fatalf("yielded %v, want only orchard, before the cancel", got)
	}
}
