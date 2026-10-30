// Package fsbackend provides a registry of filesystem backends keyed by pathloc scheme.
package fsbackend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// EntryType classifies a directory listing row.
type EntryType int

const (
	EntryFile EntryType = iota
	EntryDirectory
	EntrySymlink
	EntryOther
)

// Entry is a normalized directory row independent of UI packages.
type Entry struct {
	Name       string
	Loc        pathloc.Path
	Type       EntryType
	Size       int64
	Mode       fs.FileMode
	ModifiedAt time.Time
	// Dev/DevValid carry st_dev captured at listing time (local backend only; see localfs.Entry).
	Dev      uint64
	DevValid bool
	// AccessDenied carries a permission fact captured at listing time (local backend only;
	// see localfs.Entry).
	AccessDenied bool
}

// CreateOpts controls OpenWrite behavior (phase 2+).
type CreateOpts struct {
	Truncate bool
	Append   bool
}

// Backend lists and transfers files for one pathloc scheme.
type Backend interface {
	Scheme() pathloc.Scheme
	List(ctx context.Context, dir pathloc.Path) ([]Entry, error)
	Stat(ctx context.Context, loc pathloc.Path) (Entry, error)
	// OpenRead returns a seekable reader so offset reads (find-duplicates chunk
	// hashing) skip ahead instead of re-reading the prefix.
	OpenRead(ctx context.Context, loc pathloc.Path) (io.ReadSeekCloser, error)
	OpenWrite(ctx context.Context, loc pathloc.Path, size int64, opts CreateOpts) (io.WriteCloser, error)
	Mkdir(ctx context.Context, dir pathloc.Path, perm fs.FileMode) error
	Remove(ctx context.Context, loc pathloc.Path) error
	Rename(ctx context.Context, oldLoc, newLoc pathloc.Path) error
	ReadSymlink(ctx context.Context, loc pathloc.Path) (string, error)
	Symlink(ctx context.Context, loc pathloc.Path, target string) error
}

// TreeRemover is implemented by a backend that removes a directory and everything under it in
// one call (the fileindex backend's index-only remove); recursive deletes use it instead of
// listing and removing child by child.
type TreeRemover interface {
	RemoveTree(ctx context.Context, dir pathloc.Path) error
}

// IndexOnly is implemented by a backend whose entries record files kept elsewhere: it accepts no
// writes, and Rename and Remove change only its records, never the files.
type IndexOnly interface{ IndexOnly() }

// RenameRooter is implemented by a backend that can Rename between two of its locations in one
// call when SameRenameRoot reports true (same SFTP host, same fileindex database).
type RenameRooter interface {
	SameRenameRoot(a, b pathloc.Path) bool
}

// ErrReadOnly is returned by a backend that cannot write: mkdir, symlink and writes on a
// read-only scheme.
var ErrReadOnly = errors.New("read-only location")

// Registry maps schemes to backends.
type Registry struct {
	backends map[pathloc.Scheme]Backend
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{backends: make(map[pathloc.Scheme]Backend)}
}

// Register adds a backend for its Scheme().
func (r *Registry) Register(b Backend) error {
	if b == nil {
		return errors.New("fsbackend: nil backend")
	}
	s := b.Scheme()
	if s == "" {
		return errors.New("fsbackend: empty scheme")
	}
	if _, dup := r.backends[s]; dup {
		return fmt.Errorf("fsbackend: scheme %q already registered", s)
	}
	r.backends[s] = b
	return nil
}

// Backend returns the backend for loc's scheme.
func (r *Registry) Backend(loc pathloc.Path) (Backend, error) {
	if loc.IsZero() {
		return nil, errors.New("fsbackend: zero path")
	}
	b, ok := r.backends[loc.Scheme()]
	if !ok {
		return nil, fmt.Errorf("fsbackend: no backend for scheme %q", loc.Scheme())
	}
	return b, nil
}

// IsIndexOnly reports whether loc's backend implements IndexOnly.
func (r *Registry) IsIndexOnly(loc pathloc.Path) bool {
	b, err := r.Backend(loc)
	if err != nil {
		return false
	}
	_, ok := b.(IndexOnly)
	return ok
}

// List resolves the backend and lists dir.
func (r *Registry) List(ctx context.Context, dir pathloc.Path) ([]Entry, error) {
	b, err := r.Backend(dir)
	if err != nil {
		return nil, err
	}
	return b.List(ctx, dir)
}

// Stat resolves the backend and stats loc.
func (r *Registry) Stat(ctx context.Context, loc pathloc.Path) (Entry, error) {
	b, err := r.Backend(loc)
	if err != nil {
		return Entry{}, err
	}
	return b.Stat(ctx, loc)
}

var defaultRegistry = NewRegistry()

// Default returns the process-wide registry (file backend registered from init).
func Default() *Registry {
	return defaultRegistry
}

// RegisterDefault registers b on the process-wide registry.
func RegisterDefault(b Backend) error {
	return defaultRegistry.Register(b)
}
