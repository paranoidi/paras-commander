package fileindex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/paranoidi/fileindex/scan"
)

// diffsOf runs driftWalk over root to the end and returns "state name" per difference, relative to root.
func diffsOf(t *testing.T, x *Index, root string, depth int) []string {
	t.Helper()
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	rules, _, err := x.scanSettings()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	if _, err := driftWalk(context.Background(), d, rules, root, depth, nil, func(df Difference) bool {
		rel, _ := filepath.Rel(root, df.Path)
		out = append(out, df.State+" "+rel)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func TestDriftWalkScenarios(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	files := map[string][]string{ // directory -> files recorded in the index
		"clean":                  {"apple.txt"},
		"new-file":               {"pear.txt"},
		"changed-size":           {"plum.txt"},
		"changed-mtime":          {"fig.txt"},
		"missing-file":           {"lime.txt", "kiwi.txt"},
		"missing-subdir":         {"sub/date.txt"},
		"hidden-new":             {"nut.txt"},
		"symlink-new":            {"rye.txt"},
		"symlink-dangling":       {"oat.txt"},
		"symlink-target-changed": {"bean.txt"},
		"excluded-recorded":      {"node_modules/parcel/hay.txt"},
		"unreadable":             {"locked/straw.txt"},
	}
	var rels []string
	for dir, names := range files {
		for _, n := range names {
			writeFile(t, root, filepath.Join(dir, n), "first body")
			rels = append(rels, filepath.Join(dir, n))
		}
	}
	// Symlinks recorded under their own names, pointing at a file outside their directory.
	writeFile(t, root, "targets/reed.txt", "reed")
	writeFile(t, root, "targets/moss.txt", "moss")
	for dir, target := range map[string]string{"symlink-dangling": "moss.txt", "symlink-target-changed": "reed.txt"} {
		if err := os.Symlink(filepath.Join(root, "targets", target), filepath.Join(root, dir, "link.txt")); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, filepath.Join(dir, "link.txt"))
	}
	overlayFixture(t, x, root, rels...)

	writeFile(t, root, "new-file/quince.txt", "added")
	writeFile(t, root, "changed-size/plum.txt", "first body, longer now")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "changed-mtime/fig.txt"), later, later); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"missing-file/lime.txt", "missing-subdir/sub"} {
		if err := os.RemoveAll(filepath.Join(root, p)); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, root, "hidden-new/.secret", "hidden")
	if err := os.Symlink(filepath.Join(root, "targets/reed.txt"), filepath.Join(root, "symlink-new/link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "targets/moss.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "targets/reed.txt", "reed, rewritten and longer")
	if os.Geteuid() != 0 {
		defer func() { _ = os.Chmod(filepath.Join(root, "unreadable/locked"), 0o755) }()
		if err := os.Chmod(filepath.Join(root, "unreadable/locked"), 0); err != nil {
			t.Fatal(err)
		}
	}

	want := map[string][]string{
		"clean":                  nil,
		"new-file":               {"new quince.txt"},
		"changed-size":           {"changed plum.txt"},
		"changed-mtime":          {"changed fig.txt"},
		"missing-file":           {"missing lime.txt"},
		"missing-subdir":         {"missing sub/date.txt"},
		"hidden-new":             {"new .secret"},
		"symlink-new":            {"new link.txt"},
		"symlink-dangling":       {"missing link.txt"},
		"symlink-target-changed": {"changed link.txt"},
		"excluded-recorded":      nil, // still on disk, only the scan rules skip it
		"unreadable":             nil, // still on disk, only unreadable
	}
	for dir, w := range want {
		if got := diffsOf(t, x, filepath.Join(root, dir), 0); !slices.Equal(got, w) {
			t.Errorf("%s: differences = %q, want %q", dir, got, w)
		}
	}
	// The icon is the same walk: any difference is "differs".
	got, err := dirDriftResults(x, context.Background(), root, []string{"clean", "new-file", "excluded-recorded", "unreadable", "symlink-dangling"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for dir, st := range map[string]string{"clean": StatusIndexed, "new-file": StatusDiffers, "excluded-recorded": StatusIndexed, "unreadable": StatusIndexed, "symlink-dangling": StatusDiffers} {
		if got[dir] != st {
			t.Errorf("DirDrift[%s] = %q, want %q", dir, got[dir], st)
		}
	}
}

func TestDriftWalkDepth(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "deep/a/b/c/d/moss.txt", "moss")
	overlayFixture(t, x, root, "deep/a/b/c/d/moss.txt")
	writeFile(t, root, "deep/a/b/c/d/fern.txt", "below the limit")
	if err := os.Remove(filepath.Join(root, "deep/a/b/c/d/moss.txt")); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "deep")
	// Within depth 3 nothing is seen, and the recorded file deeper down is not compared.
	if got := diffsOf(t, x, deep, 3); len(got) != 0 {
		t.Errorf("depth 3 differences = %q, want none", got)
	}
	d, _ := x.DB()
	rules, _, _ := x.scanSettings()
	deeper, err := driftWalk(context.Background(), d, rules, deep, 3, nil, func(Difference) bool { return true })
	if err != nil || !deeper {
		t.Errorf("deeper = %v, %v; want true", deeper, err)
	}
	if got := diffsOf(t, x, deep, 0); !slices.Equal(got, []string{"missing a/b/c/d/moss.txt", "new a/b/c/d/fern.txt"}) {
		t.Errorf("unlimited differences = %q", got)
	}
	if got, _ := dirDriftResults(x, context.Background(), root, []string{"deep"}, 3); got["deep"] != StatusUnchecked {
		t.Errorf("DirDrift depth 3 = %q, want unchecked", got["deep"])
	}
}

