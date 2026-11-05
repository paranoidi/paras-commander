// Package fileindex is pc's built-in fileindex feature (ADR 0024): the lazily opened index
// database, the fileindex:// filesystem backend, the tags column provider, the scan and prune
// task jobs and tag edits. Every function here may do disk or database I/O and must run off the
// UI goroutine; internal/apphandler/fileindex owns the UI side.
package fileindex

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ficonfig "github.com/paranoidi/fileindex/config"
	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/fileindex/scan"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fschange"
)

// ErrNoIndex is wrapped by DB when no index database exists at the configured path.
var ErrNoIndex = errors.New("no fileindex database")

// ErrClosed is returned by DB after Close.
var ErrClosed = errors.New("fileindex database closed")

// ErrNewerSchema replaces db.ErrNewerSchema, which tells users of the fileindex command to
// upgrade it: pc links the fileindex library, so only upgrading pc itself helps.
var ErrNewerSchema = errors.New("database schema is newer than this paras-commander supports, upgrade paras-commander")

// pcSchemaError returns err with a db.ErrNewerSchema reworded for pc.
func pcSchemaError(err error) error {
	if errors.Is(err, db.ErrNewerSchema) {
		return ErrNewerSchema
	}
	return err
}

// Index is the shared fileindex database handle. DB opens it once, lazily, and checks its
// schema. pc creates (Create) or upgrades (Upgrade) a database only when the user confirms.
type Index struct {
	cfg config.FileIndexConfig
	// configPath is fileindex's config.ini (ficonfig.DefaultPath() unless a test sets it).
	configPath string
	home       string

	mu     sync.Mutex
	db     *db.DB
	failed *openFailure // the last open failure, returned again while the file is unchanged
	closed bool         // set by Close

	// OnOpenError, when set, is called once, from the goroutine that hit it, with the first
	// open failure, so the UI can report it once instead of on every action.
	OnOpenError func(error)
	reported    sync.Once
	// OnChange, when set, is called (from the goroutine that made it) after a change to what the
	// index holds: a scan or prune task ends, an entry is moved or removed, or the database is
	// created or upgraded.
	OnChange func()

	// Apply's queue (follow.go): batches waiting for the one goroutine that applies them.
	applyMu      sync.Mutex
	applyPending [][]fschange.Change
	applyRunning bool
	applyDone    chan struct{} // closed when the running drain ends

	// writeMu serializes pc's own index writes: Apply's drain and the backend's Rename and Remove.
	writeMu sync.Mutex
}

// New returns an Index for pc's [fileindex] settings. Nothing is read until DB is called.
func New(cfg config.FileIndexConfig) *Index {
	home, _ := os.UserHomeDir()
	return &Index{cfg: cfg, configPath: ficonfig.DefaultPath(), home: home}
}

// readConfig loads fileindex's config.ini without writing it (defaults when it is missing).
func (x *Index) readConfig() (*ficonfig.Config, error) {
	return ficonfig.Read(x.configPath)
}

// DatabasePath is the index database pc opens: [fileindex].database, else config.ini's.
func (x *Index) DatabasePath() (string, error) {
	if p := strings.TrimSpace(x.cfg.Database); p != "" {
		if p == "~" || strings.HasPrefix(p, "~/") {
			p = filepath.Join(x.home, p[1:])
		}
		return p, nil
	}
	c, err := x.readConfig()
	if err != nil {
		return "", err
	}
	return c.DatabasePath(), nil
}

// DB returns the open database, opening it on first use. A missing database wraps ErrNoIndex;
// one on an older schema fails with fileindex's own explanation, a newer one with ErrNewerSchema. A failure is returned
// again without reopening until the database file appears or changes (an index created or
// upgraded outside pc), or Create or Upgrade replaces it.
func (x *Index) DB() (*db.DB, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	d, err := x.dbLocked()
	if err != nil && !errors.Is(err, ErrClosed) && x.OnOpenError != nil {
		x.reported.Do(func() { x.OnOpenError(err) })
	}
	return d, err
}

// Available reports whether the database opens, like DB but without reporting a failure to
// OnOpenError (so asking does not prompt to create or upgrade the index).
func (x *Index) Available() bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	_, err := x.dbLocked()
	return err == nil
}

// dbLocked is DB with x.mu held and without OnOpenError.
func (x *Index) dbLocked() (*db.DB, error) {
	if x.closed {
		return nil, ErrClosed
	}
	if x.db != nil {
		return x.db, nil
	}
	if f := x.failed; f != nil && statDBFile(f.path) == f.file {
		return nil, f.err
	}
	path, d, err := x.open()
	if err != nil {
		// ponytail: the file is stat'ed after the failed open (opening can touch it), so a change
		// made during that open goes unseen until the file changes again. No path (config.ini
		// unreadable) has no file to watch, so that failure is not kept.
		if path != "" {
			x.failed = &openFailure{err: err, path: path, file: statDBFile(path)}
		}
		return nil, err
	}
	x.db, x.failed = d, nil
	return d, nil
}

