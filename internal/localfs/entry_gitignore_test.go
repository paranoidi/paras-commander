package localfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/gitignore"
)

func TestListDirHidesGitignoredFiles(t *testing.T) {
	root := initGitRepoForList(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\nbuild/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "ignored.txt"))
	mustMkdir(t, filepath.Join(root, "build"))
	mustWriteFile(t, filepath.Join(root, "visible.txt"))

	cache := gitignore.NewCache()
	matcher, err := MatcherForListing(false, cache, root)
	if err != nil {
		t.Fatalf("MatcherForListing: %v", err)
	}

	listing, err := ListDir(root, ListOptions{ShowHidden: false, Gitignore: matcher})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	names := entryNames(listing.Entries)
	if len(names) != 1 || names[0] != "visible.txt" {
		t.Fatalf("entries = %v, want only visible.txt", names)
	}
}

func TestListDirShowsGitignoredWhenShowHidden(t *testing.T) {
	root := initGitRepoForList(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "ignored.txt"))

	cache := gitignore.NewCache()
	listing, err := ListDir(root, ListOptions{ShowHidden: true, Gitignore: nil})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	names := entryNames(listing.Entries)
	foundIgnored := false
	for _, n := range names {
		if n == "ignored.txt" {
			foundIgnored = true
		}
	}
	if !foundIgnored {
		t.Fatalf("entries = %v, want ignored.txt when ShowHidden", names)
	}
	_ = cache
}

func initGitRepoForList(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir .git: %v", err)
	}
	return root
}
