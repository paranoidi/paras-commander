package find

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fswalk"
)

func TestFindWalkDoesNotStatSizeDuringIndex(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "sized.txt")
	content := []byte("hello-find-size")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sess := Start(context.Background(), root, Options{}, fswalk.Params{InitialWorkers: 1, MaxWorkers: 32, AdaptIntervalMS: 2000})
	defer sess.Close()
	var got int64
	for batch := range sess.Results() {
		for _, e := range batch {
			if e.RelLine == "sized.txt" {
				got = e.Size
			}
		}
	}
	<-sess.Done()
	if got != 0 {
		t.Fatalf("Size during walk = %d, want 0 (lazy stat on mark)", got)
	}
}
