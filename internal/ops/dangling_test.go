package ops

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestDanglingDirsAfterCascadeStopsAtSurvivingContent builds:
//
//	garden/keepme.txt        (survives — garden must NOT be reported)
//	garden/orchard/meadow/acorn.txt   (removed)
//	garden/orchard/meadow/pebble.txt  (removed)
//
// After removing the two files, meadow and orchard are both fully empty, but garden
// still has keepme.txt. DanglingDirsAfter must report only the topmost emptied dir
// (orchard), not the deeper meadow (covered by orchard's recursive removal) and not
// garden (has surviving content).
func TestDanglingDirsAfterCascadeStopsAtSurvivingContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	garden := filepath.Join(root, "garden")
	orchard := filepath.Join(garden, "orchard")
	meadow := filepath.Join(orchard, "meadow")
	if err := os.MkdirAll(meadow, 0o755); err != nil {
		t.Fatal(err)
	}
	keepme := filepath.Join(garden, "keepme.txt")
	if err := os.WriteFile(keepme, []byte("stays"), 0o644); err != nil {
		t.Fatal(err)
	}
	acorn := filepath.Join(meadow, "acorn.txt")
	pebble := filepath.Join(meadow, "pebble.txt")
	if err := os.WriteFile(acorn, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pebble, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate the completed move/delete: the files are already gone by the time
	// DanglingDirsAfter runs; only their former paths (job.Sources) are passed in.
	if err := os.Remove(acorn); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(pebble); err != nil {
		t.Fatal(err)
	}

	// A source whose parent directory no longer exists at all (the operation removed
	// the whole subtree) must be skipped without failing the other chains.
	vanished := filepath.Join(root, "thicket", "bramble.txt")

	sources := []pathloc.Path{pathloc.FileMust(acorn), pathloc.FileMust(pebble), pathloc.FileMust(vanished)}
	got, err := DanglingDirsAfter(context.Background(), sources)
	if err != nil {
		t.Fatalf("DanglingDirsAfter: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d dirs, want 1: %v", len(got), got)
	}
	if got[0].String() != pathloc.FileMust(orchard).String() {
		t.Fatalf("got %v, want topmost dir %v (not the deeper meadow, not garden which still has keepme.txt)", got[0], orchard)
	}
}

