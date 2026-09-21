package ops

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

const remoteDeleteRoot = "sftp://alice@example.com/thicket"

type remoteDeleteBackend struct {
	root     pathloc.Path
	children []pathloc.Path
	removed  []string
	onRemove func(name string)
}

func (b *remoteDeleteBackend) Scheme() pathloc.Scheme { return pathloc.SchemeSFTP }

func (b *remoteDeleteBackend) Stat(_ context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	if loc.Equal(b.root) {
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryDirectory}, nil
	}
	for _, child := range b.children {
		if loc.Equal(child) {
			return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryFile}, nil
		}
	}
	return fsbackend.Entry{}, os.ErrNotExist
}

func (b *remoteDeleteBackend) List(_ context.Context, loc pathloc.Path) ([]fsbackend.Entry, error) {
	if !loc.Equal(b.root) {
		return nil, nil
	}
	entries := make([]fsbackend.Entry, 0, len(b.children))
	for _, child := range b.children {
		entries = append(entries, fsbackend.Entry{
			Name: child.Base(),
			Loc:  child,
			Type: fsbackend.EntryFile,
		})
	}
	return entries, nil
}

func (b *remoteDeleteBackend) Remove(_ context.Context, loc pathloc.Path) error {
	b.removed = append(b.removed, loc.String())
	if b.onRemove != nil {
		b.onRemove(loc.Base())
	}
	return nil
}

func (b *remoteDeleteBackend) OpenRead(context.Context, pathloc.Path) (io.ReadCloser, error) {
	return nil, fs.ErrInvalid
}
func (b *remoteDeleteBackend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fs.ErrInvalid
}
func (b *remoteDeleteBackend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error {
	return fs.ErrInvalid
}
func (b *remoteDeleteBackend) Rename(context.Context, pathloc.Path, pathloc.Path) error {
	return fs.ErrInvalid
}
func (b *remoteDeleteBackend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fs.ErrInvalid
}
func (b *remoteDeleteBackend) Symlink(context.Context, pathloc.Path, string) error {
	return fs.ErrInvalid
}

func installOpsBackend(t *testing.T, be fsbackend.Backend) {
	t.Helper()
	prev := backendFor
	backendFor = func(loc pathloc.Path) (fsbackend.Backend, error) {
		if loc.IsRemote() {
			return be, nil
		}
		return prev(loc)
	}
	t.Cleanup(func() { backendFor = prev })
}

func TestRemovePathRecursiveCancelBetweenRemoteChildren(t *testing.T) {
	root := pathloc.MustParse(remoteDeleteRoot)
	meadow := pathloc.MustParse(remoteDeleteRoot + "/meadow.txt")
	harbor := pathloc.MustParse(remoteDeleteRoot + "/harbor.txt")
	lantern := pathloc.MustParse(remoteDeleteRoot + "/lantern.txt")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	be := &remoteDeleteBackend{
		root:     root,
		children: []pathloc.Path{meadow, harbor, lantern},
		onRemove: func(name string) {
			if name == "meadow.txt" {
				cancel()
			}
		},
	}
	installOpsBackend(t, be)

	err := removePathRecursive(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(be.removed) != 1 || be.removed[0] != meadow.String() {
		t.Fatalf("removed = %v, want only %q", be.removed, meadow)
	}
}
