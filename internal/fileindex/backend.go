package fileindex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ErrNotOnDisk is returned by OpenRead for an indexed file that is gone from disk.
var ErrNotOnDisk = errors.New("not on disk")

// Backend serves fileindex:// from the index: listings and stats come from the database,
// reads from the real file, renames and removes change the index only. It also implements
// fsbackend.TreeRemover.
type Backend struct {
	idx func() *Index
}

var (
	current      atomic.Pointer[Index]
	registerOnce sync.Once
	registerErr  error
)

// Register makes idx the index behind fileindex:// and registers the backend on
// fsbackend.Default() the first time. Later calls (a second App in tests) only swap the index.
func Register(idx *Index) error {
	current.Store(idx)
	registerOnce.Do(func() {
		registerErr = fsbackend.RegisterDefault(&Backend{idx: current.Load})
	})
	return registerErr
}

// NewBackend returns a backend over idx without registering it (tests).
func NewBackend(idx *Index) *Backend {
	return &Backend{idx: func() *Index { return idx }}
}

func (b *Backend) Scheme() pathloc.Scheme { return pathloc.SchemeFileIndex }

// IndexOnly marks the backend as records of files kept elsewhere (fsbackend.IndexOnly).
func (b *Backend) IndexOnly() {}

// SameRenameRoot is always true: every fileindex:// path lives in one database.
func (b *Backend) SameRenameRoot(_, _ pathloc.Path) bool { return true }

func (b *Backend) open() (*db.DB, error) {
	idx := b.idx()
	if idx == nil {
		return nil, fmt.Errorf("%w configured", ErrNoIndex)
	}
	return idx.DB()
}

// mtimeLayouts parse the index's naive local mtimes (db.FormatMtime): with and without the
// microsecond fraction.
var mtimeLayouts = []string{"2006-01-02 15:04:05.000000", "2006-01-02 15:04:05"}

func parseMtime(s string) time.Time {
	for _, l := range mtimeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func dirEntry(loc pathloc.Path, name string) fsbackend.Entry {
	return fsbackend.Entry{Name: name, Loc: loc, Type: fsbackend.EntryDirectory, Mode: fs.ModeDir | 0o755}
}

func fileEntry(loc pathloc.Path, f db.ListedFile) fsbackend.Entry {
	return fsbackend.Entry{Name: f.Name, Loc: loc, Type: fsbackend.EntryFile, Mode: 0o644, Size: f.Size, ModifiedAt: parseMtime(f.Mtime)}
}

// List lists an indexed directory: subdirs (no size or mtime) then files with their indexed
// size and mtime. The index root lists empty when nothing is indexed yet.
func (b *Backend) List(_ context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	host, err := pathloc.IndexHostPath(dir)
	if err != nil {
		return nil, err
	}
	d, err := b.open()
	if err != nil {
		return nil, err
	}
	id, found, err := db.DirByPath(d, host)
	if err != nil {
		return nil, err
	}
	if !found {
		if host == "/" {
			return nil, nil
		}
		return nil, &fs.PathError{Op: "list", Path: dir.String(), Err: fs.ErrNotExist}
	}
	files, subdirs, err := d.ListDir(*id)
	if err != nil {
		return nil, err
	}
	out := make([]fsbackend.Entry, 0, len(files)+len(subdirs))
	for _, s := range subdirs {
		loc, err := dir.Join(s.Name)
		if err != nil {
			continue // a name the index allows but a path element cannot hold
		}
		out = append(out, dirEntry(loc, s.Name))
	}
	for _, f := range files {
		loc, err := dir.Join(f.Name)
		if err != nil {
			continue
		}
		out = append(out, fileEntry(loc, f))
	}
	return out, nil
}

// Stat stats an indexed directory or file; the index root always exists.
func (b *Backend) Stat(_ context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	host, err := pathloc.IndexHostPath(loc)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	d, err := b.open()
	if err != nil {
		return fsbackend.Entry{}, err
	}
	if _, isDir, err := db.DirByPath(d, host); err != nil {
		return fsbackend.Entry{}, err
	} else if isDir || host == "/" {
		return dirEntry(loc, loc.Base()), nil
	}
	rec, found, err := d.FileInfo(host)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	if !found {
		return fsbackend.Entry{}, &fs.PathError{Op: "stat", Path: loc.String(), Err: fs.ErrNotExist}
	}
	return fileEntry(loc, db.ListedFile{Name: loc.Base(), Size: rec.Size, Mtime: rec.Mtime}), nil
}

// OpenRead opens the real file the index entry names, so copying out of the index goes through
// the normal transfer path. A file gone from disk fails with ErrNotOnDisk.
func (b *Backend) OpenRead(_ context.Context, loc pathloc.Path) (io.ReadSeekCloser, error) {
	host, err := pathloc.IndexHostPath(loc)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(host)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", host, ErrNotOnDisk)
	}
	return f, err
}