// openFailure is a failed open and the database file as it was then.
type openFailure struct {
	err  error
	path string
	file dbFile
}

// dbFile is what DB compares to tell whether the database file changed since a failed open.
type dbFile struct {
	exists  bool
	size    int64
	modTime int64
}

func statDBFile(path string) dbFile {
	fi, err := os.Stat(path)
	if err != nil {
		return dbFile{}
	}
	return dbFile{exists: true, size: fi.Size(), modTime: fi.ModTime().UnixNano()}
}

// open opens and schema-checks the database at DatabasePath and returns that path.
func (x *Index) open() (string, *db.DB, error) {
	path, err := x.DatabasePath()
	if err != nil {
		return "", nil, err
	}
	// db.Open creates the parent directory and SQLite the file: check first so a missing
	// index stays missing.
	if _, err := os.Stat(path); err != nil {
		return path, nil, fmt.Errorf("%w at %s (run `fileindex init`)", ErrNoIndex, path)
	}
	d, err := db.Open(path)
	if err != nil {
		return path, nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := d.RequireSchema(); err != nil {
		_ = d.Close()
		return path, nil, fmt.Errorf("%s: %w", path, pcSchemaError(err))
	}
	return path, d, nil
}

// ErrIndexExists is returned by Create when a database is already at the path.
var ErrIndexExists = errors.New("fileindex database already exists")

// Create makes an empty, schema-current database at DatabasePath (fileindex's `init` on a new
// file) and keeps it open for DB. It refuses an existing file, so it never migrates one.
func (x *Index) Create() (string, error) { return x.initDB(false) }

// NeedsUpgrade reports whether a DB error is a database that exists but Upgrade must bring to
// the current schema: an older schema, or a file that was never initialised. A newer
// schema is not: only a newer pc can open it (ErrNewerSchema).
func NeedsUpgrade(err error) bool {
	return errors.Is(err, db.ErrOutdatedSchema) || errors.Is(err, db.ErrNotInitialised)
}

// Upgrade brings the existing database to the current schema (migrating an older one, which can
// rebuild its search indexes and take minutes on a large index) and keeps it open.
func (x *Index) Upgrade() (string, error) { return x.initDB(true) }

// initDB creates or upgrades the database at DatabasePath and keeps it open: upgrade requires
// the file to exist and initialises one that never was, otherwise the file must not exist.
func (x *Index) initDB(upgrade bool) (string, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.closed {
		return "", ErrClosed
	}
	path, err := x.DatabasePath()
	if err != nil {
		return "", err
	}
	verb := "create"
	_, statErr := os.Stat(path)
	if upgrade {
		verb = "upgrade"
		if statErr != nil {
			return path, fmt.Errorf("%w at %s", ErrNoIndex, path)
		}
	} else if statErr == nil {
		return path, fmt.Errorf("%w at %s", ErrIndexExists, path)
	}
	d, err := db.Open(path)
	if err != nil {
		return path, fmt.Errorf("%s: %w", path, err)
	}
	if upgrade {
		err = d.Upgrade(io.Discard)
	}
	if !upgrade || errors.Is(err, db.ErrNotInitialised) {
		err = d.Init()
	}
	if err != nil {
		_ = d.Close()
		return path, fmt.Errorf("%s %s: %w", verb, path, pcSchemaError(err))
	}
	if x.db != nil {
		_ = x.db.Close()
	}
	x.db, x.failed = d, nil
	defer x.changed()
	return path, nil
}

// Close closes the database if it was opened. The index stays closed: later calls fail with
// ErrClosed instead of reopening the file.
func (x *Index) Close() error {
	err := x.closeDB()
	x.WaitApplied() // a drain in flight sees the closed index and ends
	return err
}

func (x *Index) closeDB() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.closed = true
	if x.db == nil {
		return nil
	}
	err := x.db.Close()
	x.db = nil
	return err
}

// scanSettings returns the include/exclude rules from config.ini and the hash worker count:
// [fileindex].workers, else config.ini's parallel_workers (0 = auto).
func (x *Index) scanSettings() (scan.Rules, int, error) {
	c, err := x.readConfig()
	if err != nil {
		return scan.Rules{}, 0, err
	}
	rules, err := scan.RulesFromConfig(c)
	if err != nil {
		return scan.Rules{}, 0, err
	}
	if x.cfg.Workers > 0 {
		return rules, x.cfg.Workers, nil
	}
	workers, err := c.ParallelWorkers()
	return rules, workers, err
}
