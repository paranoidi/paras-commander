package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// markAsGitWorkTree makes gitignore.ValidWorkTreeRoot(root) resolve, without shelling out to a
// real git binary: it only needs .git/HEAD to exist.
func markAsGitWorkTree(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitStatusForPathUsesPanelMapWhenPresent(t *testing.T) {
	h, fh := newTestHandler(t, 80, 24)
	root := t.TempDir()
	path := filepath.Join(root, "harbor.txt")
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.Primary.GitColumnActive = true
	h.model.Primary.GitByPath = map[string]gitstatus.Cell{
		path: {Unstaged: gitstatus.Modified},
	}
	h.model.Primary.Path = pathloc.MustParse(root)
	fh.peekGitStatus = func(string, string, []gitstatus.ListingPaths) (map[string]gitstatus.Cell, bool) {
		t.Fatal("PeekGitStatus must not run when GitByPath already has the path")
		return nil, false
	}

	got := h.gitStatusForPath(path)
	if got == nil || got.Unstaged != gitstatus.Modified {
		t.Fatalf("gitStatusForPath = %+v, want unstaged Modified from panel map", got)
	}
}

func TestGitStatusForPathUsesCachePeekWhenPanelMapPending(t *testing.T) {
	root := t.TempDir()
	markAsGitWorkTree(t, root)
	path := filepath.Join(root, "meadow.txt")

	h, fh := newTestHandler(t, 80, 24)
	// Simulate CLI open before the panel's async listing/git fetch lands: empty GitByPath and
	// a panel path that is not even the file's parent yet.
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.Primary.GitByPath = nil
	h.model.Primary.Path = pathloc.MustParse(t.TempDir())

	want := gitstatus.Cell{Unstaged: gitstatus.Modified}
	fh.peekGitStatus = func(workRoot, listDir string, paths []gitstatus.ListingPaths) (map[string]gitstatus.Cell, bool) {
		if filepath.Clean(listDir) != filepath.Clean(root) {
			t.Fatalf("listDir = %q, want %q", listDir, root)
		}
		out := make(map[string]gitstatus.Cell, len(paths))
		for _, p := range paths {
			out[p.AbsPath] = want
		}
		return out, true
	}

	got := h.gitStatusForPath(path)
	if got == nil || got.Unstaged != gitstatus.Modified {
		t.Fatalf("gitStatusForPath = %+v, want unstaged Modified from cache peek", got)
	}
}

func TestGitStatusForPathReturnsNilOnCacheMiss(t *testing.T) {
	root := t.TempDir()
	markAsGitWorkTree(t, root)
	path := filepath.Join(root, "meadow.txt")

	h, fh := newTestHandler(t, 80, 24)
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.Primary.GitByPath = nil
	h.model.Primary.Path = pathloc.MustParse(t.TempDir())
	var peekCalls int
	fh.peekGitStatus = func(string, string, []gitstatus.ListingPaths) (map[string]gitstatus.Cell, bool) {
		peekCalls++
		return nil, false
	}

	if got := h.gitStatusForPath(path); got != nil {
		t.Fatalf("gitStatusForPath = %+v, want nil on cache miss (must not block on git)", got)
	}
	if peekCalls != 1 {
		t.Fatalf("PeekGitStatus calls = %d, want 1", peekCalls)
	}
}
