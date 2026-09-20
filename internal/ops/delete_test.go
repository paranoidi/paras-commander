package ops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/paranoidi/paras-commander/internal/fsbackend/file"
)

// English-word basenames for generated trees (repo convention: never use local filenames).
var deleteTreeWords = []string{
	"meadow", "harbor", "lantern", "copper", "willow", "orchard",
	"pebble", "thicket", "bramble", "garden", "acorn", "wombat",
	"cinder", "ripple", "falcon", "grove", "nectar", "plover",
	"quarry", "sundew",
}

func buildLargeDeleteTree(t *testing.T) (root string, entries int) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "thicket")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	const dirs = 16
	const filesPerDir = 32
	for i := 0; i < dirs; i++ {
		dir := filepath.Join(root, fmt.Sprintf("%s-%02d", deleteTreeWords[i%len(deleteTreeWords)], i))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", dir, err)
		}
		entries++
		for j := 0; j < filesPerDir; j++ {
			name := fmt.Sprintf("%s-%02d.txt", deleteTreeWords[(i+j)%len(deleteTreeWords)], j)
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
				t.Fatalf("write %q: %v", path, err)
			}
			entries++
		}
	}
	return root, entries
}

func countTreeEntries(t *testing.T, root string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if path == root {
			return nil
		}
		n++
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk %q: %v", root, err)
	}
	return n
}

func waitUntilTreeShrinks(t *testing.T, root string, initial int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countTreeEntries(t, root) < initial {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout waiting for recursive delete to start removing children")
}

func TestExecuteDeletePathsCancelMidRecursiveWalk(t *testing.T) {
	t.Parallel()
	root, initial := buildLargeDeleteTree(t)
	if initial < 100 {
		t.Fatalf("generated tree entries = %d, want a large tree", initial)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, _, err := ExecuteDeletePaths(ctx, []string{root}, nil)
		done <- err
	}()

	waitUntilTreeShrinks(t, root, initial)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("delete did not return promptly after cancel")
	}

	remaining := countTreeEntries(t, root)
	if remaining == 0 {
		t.Fatal("entire tree was deleted after cancel; walk should have stopped mid-tree")
	}
}
