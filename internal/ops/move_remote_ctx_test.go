package ops

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

const (
	remoteMoveSrcRoot  = "sftp://alice@example.com/src"
	remoteMoveDestRoot = "sftp://alice@example.com/dest"
)

// blockingMoveBackend is a fake SFTP backend whose Stat/Rename wait on ctx.Done
// so cancel can be shown to unwind the move rename phase.
type blockingMoveBackend struct {
	destDir pathloc.Path
	mode    string // "stat", "rename", "rollback"

	phase   atomic.Value // string
	entered chan struct{}
	renames atomic.Int32
}

func newBlockingMoveBackend(mode string) *blockingMoveBackend {
	b := &blockingMoveBackend{
		destDir: pathloc.MustParse(remoteMoveDestRoot),
		mode:    mode,
		entered: make(chan struct{}, 1),
	}
	b.phase.Store("")
	return b
}

func (b *blockingMoveBackend) Scheme() pathloc.Scheme { return pathloc.SchemeSFTP }

func (b *blockingMoveBackend) Phase() string {
	s, _ := b.phase.Load().(string)
	return s
}

func (b *blockingMoveBackend) block(ctx context.Context, phase string) error {
	b.phase.Store(phase)
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingMoveBackend) Stat(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	if b.mode == "stat" {
		return fsbackend.Entry{}, b.block(ctx, "stat")
	}
	if loc.Equal(b.destDir) {
		return fsbackend.Entry{
			Name: loc.Base(),
			Loc:  loc,
			Type: fsbackend.EntryDirectory,
		}, nil
	}
	return fsbackend.Entry{}, os.ErrNotExist
}

func (b *blockingMoveBackend) Rename(ctx context.Context, _, _ pathloc.Path) error {
	n := b.renames.Add(1)
	switch b.mode {
	case "rename":
		return b.block(ctx, "rename")
	case "rollback":
		switch n {
		case 1:
			return nil
		case 2:
			return errors.New("second source rename failed")
		default:
			return b.block(ctx, "rollback")
		}
	default:
		return b.block(ctx, "rename")
	}
}

func (b *blockingMoveBackend) List(context.Context, pathloc.Path) ([]fsbackend.Entry, error) {
	return nil, nil
}
func (b *blockingMoveBackend) OpenRead(context.Context, pathloc.Path) (io.ReadCloser, error) {
	return nil, fs.ErrInvalid
}
func (b *blockingMoveBackend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fs.ErrInvalid
}
func (b *blockingMoveBackend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error {
	return fs.ErrInvalid
}
func (b *blockingMoveBackend) Remove(context.Context, pathloc.Path) error { return fs.ErrInvalid }
func (b *blockingMoveBackend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fs.ErrInvalid
}
func (b *blockingMoveBackend) Symlink(context.Context, pathloc.Path, string) error {
	return fs.ErrInvalid
}

func installMoveBackend(t *testing.T, be fsbackend.Backend) {
	t.Helper()
	prev := lookupMoveBackend
	lookupMoveBackend = func(loc pathloc.Path) (fsbackend.Backend, error) {
		if loc.IsRemote() {
			return be, nil
		}
		return backendFor(loc)
	}
	t.Cleanup(func() { lookupMoveBackend = prev })
}

func waitBlockedThenCancel(t *testing.T, entered <-chan struct{}, cancel context.CancelFunc, done <-chan error, wantPhase string, be *blockingMoveBackend, requireCanceled bool) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking backend never entered the expected wait")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("rename phase err = nil, want error after cancel")
		}
		if requireCanceled && !errors.Is(err, context.Canceled) {
			t.Fatalf("rename phase err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rename phase did not exit after cancel")
	}
	if got := be.Phase(); got != wantPhase {
		t.Fatalf("blocked phase = %q, want %q", got, wantPhase)
	}
}

func startRemoteRenamePhase(ctx context.Context, sources []pathloc.Path, dest pathloc.Path) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, _, _, err := executeMoveRenamePhase(ctx, sources, dest, nil, true, ProgressEmitThrottle{}, nil, nil)
		done <- err
	}()
	return done
}

func TestRemoteMoveRenamePhaseCancelInterruptsBlockedRename(t *testing.T) {
	be := newBlockingMoveBackend("rename")
	installMoveBackend(t, be)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := pathloc.MustParse(remoteMoveSrcRoot + "/cedar.txt")
	dest := pathloc.MustParse(remoteMoveDestRoot)
	done := startRemoteRenamePhase(ctx, []pathloc.Path{src}, dest)
	waitBlockedThenCancel(t, be.entered, cancel, done, "rename", be, true)
}

func TestRemoteMoveRenamePhaseCancelInterruptsBlockedDestStat(t *testing.T) {
	be := newBlockingMoveBackend("stat")
	installMoveBackend(t, be)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := pathloc.MustParse(remoteMoveSrcRoot + "/cedar.txt")
	dest := pathloc.MustParse(remoteMoveDestRoot)
	done := startRemoteRenamePhase(ctx, []pathloc.Path{src}, dest)
	waitBlockedThenCancel(t, be.entered, cancel, done, "stat", be, true)
}

func TestRemoteMoveRollbackCancelInterruptsBlockedRename(t *testing.T) {
	be := newBlockingMoveBackend("rollback")
	installMoveBackend(t, be)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sources := []pathloc.Path{
		pathloc.MustParse(remoteMoveSrcRoot + "/cedar.txt"),
		pathloc.MustParse(remoteMoveSrcRoot + "/harbor.txt"),
	}
	dest := pathloc.MustParse(remoteMoveDestRoot)
	done := startRemoteRenamePhase(ctx, sources, dest)
	waitBlockedThenCancel(t, be.entered, cancel, done, "rollback", be, false)
}

func TestRemoteMoveRenamePhaseCancelExitsWorker(t *testing.T) {
	be := newBlockingMoveBackend("rename")
	installMoveBackend(t, be)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _ = ExecuteMove(
			ctx,
			[]pathloc.Path{pathloc.MustParse(remoteMoveSrcRoot + "/cedar.txt")},
			pathloc.MustParse(remoteMoveDestRoot),
			Options{},
			ProgressEmitThrottle{},
			nil,
			nil,
			nil,
		)
	}()

	select {
	case <-be.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteMove never reached blocked remote rename")
	}
	cancel()

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteMove worker did not exit after cancel")
	}
	if got := be.Phase(); got != "rename" {
		t.Fatalf("blocked phase = %q, want rename", got)
	}
}
