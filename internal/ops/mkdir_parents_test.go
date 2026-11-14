package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteMkdirParentsCreatesNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "willow", "harbor", "beacon")
	if got := topmostMissing(target); got != filepath.Join(root, "willow") {
		t.Fatalf("topmostMissing = %q", got)
	}
	plan := MkdirPlan{Path: target, Parents: true}
	if err := ExecuteMkdir(context.Background(), plan); err != nil {
		t.Fatalf("ExecuteMkdir: %v", err)
	}
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		t.Fatalf("not created: %v", err)
	}
	if got := topmostMissing(target); got != "" {
		t.Fatalf("topmostMissing after create = %q", got)
	}
}
