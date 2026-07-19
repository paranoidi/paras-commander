// Package file implements the host filesystem backend via internal/localfs.
package file

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func init() {
	if err := fsbackend.RegisterDefault(New()); err != nil {
		panic(err)
	}
}

// Backend serves pathloc file scheme paths.
type Backend struct{}

// New returns a host file backend.
func New() *Backend {
	return &Backend{}
}

// Scheme implements fsbackend.Backend.
func (b *Backend) Scheme() pathloc.Scheme {
	return pathloc.SchemeFile
}

// List implements fsbackend.Backend.
func (b *Backend) List(ctx context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	_ = ctx
	host, err := dir.FilePath()
	if err != nil {
		return nil, err
	}
	listing, err := localfs.ListDir(host, localfs.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]fsbackend.Entry, len(listing.Entries))
	for i, e := range listing.Entries {
		loc, err := pathloc.File(e.Path)
		if err != nil {
			return nil, err
		}
		out[i] = localEntryToBackend(loc, e)
	}
	return out, nil
}

// ListWithOptions lists using panel listing options (hidden, gitignore).
func (b *Backend) ListWithOptions(ctx context.Context, dir pathloc.Path, opts localfs.ListOptions) ([]fsbackend.Entry, error) {
	_ = ctx
	host, err := dir.FilePath()
	if err != nil {
		return nil, err
	}
	listing, err := localfs.ListDir(host, opts)
	if err != nil {
		return nil, err
	}
	out := make([]fsbackend.Entry, len(listing.Entries))
	for i, e := range listing.Entries {
		loc, err := pathloc.File(e.Path)
		if err != nil {
			return nil, err
		}
		out[i] = localEntryToBackend(loc, e)
	}
	return out, nil
}

// Stat implements fsbackend.Backend.
func (b *Backend) Stat(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	_ = ctx
	host, err := loc.FilePath()
	if err != nil {
		return fsbackend.Entry{}, err
	}
	e, err := localfs.EntryFromPath(host)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	p, err := pathloc.File(e.Path)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	return localEntryToBackend(p, e), nil
}

// OpenRead implements fsbackend.Backend.
func (b *Backend) OpenRead(ctx context.Context, loc pathloc.Path) (io.ReadCloser, error) {
	_ = ctx
	host, err := loc.FilePath()
	if err != nil {
		return nil, err
	}
	return os.Open(host)
}

// OpenWrite implements fsbackend.Backend.
func (b *Backend) OpenWrite(ctx context.Context, loc pathloc.Path, size int64, opts fsbackend.CreateOpts) (io.WriteCloser, error) {
	_ = ctx
	host, err := loc.FilePath()
	if err != nil {
		return nil, err
	}
	flags := os.O_WRONLY | os.O_CREATE
	if opts.Truncate {
		flags |= os.O_TRUNC
	}
	if opts.Append {
		flags |= os.O_APPEND
	}
	f, err := os.OpenFile(host, flags, 0o644)
	if err != nil {
		return nil, err
	}
	if opts.Truncate && size > 0 {
		_ = f.Truncate(size)
	}
	return f, nil
}

// Mkdir implements fsbackend.Backend.
func (b *Backend) Mkdir(ctx context.Context, dir pathloc.Path, perm fs.FileMode) error {
	_ = ctx
	host, err := dir.FilePath()
	if err != nil {
		return err
	}
	return localfs.Mkdir(host, perm)
}

// Remove implements fsbackend.Backend.
func (b *Backend) Remove(ctx context.Context, loc pathloc.Path) error {
	_ = ctx
	host, err := loc.FilePath()
	if err != nil {
		return err
	}
	return localfs.Remove(host)
}

// Rename implements fsbackend.Backend.
func (b *Backend) Rename(ctx context.Context, oldLoc, newLoc pathloc.Path) error {
	_ = ctx
	oldHost, err := oldLoc.FilePath()
	if err != nil {
		return err
	}
	newHost, err := newLoc.FilePath()
	if err != nil {
		return err
	}
	return localfs.Rename(oldHost, newHost)
}

// ReadSymlink implements fsbackend.Backend.
func (b *Backend) ReadSymlink(ctx context.Context, loc pathloc.Path) (string, error) {
	_ = ctx
	host, err := loc.FilePath()
	if err != nil {
		return "", err
	}
	return localfs.ReadSymlink(host)
}

// Symlink implements fsbackend.Backend.
func (b *Backend) Symlink(ctx context.Context, loc pathloc.Path, target string) error {
	_ = ctx
	host, err := loc.FilePath()
	if err != nil {
		return err
	}
	return localfs.MakeSymlink(target, host)
}

func localEntryToBackend(loc pathloc.Path, e localfs.Entry) fsbackend.Entry {
	return fsbackend.Entry{
		Name:       e.Name,
		Loc:        loc,
		Type:       entryTypeFromLocal(e.Type),
		Size:       e.Size,
		Mode:       e.Mode,
		ModifiedAt: e.ModifiedAt,
		Dev:        e.Dev,
		DevValid:   e.DevValid,
	}
}

func entryTypeFromLocal(t localfs.EntryType) fsbackend.EntryType {
	return fsbackend.BackendTypeFromLocal(t)
}

// ToLocalEntries converts backend entries to localfs rows with host file paths
// (file-scheme only). Prefer fsbackend.ToPanelEntries when the canonical Loc string is enough.
func ToLocalEntries(entries []fsbackend.Entry) ([]localfs.Entry, error) {
	out := make([]localfs.Entry, len(entries))
	for i, e := range entries {
		host, err := e.Loc.FilePath()
		if err != nil {
			return nil, fmt.Errorf("entry %q: %w", e.Name, err)
		}
		row := fsbackend.ToPanelEntry(e)
		row.Path = host
		out[i] = row
	}
	return out, nil
}
