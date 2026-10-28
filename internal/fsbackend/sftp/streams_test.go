package sftp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenReadSeeksWithoutReadingPrefix(t *testing.T) {
	root := t.TempDir()
	content := []byte("harbor-lantern-meadow-quartz")
	if err := os.WriteFile(filepath.Join(root, "orchard.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{workDir: root})
	p := srv.newPool(time.Hour)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	rc, err := b.OpenRead(context.Background(), srv.fileLoc(filepath.Join(root, "orchard.txt")))
	if err != nil {
		t.Fatalf("OpenRead: %v", err)
	}
	defer func() { _ = rc.Close() }()
	sk, ok := rc.(io.Seeker)
	if !ok {
		t.Fatal("OpenRead reader does not implement io.Seeker")
	}
	const offset = 14
	if pos, err := sk.Seek(offset, io.SeekStart); err != nil || pos != offset {
		t.Fatalf("Seek = %d, %v; want %d", pos, err, offset)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content[offset:]) {
		t.Fatalf("read after seek = %q, want %q", got, content[offset:])
	}
}
