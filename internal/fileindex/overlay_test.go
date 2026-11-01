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

	got, err := x.DirDrift(context.Background(), root, []string{"orchard", "pasture", "marsh", "quarry", "meadow"}, 0)
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
	if _, err := x.DirDrift(ctx, root, []string{"orchard"}, 0); err == nil {
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

	got, err := x.DirDrift(context.Background(), root, names, 3)
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
	got, err = x.DirDrift(context.Background(), root, names, 0)
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
