package sftp

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHandshakeHonorsDialTimeoutWhenServerStallsAfterAccept(t *testing.T) {
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{stallHandshake: true})
	p := srv.newPool(time.Hour)
	p.settings.DialTimeout = 150 * time.Millisecond
	t.Cleanup(p.CloseAll)

	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		errc <- p.Touch(ctx, srv.loc)
	}()

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected handshake to fail while the server stalls after accept")
		}
		if !errors.Is(err, context.DeadlineExceeded) && !isTimeoutish(err) {
			t.Fatalf("Touch err = %v, want a handshake timeout or canceled context", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SSH handshake ignored dial/context deadline after TCP accept")
	}

	waitGoroutineCount(t, baseline)
}

func TestReadDirHonorsContextWhenServerStalls(t *testing.T) {
	listEntered := make(chan struct{})
	listGate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-listGate:
		default:
			close(listGate)
		}
	})
	srv := startLoopbackSFTP(t, loopbackSFTPOpts{
		listGate:    listGate,
		listEntered: listEntered,
	})
	p := srv.newPool(time.Hour)
	t.Cleanup(p.CloseAll)
	b := &Backend{pool: p}

	if err := p.Touch(context.Background(), srv.loc); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	errc := make(chan error, 1)
	go func() {
		_, err := b.List(ctx, srv.loc)
		errc <- err
	}()

	select {
	case <-listEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("ReadDir never reached the server")
	}

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected List to fail when ReadDir is stalled past the context deadline")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("List err = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadDir ignored the listing context deadline")
	}

	select {
	case <-listGate:
	default:
		close(listGate)
	}
	waitGoroutineCount(t, baseline)
}

func isTimeoutish(err error) bool {
	if err == nil {
		return false
	}
	var netTimeout interface{ Timeout() bool }
	if errors.As(err, &netTimeout) && netTimeout.Timeout() {
		return true
	}
	msg := err.Error()
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "timeout")
}

func waitGoroutineCount(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var n int
	for time.Now().Before(deadline) {
		n = runtime.NumGoroutine()
		if n <= baseline+5 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutine leak: have %d, baseline %d", n, baseline)
}
