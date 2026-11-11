package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExactDestCopyToRemoteTarget(t *testing.T) {
	mem := newMemRemote("/quarry", "/thicket")
	installOpsBackend(t, mem)
	src := filepath.Join(t.TempDir(), "willow.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(dst string, r ConflictResolver) error {
		_, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(memRemoteHost+dst), Options{CopyBufferKiB: 4, ExactDest: true}, ProgressEmitThrottle{}, nil, r, nil)
		return err
	}

	if err := run("/quarry/reed.txt", nil); err != nil { // missing target: written under its own name
		t.Fatal(err)
	}
	if got, ok := mem.file("/quarry/reed.txt"); !ok || got != "payload" {
		t.Fatalf("remote target = %q, %v", got, ok)
	}

	mem.dirs[memRemoteHost+"/thicket/willow.txt"] = true // a directory sits at the target
	var seen []FileConflictFacts
	skip := func(_ context.Context, _, _ string, f FileConflictFacts) (ConflictResolution, error) {
		seen = append(seen, f)
		return ConflictResolution{Action: ActionSkip}, nil
	}
	if err := run("/thicket/willow.txt", skip); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || !seen[0].OverwriteUnsafe() || !seen[0].DestIsDir() {
		t.Fatalf("resolver facts = %+v, want one directory mismatch", seen)
	}
	if _, ok := mem.file("/thicket/willow.txt/willow.txt"); ok {
		t.Fatal("source landed inside the remote directory")
	}
}