func TestEmptyDirRootsClimbsToStop(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	at := func(rel string) string { return filepath.Join(root, rel) }
	loc := func(rel string) pathloc.Path { return pathloc.MustParse(at(rel)) }
	write := func(rel string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(at(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at(rel), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("scan/harbor/deep/inner/fern.txt")
	write("scan/harbor/deep/other/reed.txt")
	write("scan/orchard/north/clover.txt")
	write("scan/keep.txt")
	if err := os.MkdirAll(at("scan/harbor/deep/hollow/void"), 0o755); err != nil {
		t.Fatal(err)
	}
	preview := func(dirs, stops []pathloc.Path, removed ...string) []string {
		t.Helper()
		set := map[string]bool{}
		for _, r := range removed {
			set[loc(r).String()] = true
		}
		roots, err := EmptyDirRoots(ctx, dirs, stops, set)
		if err != nil {
			t.Fatal(err)
		}
		got, err := PreviewEmptyDirsUnder(ctx, roots, set)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(got))
		for i, d := range got {
			out[i], _ = filepath.Rel(root, d.String())
		}
		slices.Sort(out)
		return out
	}
	stop := []pathloc.Path{loc("scan")}

	// A sibling with a file keeps the parents.
	if got := preview([]pathloc.Path{loc("scan/harbor/deep/inner")}, stop, "scan/harbor/deep/inner/fern.txt"); !slices.Equal(got, []string{"scan/harbor/deep/inner"}) {
		t.Fatalf("sibling stays: %v", got)
	}
	// All the way up, any number of levels, an already empty sibling included; the stop holds other content.
	want := []string{"scan/harbor", "scan/harbor/deep", "scan/harbor/deep/hollow", "scan/harbor/deep/hollow/void", "scan/harbor/deep/inner", "scan/harbor/deep/other"}
	if got := preview([]pathloc.Path{loc("scan/harbor/deep/inner"), loc("scan/harbor/deep/other")}, stop, "scan/harbor/deep/inner/fern.txt", "scan/harbor/deep/other/reed.txt"); !slices.Equal(got, want) {
		t.Fatalf("climb: %v, want %v", got, want)
	}
	// Never above the stop, though everything up there would be empty too.
	got := preview([]pathloc.Path{loc("scan/orchard/north")}, []pathloc.Path{loc("scan/orchard/north")}, "scan/orchard/north/clover.txt")
	if !slices.Equal(got, []string{"scan/orchard/north"}) {
		t.Fatalf("at the stop: %v", got)
	}
	// Under no stop: no climb, as before.
	if got := preview([]pathloc.Path{loc("scan/orchard/north")}, nil, "scan/orchard/north/clover.txt"); !slices.Equal(got, []string{"scan/orchard/north"}) {
		t.Fatalf("no stop: %v", got)
	}
	// The stop itself goes when all of it is emptied.
	write("whole/a/b/moss.txt")
	if got := preview([]pathloc.Path{loc("whole/a/b")}, []pathloc.Path{loc("whole")}, "whole/a/b/moss.txt"); !slices.Equal(got, []string{"whole", "whole/a", "whole/a/b"}) {
		t.Fatalf("whole: %v", got)
	}

	// The removal does what the preview said, once the files are gone.
	for _, f := range []string{"scan/harbor/deep/inner/fern.txt", "scan/harbor/deep/other/reed.txt"} {
		if err := os.Remove(at(f)); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := EmptyDirRoots(ctx, []pathloc.Path{loc("scan/harbor/deep/inner"), loc("scan/harbor/deep/other")}, stop, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveEmptyDirsUnder(ctx, roots); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(at("scan/harbor")); !os.IsNotExist(err) {
		t.Fatalf("harbor still there: %v", err)
	}
	if _, err := os.Stat(at("scan/keep.txt")); err != nil {
		t.Fatal(err)
	}
}

// The empty-directories job only ever uses rmdir: what gained a file since the check stays.
func TestExecuteRemoveEmptyDirsNeverDeletesFiles(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	at := func(rel string) string { return filepath.Join(root, rel) }
	for _, d := range []string{"empty/a/b", "arrived/hollow", "arrived/busy", "linked-target/inner"} {
		if err := os.MkdirAll(at(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Arrived while the prompt was open.
	if err := os.WriteFile(at("arrived/busy/download.part"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at("a-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(at("linked-target"), at("a-link")); err != nil {
		t.Fatal(err)
	}
	paths := []string{at("empty"), at("arrived"), at("gone"), at("a-file"), at("a-link")}
	results := map[string]string{}
	started := 0
	done, err := ExecuteRemoveEmptyDirs(ctx, paths, nil, func(item PlanItem, res ItemResult, reason string) {
		if res == ItemDone {
			reason = "done"
		}
		results[item.Src.Base()] = reason
	}, func(int32) { started++ })
	if err != nil {
		t.Fatal(err)
	}
	if done != 2 || started != len(paths) {
		t.Fatalf("done = %d started = %d", done, started)
	}
	want := map[string]string{"empty": "done", "gone": "done", "arrived": ReasonNotEmpty, "a-file": ReasonNotEmpty, "a-link": ReasonNotEmpty}
	for name, reason := range want {
		if results[name] != reason {
			t.Errorf("%s: %q, want %q", name, results[name], reason)
		}
	}
	for _, kept := range []string{"arrived/busy/download.part", "a-file", "a-link", "linked-target/inner"} {
		if _, err := os.Lstat(at(kept)); err != nil {
			t.Errorf("%s went: %v", kept, err)
		}
	}
	for _, gone := range []string{"empty", "arrived/hollow"} {
		if _, err := os.Lstat(at(gone)); !os.IsNotExist(err) {
			t.Errorf("%s stayed: %v", gone, err)
		}
	}
}

func TestExecuteRemoveEmptyDirsCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := t.TempDir()
	if _, err := ExecuteRemoveEmptyDirs(ctx, []string{root}, nil, nil, nil); err == nil {
		t.Fatal("no error from a canceled context")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}

func TestRmdirRefusesAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "meadow.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := localfs.Rmdir(f); err == nil {
		t.Fatal("rmdir on a file succeeded")
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatal(err)
	}
}

// A directory it may not remove is retried as root with rmdir, never rm -rf.
func TestExecuteRemoveEmptyDirsElevatesWithRmdir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root is never denied")
	}
	parent := filepath.Join(t.TempDir(), "harbor")
	dir := filepath.Join(parent, "locked")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionElevate, Password: "heron"}}}
	done, err := ExecuteRemoveEmptyDirs(h.ctx(true), []string{dir}, nil, nil, nil)
	if err != nil || done != 1 {
		t.Fatalf("done = %d err = %v", done, err)
	}
	if len(h.runs) != 1 || !slices.Equal(h.runs[0], []string{"rmdir", "--", dir}) {
		t.Fatalf("ran %v, want rmdir", h.runs)
	}
}
