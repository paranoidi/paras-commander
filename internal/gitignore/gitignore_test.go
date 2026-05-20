package gitignore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkTreeRootFindsGitDir(t *testing.T) {
	root := initGitRepo(t)
	sub := filepath.Join(root, "src", "pkg")
	mustMkdirAll(t, sub)

	got := WorkTreeRoot(sub)
	if got != root {
		t.Fatalf("WorkTreeRoot(%q) = %q, want %q", sub, got, root)
	}
}

func TestWorkTreeRootOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	if got := WorkTreeRoot(dir); got != "" {
		t.Fatalf("WorkTreeRoot(%q) = %q, want empty", dir, got)
	}
}

func TestMatcherIgnoresListedPaths(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "ignored.txt\nbuild/\n")
	mustWriteFile(t, filepath.Join(root, "ignored.txt"), "x")
	mustMkdirAll(t, filepath.Join(root, "build", "out"))
	mustWriteFile(t, filepath.Join(root, "build", "out", "a.txt"), "a")
	mustWriteFile(t, filepath.Join(root, "visible.txt"), "v")

	cache := NewCache()
	m, err := cache.MatcherForDir(root)
	if err != nil {
		t.Fatalf("MatcherForDir: %v", err)
	}
	if m == nil {
		t.Fatal("matcher = nil, want non-nil")
	}

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{filepath.Join(root, "ignored.txt"), false, true},
		{filepath.Join(root, "build"), true, true},
		{filepath.Join(root, "build", "out"), true, true},
		{filepath.Join(root, "build", "out", "a.txt"), false, true},
		{filepath.Join(root, "visible.txt"), false, false},
	}
	for _, tc := range cases {
		if got := m.Ignored(tc.path, tc.isDir); got != tc.want {
			t.Errorf("Ignored(%q, isDir=%v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
		}
	}
}

func TestMatcherNestedGitignore(t *testing.T) {
	root := initGitRepo(t)
	sub := filepath.Join(root, "sub")
	mustMkdirAll(t, sub)
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "*.log\n")
	mustWriteFile(t, filepath.Join(sub, ".gitignore"), "local.tmp\n")
	mustWriteFile(t, filepath.Join(sub, "local.tmp"), "x")
	mustWriteFile(t, filepath.Join(sub, "app.log"), "log")
	mustWriteFile(t, filepath.Join(root, "top.log"), "log")

	cache := NewCache()
	m, err := cache.MatcherForDir(sub)
	if err != nil {
		t.Fatalf("MatcherForDir: %v", err)
	}

	if !m.Ignored(filepath.Join(sub, "local.tmp"), false) {
		t.Fatal("expected local.tmp ignored in sub")
	}
	if !m.Ignored(filepath.Join(sub, "app.log"), false) {
		t.Fatal("expected app.log ignored via root *.log")
	}
	if !m.Ignored(filepath.Join(root, "top.log"), false) {
		t.Fatal("top.log should be ignored via root *.log even when matcher is built for sub")
	}
	mRoot, err := cache.MatcherForDir(root)
	if err != nil {
		t.Fatalf("MatcherForDir(root): %v", err)
	}
	if !mRoot.Ignored(filepath.Join(root, "top.log"), false) {
		t.Fatal("expected top.log ignored at repo root")
	}
}

func TestMatcherNegation(t *testing.T) {
	root := initGitRepo(t)
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "*.log\n!important.log\n")
	mustWriteFile(t, filepath.Join(root, "a.log"), "a")
	mustWriteFile(t, filepath.Join(root, "important.log"), "i")

	cache := NewCache()
	m, err := cache.MatcherForDir(root)
	if err != nil {
		t.Fatalf("MatcherForDir: %v", err)
	}
	if !m.Ignored(filepath.Join(root, "a.log"), false) {
		t.Fatal("expected a.log ignored")
	}
	if m.Ignored(filepath.Join(root, "important.log"), false) {
		t.Fatal("expected important.log not ignored after negation")
	}
}

func TestCacheInvalidatesOnGitignoreChange(t *testing.T) {
	root := initGitRepo(t)
	ignorePath := filepath.Join(root, ".gitignore")
	mustWriteFile(t, filepath.Join(root, "tracked.txt"), "t")

	cache := NewCache()
	m1, err := cache.MatcherForDir(root)
	if err != nil {
		t.Fatalf("MatcherForDir: %v", err)
	}
	if m1.Ignored(filepath.Join(root, "tracked.txt"), false) {
		t.Fatal("tracked.txt should not be ignored initially")
	}

	time.Sleep(10 * time.Millisecond)
	mustWriteFile(t, ignorePath, "tracked.txt\n")

	m2, err := cache.MatcherForDir(root)
	if err != nil {
		t.Fatalf("MatcherForDir after edit: %v", err)
	}
	if !m2.Ignored(filepath.Join(root, "tracked.txt"), false) {
		t.Fatal("tracked.txt should be ignored after .gitignore update")
	}
}

func TestMatcherForDirOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache()
	m, err := cache.MatcherForDir(dir)
	if err != nil {
		t.Fatalf("MatcherForDir: %v", err)
	}
	if m != nil {
		t.Fatalf("matcher = %v, want nil outside repo", m)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("Mkdir .git: %v", err)
	}
	return root
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
