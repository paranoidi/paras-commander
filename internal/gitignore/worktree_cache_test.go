package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func resetWorkTreeCache(t *testing.T) {
	t.Helper()
	sharedWorkTreeResolver.mu.Lock()
	sharedWorkTreeResolver.entries = make(map[string]workTreeCacheEntry)
	sharedWorkTreeResolver.mu.Unlock()
}

func TestWorkTreeRootCachesDescendantWithoutReWalk(t *testing.T) {
	resetWorkTreeCache(t)
	root := initGitRepo(t)
	sub := filepath.Join(root, "src", "pkg")
	mustMkdirAll(t, sub)

	if got := WorkTreeRoot(root); got != root {
		t.Fatalf("WorkTreeRoot(%q) = %q, want %q", root, got, root)
	}
	if got := WorkTreeRoot(sub); got != root {
		t.Fatalf("WorkTreeRoot(%q) = %q, want %q", sub, got, root)
	}

	sharedWorkTreeResolver.mu.Lock()
	ent, ok := sharedWorkTreeResolver.entries[sub]
	sharedWorkTreeResolver.mu.Unlock()
	if !ok {
		t.Fatal("expected descendant path cached after lookup")
	}
	if ent.workRoot != root {
		t.Fatalf("cached workRoot = %q, want %q", ent.workRoot, root)
	}
}

func TestValidWorkTreeRootCachesDescendant(t *testing.T) {
	resetWorkTreeCache(t)
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main\n")
	sub := filepath.Join(root, "internal", "panel")
	mustMkdirAll(t, sub)

	if got := ValidWorkTreeRoot(root); got != root {
		t.Fatalf("ValidWorkTreeRoot(%q) = %q, want %q", root, got, root)
	}
	if got := ValidWorkTreeRoot(sub); got != root {
		t.Fatalf("ValidWorkTreeRoot(%q) = %q, want %q", sub, got, root)
	}

	sharedWorkTreeResolver.mu.Lock()
	ent, ok := sharedWorkTreeResolver.entries[sub]
	sharedWorkTreeResolver.mu.Unlock()
	if !ok || !ent.metadataValid || ent.workRoot != root {
		t.Fatalf("cached entry = %+v, want valid metadata for %q", ent, root)
	}
}

func TestWorkTreeCacheInvalidatesWhenHEADChanges(t *testing.T) {
	resetWorkTreeCache(t)
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	headPath := filepath.Join(gitDir, "HEAD")
	mustWriteFile(t, headPath, "ref: refs/heads/main\n")

	if got := ValidWorkTreeRoot(root); got != root {
		t.Fatalf("ValidWorkTreeRoot before HEAD edit = %q, want %q", got, root)
	}

	mustWriteFile(t, headPath, "ref: refs/heads/other\n")
	if got := ValidWorkTreeRoot(root); got != root {
		t.Fatalf("ValidWorkTreeRoot after HEAD edit = %q, want %q", got, root)
	}
}
