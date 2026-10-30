package foldercolor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
)

// ponytail: only the "home" metadata database is read, so folders outside $HOME (the "root" DB,
// "label-*" removable volumes) are not coloured. Add those databases when needed.
const metaDBName = "home"

type snapshot struct {
	colors  map[string]tcell.Color // absolute dir path -> colour
	bases   map[string]string      // absolute dir path -> current icon basename ("folder-documents")
	choices []Choice               // colours of the current icon-theme style
}

// Store is an in-memory copy of the folder colours. Reload and Changed do file I/O and belong on a
// background goroutine; Lookup only reads the last published snapshot and is safe on the UI thread.
type Store struct {
	home, dir string
	snap      atomic.Pointer[snapshot]

	mu  sync.Mutex // serializes Reload and guards tag/sig
	tag uint32
	sig string
}

// New returns a Store reading the current user's GVfs metadata. Nothing is loaded until Reload.
func New() *Store {
	home, _ := os.UserHomeDir()
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	return &Store{home: home, dir: filepath.Join(data, "gvfs-metadata")}
}

// Lookup returns the folder colour of an absolute local directory path.
func (s *Store) Lookup(p string) (tcell.Color, bool) {
	if sn := s.snap.Load(); sn != nil {
		c, ok := sn.colors[p]
		return c, ok
	}
	return tcell.ColorDefault, false
}

// Choices lists the selectable colours of the current icon-theme style (empty without a palette).
func (s *Store) Choices() []Choice {
	if sn := s.snap.Load(); sn != nil {
		return sn.choices
	}
	return nil
}

func (s *Store) treePath() string { return filepath.Join(s.dir, metaDBName) }

func (s *Store) journalPath(tag uint32) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s-%08x.log", metaDBName, tag))
}

// signature identifies the on-disk state cheaply: tree and journal size/mtime plus the journal's
// entry count (the journal is a fixed-size mmap, so size alone never changes).
func (s *Store) signature(tag uint32) string {
	var b strings.Builder
	for _, p := range []string{s.treePath(), s.journalPath(tag)} {
		if fi, err := os.Stat(p); err == nil {
			fmt.Fprintf(&b, "%d:%d;", fi.Size(), fi.ModTime().UnixNano())
		}
	}
	if f, err := os.Open(s.journalPath(tag)); err == nil {
		hdr := make([]byte, journalHeaderLen)
		if n, _ := f.Read(hdr); n == journalHeaderLen {
			c, _ := journalEntryCount(hdr)
			fmt.Fprintf(&b, "%d", c)
		}
		_ = f.Close()
	}
	return b.String()
}

// Changed reports whether the metadata files differ from what the last Reload read.
func (s *Store) Changed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signature(s.tag) != s.sig
}

// Reload re-reads the tree, journal and palette and publishes a new snapshot.
func (s *Store) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Signature first: a write during the read below then shows up as Changed on the next tick.
	sig := s.signature(s.tag)
	pal := loadPalette()
	refs := map[string]iconRef{}
	var tag uint32
	if data, err := os.ReadFile(s.treePath()); err == nil {
		if m, t, err := readTree(data); err == nil {
			refs, tag = m, t
		}
	}
	if data, err := os.ReadFile(s.journalPath(tag)); err == nil {
		replayJournal(data, tag, refs)
	}
	sn := &snapshot{colors: map[string]tcell.Color{}, bases: map[string]string{}}
	for rel, ref := range refs {
		abs := filepath.Join(s.home, rel)
		if c, ok := pal.color(ref); ok {
			sn.colors[abs] = c
		}
		if ref.icon != "" {
			sn.bases[abs] = strings.TrimSuffix(path.Base(ref.icon), path.Ext(ref.icon))
		}
	}
	sn.choices = pal.styles[pal.styleFor(currentIconTheme())]
	if tag != s.tag {
		sig = s.signature(tag) // the pre-read signature named the previous generation's journal
	}
	s.snap.Store(sn)
	s.tag, s.sig = tag, sig
}

// currentIconTheme returns the desktop's icon theme (e.g. "Mint-Y-Sand"), or "" when unknown.
func currentIconTheme() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gsettings", "get", "org.cinnamon.desktop.interface", "icon-theme").Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'")
}
