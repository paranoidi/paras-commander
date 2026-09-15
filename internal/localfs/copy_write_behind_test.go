package localfs

import (
	"bytes"
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// TestCopyFileWriteBehindSync exercises the mid-copy fsync wrapper (SyncWriteBehindBytes):
// content must still match and onWritten's reported bytes must total the file size.
func TestCopyFileWriteBehindSync(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "umbrella-forest-canyon.bin")
	dst := filepath.Join(dir, "lighthouse-meadow-otter.bin")

	const size = 3 << 20 // 3 MiB
	payload := make([]byte, size)
	rand.New(rand.NewSource(1)).Read(payload) //nolint:gosec // test data, not crypto
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	var total int64
	opts := CopyFileOpts{
		SyncPerFile:          true,
		SyncWriteBehindBytes: 1 << 20, // 1 MiB, well below the file size
	}
	err := CopyFile(context.Background(), src, dst, 256<<10, false, false, false, false, opts, func(n int64) {
		total += n
	})
	if err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if total != size {
		t.Fatalf("onWritten total = %d, want %d", total, size)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("copied content does not match source")
	}
}
