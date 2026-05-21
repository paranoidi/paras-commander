package gitstatus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePorcelainUntrackedAndModified(t *testing.T) {
	root := t.TempDir()
	stdout := "?? new.txt\n M changed.txt\n"
	entries := parsePorcelain(stdout, root)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	wantNew := filepath.Join(root, "new.txt")
	wantMod := filepath.Join(root, "changed.txt")
	var gotNew, gotMod *entry
	for i := range entries {
		switch entries[i].path {
		case wantNew:
			gotNew = &entries[i]
		case wantMod:
			gotMod = &entries[i]
		}
	}
	if gotNew == nil || gotNew.staged != NotModified || gotNew.unstaged != New {
		t.Fatalf("new.txt cell = %+v, want - N", gotNew)
	}
	if gotMod == nil || gotMod.staged != NotModified || gotMod.unstaged != Modified {
		t.Fatalf("changed.txt cell = %+v, want - M", gotMod)
	}
}

func TestDirAggregationModifiedChild(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	entries := parsePorcelain(" M sub/file.txt\n", root)
	sn := &snapshot{entries: entries}
	cell := sn.dirCell(sub)
	if cell.Unstaged != Modified {
		t.Fatalf("dir unstaged = %v, want Modified", cell.Unstaged)
	}
}

func TestCacheStatusesForListingRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := initGitRepo(t)
	tracked := filepath.Join(root, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "init")
	if err := os.WriteFile(tracked, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	untracked := filepath.Join(root, "fresh.txt")
	if err := os.WriteFile(untracked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	GitCommand = func(ctx context.Context, workRoot string) (string, error) {
		return defaultGitCommand(ctx, workRoot)
	}
	cache := NewCache()
	paths := []ListingPaths{
		{AbsPath: tracked, IsDir: false},
		{AbsPath: untracked, IsDir: false},
		{AbsPath: root, IsDir: true},
	}
	m, err := cache.StatusesForListing(context.Background(), root, root, paths)
	if err != nil {
		t.Fatal(err)
	}
	if m[tracked].Unstaged != Modified {
		t.Fatalf("tracked = %+v, want unstaged M", m[tracked])
	}
	if m[untracked].Unstaged != New {
		t.Fatalf("untracked = %+v, want unstaged N", m[untracked])
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "test")
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
