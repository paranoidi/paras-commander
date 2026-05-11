package diskusage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
)

func TestWalkFolderFlatSizes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("xy"), 0o644); err != nil {
		t.Fatal(err)
	}

	tree := diskusage.WalkFolder(root, nil, nil, nil, 4)
	got := map[string]int64{}
	diskusage.FlattenSizes(tree, got)

	if _, ok := got[filepath.Clean(root)]; !ok {
		t.Fatalf("missing root key in %#v", got)
	}
	if got[filepath.Clean(root)] < 7 {
		t.Fatalf("aggregate too small %d", got[filepath.Clean(root)])
	}
}