func TestDriftWalkStopSkipsMissingCheck(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "orchard/gone.txt", "gone")
	overlayFixture(t, x, root, "orchard/gone.txt")
	if err := os.Remove(filepath.Join(root, "orchard/gone.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "orchard/added.txt", "added")
	d, _ := x.DB()
	rules, _, _ := x.scanSettings()
	var got []string
	if _, err := driftWalk(context.Background(), d, rules, filepath.Join(root, "orchard"), 0, nil, func(df Difference) bool {
		got = append(got, df.State)
		return false
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{DiffNew}) {
		t.Fatalf("yielded %q after yield returned false, want only the first difference", got)
	}
}

func TestDriftWalkLinkBackToRootIsNotWalked(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "orchard/apple.txt", "apple")
	overlayFixture(t, x, root, "orchard/apple.txt")
	orchard := filepath.Join(root, "orchard")
	if err := os.Symlink(orchard, filepath.Join(orchard, "loop")); err != nil {
		t.Fatal(err)
	}
	d, _ := x.DB()
	rules := scan.Rules{FollowSymlinks: true}
	var got []string
	if _, err := driftWalk(context.Background(), d, rules, orchard, 3, nil, func(df Difference) bool {
		got = append(got, df.Path)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("a link back to the root yielded %q, want nothing", got)
	}
}

func TestDriftWalkNotIndexed(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	d, _ := x.DB()
	if _, err := driftWalk(context.Background(), d, scan.Rules{}, root, 0, nil, func(Difference) bool { return true }); err != ErrNotIndexed {
		t.Fatalf("err = %v, want ErrNotIndexed", err)
	}
}

func TestDifferencesTaskSortedAndDoneOnCancel(t *testing.T) {
	root := tree(t)
	x := testIndex(t, initIndex(t))
	writeFile(t, root, "orchard/pear.txt", "pear")
	overlayFixture(t, x, root, "orchard/pear.txt")
	writeFile(t, root, "orchard/zebra.txt", "z")
	writeFile(t, root, "orchard/apple.txt", "a")
	if err := os.Remove(filepath.Join(root, "orchard/pear.txt")); err != nil {
		t.Fatal(err)
	}
	var got []Difference
	var gotErr error
	calls := 0
	task := x.DifferencesTask(filepath.Join(root, "orchard"), func(d []Difference, err error) { got, gotErr, calls = d, err, calls+1 })
	if err := task.Run(context.Background(), &recReporter{}); err != nil || calls != 1 || gotErr != nil {
		t.Fatalf("run = %v, done calls %d, done err %v", err, calls, gotErr)
	}
	var paths []string
	for _, d := range got {
		paths = append(paths, d.State+" "+filepath.Base(d.Path))
	}
	if want := []string{"new apple.txt", "missing pear.txt", "new zebra.txt"}; !slices.Equal(paths, want) {
		t.Fatalf("differences = %q, want %q (sorted by path)", paths, want)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if err := task.Run(ctx, &recReporter{}); err == nil || calls != 1 || gotErr == nil {
		t.Fatalf("canceled run = %v, done calls %d, done err %v; want an error and one done call", err, calls, gotErr)
	}

	// A directory the index lacks ends the job normally and says so through done.
	calls = 0
	missing := x.DifferencesTask(filepath.Join(root, "nowhere"), func(_ []Difference, err error) { gotErr, calls = err, calls+1 })
	if err := missing.Run(context.Background(), &recReporter{}); err != nil || calls != 1 || gotErr != ErrNotIndexed {
		t.Fatalf("not indexed: run %v, done calls %d, err %v", err, calls, gotErr)
	}
}
