package foldercolor

import (
	"context"
	"fmt"
	"maps"
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

// GVfs keeps one metadata database per tree: "home" ($HOME, paths relative to it), "uuid-*" /
// "label-*" (a mounted block device, paths relative to its mount point) and "root" (everything
// else, absolute paths). The network and device backends (mtp:, google-drive:, network:, admin:)
// are intentionally not read: their folders are not local paths.
const (
	homeTree = "home"
	rootTree = "root"
)

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

	mu   sync.Mutex        // serializes Reload and guards tags/sig
	tags map[string]uint32 // tree name -> random_tag of the loaded tree (names its journal)
	sig  string
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

func (s *Store) treePath(tree string) string { return filepath.Join(s.dir, tree) }

func (s *Store) journalPath(tree string, tag uint32) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s-%08x.log", tree, tag))
}

// signature identifies the on-disk state cheaply: for each tree, tree and journal size/mtime plus
// the journal's entry count (the journal is a fixed-size mmap, so size alone never changes), and
// the mounted device trees, so plugging or unplugging a drive counts as a change.
func (s *Store) signature(mounts []devMount, tags map[string]uint32) string {
	var b strings.Builder
	trees := []string{rootTree, homeTree}
	for _, m := range mounts {
		trees = append(trees, m.tree)
		fmt.Fprintf(&b, "%s@%s:%s;", m.tree, m.point, m.root)
	}
	for _, t := range trees {
		jp := s.journalPath(t, tags[t])
		for _, p := range []string{s.treePath(t), jp} {
			if fi, err := os.Stat(p); err == nil {
				fmt.Fprintf(&b, "%d:%d;", fi.Size(), fi.ModTime().UnixNano())
			}
		}
		if f, err := os.Open(jp); err == nil {
			hdr := make([]byte, journalHeaderLen)
			if n, _ := f.Read(hdr); n == journalHeaderLen {
				c, _ := journalEntryCount(hdr)
				fmt.Fprintf(&b, "%d;", c)
			}
			_ = f.Close()
		}
	}
	return b.String()
}

// Changed reports whether the metadata files or the mounted drives differ from what the last
// Reload read.
func (s *Store) Changed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signature(deviceMounts(), s.tags) != s.sig
}

// readTreeDB loads one tree with its journal replayed; a missing or malformed file yields nothing.
func (s *Store) readTreeDB(tree string) (map[string]iconRef, uint32) {
	refs := map[string]iconRef{}
	var tag uint32
	if data, err := os.ReadFile(s.treePath(tree)); err == nil {
		if m, t, err := readTree(data); err == nil {
			refs, tag = m, t
		}
	}
	if data, err := os.ReadFile(s.journalPath(tree, tag)); err == nil {
		replayJournal(data, tag, refs)
	}
	return refs, tag
}

// Reload re-reads the trees, journals and palette and publishes a new snapshot. Entries are
// applied root first, then mounted devices, then home, so the more specific tree wins.
func (s *Store) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	mounts := deviceMounts()
	// Signature first: a write during the read below then shows up as Changed on the next tick.
	sig := s.signature(mounts, s.tags)
	pal := loadPalette()
	sn := &snapshot{colors: map[string]tcell.Color{}, bases: map[string]string{}}
	tags := map[string]uint32{}
	add := func(abs string, ref iconRef) {
		delete(sn.colors, abs)
		delete(sn.bases, abs)
		if c, ok := pal.color(ref); ok {
			sn.colors[abs] = c
		}
		if ref.icon != "" {
			sn.bases[abs] = strings.TrimSuffix(path.Base(ref.icon), path.Ext(ref.icon))
		}
	}
	refs, tag := s.readTreeDB(rootTree)
	tags[rootTree] = tag
	for rel, ref := range refs {
		add(filepath.Join("/", rel), ref)
	}
	loaded := map[string]map[string]iconRef{} // a filesystem mounted twice is read once
	for _, m := range mounts {
		if _, ok := loaded[m.tree]; !ok {
			loaded[m.tree], tags[m.tree] = s.readTreeDB(m.tree)
		}
		for rel, ref := range loaded[m.tree] {
			if abs, ok := m.abs("/" + rel); ok {
				add(abs, ref)
			}
		}
	}
	refs, tag = s.readTreeDB(homeTree)
	tags[homeTree] = tag
	for rel, ref := range refs {
		add(filepath.Join(s.home, rel), ref)
	}
	sn.choices = pal.styles[pal.styleFor(currentIconTheme())]
	if !maps.Equal(tags, s.tags) {
		sig = s.signature(mounts, tags) // the pre-read signature named the previous generation's journals
	}
	s.snap.Store(sn)
	s.tags, s.sig = tags, sig
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
