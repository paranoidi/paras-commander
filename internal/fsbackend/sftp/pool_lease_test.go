package sftp

import (
	"context"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestIdleExpiryDoesNotCloseBlockedReadDir(t *testing.T) {
	listEntered := make(chan struct{})
	listGate := make(chan struct{})
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{
		listGate:    listGate,
		listEntered: listEntered,
	})
	idle := 30 * time.Millisecond
	p := srv.newPool(idle)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	if err := p.Touch(context.Background(), srv.loc); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	errc := make(chan error, 1)
	go func() {
		_, err := b.List(context.Background(), srv.loc)
		errc <- err
	}()

	select {
	case <-listEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("ReadDir never reached the server")
	}
	time.Sleep(5 * idle)
	close(listGate)

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("blocked ReadDir failed after idle window: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("List did not return after unblocking ReadDir")
	}

	host, err := pathloc.SFTPHostPart(srv.loc)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	_, ok := p.conns[host]
	p.mu.Unlock()
	if !ok {
		t.Fatal("pooled connection was closed during in-flight ReadDir")
	}
}

func TestIdleExpiryDoesNotCloseBlockedStreamOpen(t *testing.T) {
	openEntered := make(chan struct{})
	openGate := make(chan struct{})
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{
		openGate:    openGate,
		openEntered: openEntered,
	})
	idle := 30 * time.Millisecond
	p := srv.newPool(idle)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	if err := p.Touch(context.Background(), srv.loc); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	errc := make(chan error, 1)
	var rc closer
	go func() {
		f, err := b.OpenRead(context.Background(), srv.fileLoc("notes.txt"))
		if err != nil {
			errc <- err
			return
		}
		rc = f
		errc <- nil
	}()

	select {
	case <-openEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("Open never reached the server")
	}
	time.Sleep(5 * idle)
	close(openGate)

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("blocked stream-open failed after idle window: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OpenRead did not return after unblocking")
	}
	if rc == nil {
		t.Fatal("OpenRead returned no reader")
	}

	host, err := pathloc.SFTPHostPart(srv.loc)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	c, ok := p.conns[host]
	streams := 0
	if ok {
		streams = c.activeStreams
	}
	p.mu.Unlock()
	if !ok {
		t.Fatal("pooled connection was closed during in-flight stream-open")
	}
	if streams != 1 {
		t.Fatalf("activeStreams = %d, want 1 after successful open", streams)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

type closer interface {
	Close() error
}
