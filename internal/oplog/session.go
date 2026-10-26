package oplog

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Session is this process's spill directory, <cache root>/pc/jobs/<pid>-<start unix>. The
// directory is created on first spill, so a session that never spills never touches the disk.
type Session struct {
	root string // <cache root>/pc/jobs
	name string

	once sync.Once
	err  error
}

// NewSession returns the session for this process under cacheRoot (os.UserCacheDir()).
func NewSession(cacheRoot string) *Session {
	return &Session{
		root: filepath.Join(cacheRoot, "pc", "jobs"),
		name: fmt.Sprintf("%d-%d", os.Getpid(), time.Now().Unix()),
	}
}

// Dir is the session directory path (it may not exist yet).
func (s *Session) Dir() string { return filepath.Join(s.root, s.name) }

func (s *Session) filePath(jobID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("oplog: no spill session")
	}
	s.once.Do(func() { s.err = os.MkdirAll(s.Dir(), 0o700) })
	if s.err != nil {
		return "", s.err
	}
	return filepath.Join(s.Dir(), sanitizeName(jobID)+".oplog"), nil
}

// sanitizeName keeps a job ID usable as a file name.
func sanitizeName(id string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == os.PathSeparator || r == 0 {
			return '_'
		}
		return r
	}, id)
}

// Remove deletes the session directory and everything in it.
func (s *Session) Remove() error {
	if s == nil {
		return nil
	}
	return os.RemoveAll(s.Dir())
}

// SweepStale removes the spill directories of sessions whose process is gone (a crash or a
// kill left them behind). It does disk IO: run it off the UI thread.
func (s *Session) SweepStale() {
	if s == nil {
		return
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == s.name {
			continue
		}
		pidStr, _, ok := strings.Cut(e.Name(), "-")
		pid, err := strconv.Atoi(pidStr)
		if !ok || err != nil || processAlive(pid) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(s.root, e.Name()))
	}
}
