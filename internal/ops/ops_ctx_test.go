package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestRenameFastPathHonorsCallerContext(t *testing.T) {
	be := newBlockingMoveBackend("rename")
	installOpsBackend(t, be)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := pathloc.MustParse(remoteMoveSrcRoot + "/cedar.txt")
	dest := pathloc.MustParse(remoteMoveDestRoot + "/cedar.txt")

	done := make(chan error, 1)
	go func() {
		_, err := RenameFastPath(ctx, src, dest)
		done <- err
	}()

	select {
	case <-be.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("RenameFastPath never entered blocked remote rename")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RenameFastPath did not return after cancel")
	}
}
