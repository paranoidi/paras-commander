package sftp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestPoolEvictsDeadSessionAndRetriesList(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{workDir: root})
	p := srv.newPool(time.Hour)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	entries, err := b.List(context.Background(), srv.loc)
	if err != nil {
		t.Fatalf("initial List: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("initial List returned no entries")
	}

	host, err := pathloc.SFTPHostPart(srv.loc)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	first, ok := p.conns[host]
	p.mu.Unlock()
	if !ok || first.sftpClient == nil {
		t.Fatal("expected pooled client after List")
	}
	firstClient := first.sftpClient
	acceptsBefore := srv.accepts.Load()

	srv.Restart()

	entries, err = b.List(context.Background(), srv.loc)
	if err != nil {
		t.Fatalf("List after server restart: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("List after restart returned no entries")
	}
	p.mu.Lock()
	second, ok := p.conns[host]
	p.mu.Unlock()
	if !ok || second.sftpClient == nil {
		t.Fatal("expected a pooled client after retry")
	}
	if second.sftpClient == firstClient {
		t.Fatal("dead pooled client was reused after transport failure")
	}
	if srv.accepts.Load() <= acceptsBefore {
		t.Fatal("List after restart should dial a new connection")
	}
}

func TestPoolKeepsHealthySession(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bravo.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{workDir: root})
	p := srv.newPool(time.Hour)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	if _, err := b.List(context.Background(), srv.loc); err != nil {
		t.Fatalf("first List: %v", err)
	}
	host, err := pathloc.SFTPHostPart(srv.loc)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	first := p.conns[host].sftpClient
	p.mu.Unlock()
	accepts := srv.accepts.Load()

	if _, err := b.List(context.Background(), srv.loc); err != nil {
		t.Fatalf("second List: %v", err)
	}
	p.mu.Lock()
	second := p.conns[host].sftpClient
	p.mu.Unlock()
	if first != second {
		t.Fatal("healthy session should stay pooled across Lists")
	}
	if srv.accepts.Load() != accepts {
		t.Fatal("reuse must not dial a new connection")
	}
}

func TestPoolDoesNotRetryMutationAfterTransportError(t *testing.T) {
	root := t.TempDir()
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{workDir: root})
	p := srv.newPool(time.Hour)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	if err := p.Touch(context.Background(), srv.loc); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	host, err := pathloc.SFTPHostPart(srv.loc)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	firstClient := p.conns[host].sftpClient
	p.mu.Unlock()
	acceptsBefore := srv.accepts.Load()

	srv.Restart()

	mkdirLoc, err := srv.loc.Join("newdir")
	if err != nil {
		t.Fatal(err)
	}
	err = b.Mkdir(context.Background(), mkdirLoc, 0o755)
	if err == nil {
		t.Fatal("Mkdir on a dead session should fail without a replay retry")
	}
	if srv.accepts.Load() != acceptsBefore {
		t.Fatal("mutation must not redial after a transport error")
	}
	p.mu.Lock()
	_, stillPooled := p.conns[host]
	p.mu.Unlock()
	if stillPooled {
		t.Fatal("dead connection should be evicted even when the mutation is not retried")
	}

	if _, err := b.List(context.Background(), srv.loc); err != nil {
		t.Fatalf("List after mutation eviction: %v", err)
	}
	p.mu.Lock()
	next := p.conns[host]
	p.mu.Unlock()
	if next == nil || next.sftpClient == firstClient {
		t.Fatal("read after eviction should dial a replacement session")
	}
}

func TestPoolDoesNotEvictOnMissingFile(t *testing.T) {
	root := t.TempDir()
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{workDir: root})
	p := srv.newPool(time.Hour)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	if _, err := b.List(context.Background(), srv.loc); err != nil {
		t.Fatalf("List: %v", err)
	}
	host, err := pathloc.SFTPHostPart(srv.loc)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	client := p.conns[host].sftpClient
	p.mu.Unlock()

	missing, err := srv.loc.Join("missing-file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Stat(context.Background(), missing); err == nil {
		t.Fatal("expected stat error for missing file")
	}
	p.mu.Lock()
	still := p.conns[host]
	p.mu.Unlock()
	if still == nil || still.sftpClient != client {
		t.Fatal("protocol errors must not evict a healthy pooled session")
	}
}