func (b *Backend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fsbackend.ErrReadOnly
}

func (b *Backend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error {
	return fsbackend.ErrReadOnly
}

func (b *Backend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fs.ErrInvalid // the index records no symlinks
}

func (b *Backend) Symlink(context.Context, pathloc.Path, string) error {
	return fsbackend.ErrReadOnly
}

// Rename records that the entry moved (fileindex's Move: an instance moves, a directory moves
// or merges into an indexed one). The disk is not touched.
func (b *Backend) Rename(_ context.Context, oldLoc, newLoc pathloc.Path) error {
	src, err := pathloc.IndexHostPath(oldLoc)
	if err != nil {
		return err
	}
	dst, err := pathloc.IndexHostPath(newLoc)
	if err != nil {
		return fmt.Errorf("rename out of the fileindex: %w", err)
	}
	d, err := b.open()
	if err != nil {
		return err
	}
	_, err = d.Move(src, dst, nil)
	return err
}

// Remove removes an indexed file's instance, or a directory with everything under it, from the
// index. The disk is not touched.
func (b *Backend) Remove(ctx context.Context, loc pathloc.Path) error {
	host, err := pathloc.IndexHostPath(loc)
	if err != nil {
		return err
	}
	d, err := b.open()
	if err != nil {
		return err
	}
	if _, isDir, err := db.DirByPath(d, host); err != nil {
		return err
	} else if isDir {
		return b.RemoveTree(ctx, loc)
	}
	_, err = d.RemoveInstance(host)
	return err
}

// RemoveTree removes an indexed directory and everything under it from the index.
func (b *Backend) RemoveTree(_ context.Context, dir pathloc.Path) error {
	host, err := pathloc.IndexHostPath(dir)
	if err != nil {
		return err
	}
	d, err := b.open()
	if err != nil {
		return err
	}
	_, err = d.RemoveTree(host)
	return err
}

// indexPath maps a panel location to the absolute path the index records: a fileindex:// path
// as named, a local path with symlinks resolved (the index stores real paths). SFTP has none.
func indexPath(loc pathloc.Path) (string, error) {
	switch loc.Scheme() {
	case pathloc.SchemeFileIndex:
		return pathloc.IndexHostPath(loc)
	case pathloc.SchemeFile:
		host, err := loc.FilePath()
		if err != nil {
			return "", err
		}
		return db.Resolve(host)
	}
	return "", fmt.Errorf("%s is not indexed: fileindex covers local paths only", loc)
}

// IndexPath is indexPath for a path string (panel paths, selection entries).
func IndexPath(raw string) (string, error) {
	loc, err := pathloc.Parse(raw)
	if err != nil {
		return "", err
	}
	return indexPath(loc)
}

// IndexLocation is the fileindex:// location of a local path or of a fileindex:// path itself.
func IndexLocation(raw string) (pathloc.Path, error) {
	host, err := IndexPath(raw)
	if err != nil {
		return pathloc.Path{}, err
	}
	return pathloc.FileIndex(filepath.Clean(host))
}
