package find

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFindWalkCapturesFileSize(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "sized.txt")
	content := []byte("hello-find-size")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sess := Start(context.Background(), root, Options{})
	defer sess.Close()
	var got int64
	for batch := range sess.Results() {
		for _, e := range batch {
			if e.Path == filepath.Clean(path) {
				got = e.Size
			}
		}
	}
	<-sess.Done()
	if got != int64(len(content)) {
		t.Fatalf("Size = %d, want %d", got, len(content))
	}
}
