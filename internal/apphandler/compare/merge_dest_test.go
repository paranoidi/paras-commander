package compare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeCopyDestTargetsFilePathUnlessDirectoryThere(t *testing.T) {
	root := t.TempDir()
	missingParent := filepath.Join(root, "harbor", "lantern.txt")
	if got := mergeCopyDest(missingParent); got != missingParent {
		t.Fatalf("missing parent: dest = %q, want the file path %q", got, missingParent)
	}
	clash := filepath.Join(root, "meadow")
	if err := os.Mkdir(clash, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := mergeCopyDest(clash); got != root {
		t.Fatalf("directory at target: dest = %q, want parent %q", got, root)
	}
}
