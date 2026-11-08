package fileindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ghosts lists dir through Overlay and returns the names of its ghost rows.
func ghosts(t *testing.T, x *Index, dir string) []string {
	t.Helper()
	loc, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := x.Overlay(context.Background(), loc, listLocal(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range out {
		if e.IndexStatus == StatusMissing {
			names = append(names, e.Name)
		}
	}
	return names
}

// TestNoGhostWhileChangeWaitsToApply lists a directory while a change pc reported has not reached
// the index yet (a scan or a long commit holding the write): the record is still there, and the
// row must not show as a ghost, neither before nor after the change lands.
func TestNoGhostWhileChangeWaitsToApply(t *testing.T) {
	for _, tc := range []struct {
		name   string
		onDisk func(root string) error
		change func(root string) fschange.Change
	}{
		{
			name:   "removed file",
			onDisk: func(root string) error { return os.Remove(filepath.Join(root, "harbor.txt")) },
			change: func(root string) fschange.Change {
				return fschange.Change{Kind: fschange.Removed, Path: filepath.Join(root, "harbor.txt")}
			},
		},
		{
			name:   "removed directory",
			onDisk: func(root string) error { return os.RemoveAll(filepath.Join(root, "lantern")) },
			change: func(root string) fschange.Change {
				return fschange.Change{Kind: fschange.Removed, Path: filepath.Join(root, "lantern"), IsDir: true}
			},
		},
		{
			name: "renamed file",
			onDisk: func(root string) error {
				return os.Rename(filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt"))
			},
			change: func(root string) fschange.Change {
				return fschange.Change{Kind: fschange.Renamed, From: filepath.Join(root, "harbor.txt"), Path: filepath.Join(root, "anchor.txt")}
			},
		},
		{
			name: "renamed directory",
			onDisk: func(root string) error {
				return os.Rename(filepath.Join(root, "lantern"), filepath.Join(root, "beacon"))
			},
			change: func(root string) fschange.Change {
				return fschange.Change{Kind: fschange.Renamed, From: filepath.Join(root, "lantern"), Path: filepath.Join(root, "beacon"), IsDir: true}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, root := scanned(t)
			if err := tc.onDisk(root); err != nil {
				t.Fatal(err)
			}
			x.writeMu.Lock() // the drain cannot commit
			x.Apply([]fschange.Change{tc.change(root)})
			got := ghosts(t, x, root)
			x.writeMu.Unlock()
			if len(got) != 0 {
				t.Fatalf("ghosts while the change waits = %v, want none", got)
			}
			x.WaitApplied()
			if got := ghosts(t, x, root); len(got) != 0 {
				t.Fatalf("ghosts after the change landed = %v, want none", got)
			}
		})
	}
}

// A panel can sit inside a directory whose removal is waiting: the rows below it are gone too.
func TestNoGhostBelowDirectoryWaitingToApply(t *testing.T) {
	x, root := scanned(t)
	lantern := filepath.Join(root, "lantern")
	if err := os.Remove(filepath.Join(lantern, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	x.writeMu.Lock()
	defer x.writeMu.Unlock()
	x.Apply([]fschange.Change{{Kind: fschange.Removed, Path: lantern, IsDir: true}})
	if got := ghosts(t, x, lantern); len(got) != 0 {
		t.Fatalf("ghosts below the removed directory = %v, want none", got)
	}
}

// The panel and the change can name the same file through different paths.
func TestNoGhostThroughSymlinkedPath(t *testing.T) {
	x, root := scanned(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "harbor.txt")); err != nil {
		t.Fatal(err)
	}
	x.writeMu.Lock()
	defer x.writeMu.Unlock()
	x.Apply([]fschange.Change{{Kind: fschange.Removed, Path: filepath.Join(link, "harbor.txt")}})
	if got := ghosts(t, x, root); len(got) != 0 {
		t.Fatalf("ghosts = %v, want none", got)
	}
}

// When the index refuses the change the record stays, so the row is a true ghost again.
func TestGhostShowsWhenApplyFails(t *testing.T) {
	x, root := scanned(t)
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("CREATE TRIGGER refuse BEFORE DELETE ON instance BEGIN SELECT RAISE(ABORT, 'refused'); END"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "harbor.txt")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	applyAndWait(x, []fschange.Change{{Kind: fschange.Removed, Path: file}})
	if _, ok := recorded(t, x, file); !ok {
		t.Fatal("the test needs the removal to fail")
	}
	if got := ghosts(t, x, root); len(got) != 1 || got[0] != "harbor.txt" {
		t.Fatalf("ghosts = %v, want harbor.txt", got)
	}
}

// A change pc did not report leaves the row a ghost, and so does a change to another path.
func TestGhostShowsForFileRemovedOutsidePc(t *testing.T) {
	x, root := scanned(t)
	if err := os.Remove(filepath.Join(root, "harbor.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "lantern", "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	x.writeMu.Lock()
	defer x.writeMu.Unlock()
	x.Apply([]fschange.Change{{Kind: fschange.Removed, Path: filepath.Join(root, "lantern", "meadow.txt")}})
	if got := ghosts(t, x, root); len(got) != 1 || got[0] != "harbor.txt" {
		t.Fatalf("ghosts = %v, want harbor.txt", got)
	}
}

// The same path can be queued twice (removed, created again, removed): the row stays hidden until
// the last change has landed.
func TestNoGhostWhileSamePathQueuedTwice(t *testing.T) {
	x, root := scanned(t)
	file := filepath.Join(root, "harbor.txt")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	gone := gonePaths([]fschange.Change{{Kind: fschange.Removed, Path: file}})
	x.holdGone(gone)
	x.holdGone(gone)
	x.releaseGone(gone)
	if got := ghosts(t, x, root); len(got) != 0 {
		t.Fatalf("ghosts = %v, want none while one change still waits", got)
	}
	x.releaseGone(gone)
	if got := ghosts(t, x, root); len(got) != 1 || got[0] != "harbor.txt" {
		t.Fatalf("ghosts = %v, want harbor.txt once nothing waits", got)
	}
}
