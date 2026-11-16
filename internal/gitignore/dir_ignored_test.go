package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheDirIgnored(t *testing.T) {
	t.Parallel()
	if (*Cache)(nil).DirIgnored("/anything") {
		t.Fatal("nil cache should never report excluded")
	}

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored_dir/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoredDir := filepath.Join(root, "ignored_dir")
	if err := os.Mkdir(ignoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plainDir := filepath.Join(root, "plain_dir")
	if err := os.Mkdir(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cache := NewCache()
	if !cache.DirIgnored(ignoredDir) {
		t.Fatal("ignored_dir should be reported as gitignore-excluded")
	}
	if cache.DirIgnored(plainDir) {
		t.Fatal("plain_dir should not be reported as gitignore-excluded")
	}
	if cache.DirIgnored(root) {
		t.Fatal("repo root itself should never be reported as gitignore-excluded")
	}
}
