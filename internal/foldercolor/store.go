package foldercolor

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
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

	mu      sync.Mutex        // serializes Reload/Refresh and guards everything below
	tags    map[string]uint32 // tree name -> random_tag of the loaded tree (names its journal)
	sig     string
	mounts  mountCache
	palette paletteCache
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
		if fi, err := os.Stat(s.treePath(t)); err == nil {
			fmt.Fprintf(&b, "%d:%d;", fi.Size(), fi.ModTime().UnixNano())
		}
		// One open per journal: fstat and the header read share the descriptor.
		if f, err := os.Open(s.journalPath(t, tags[t])); err == nil {
			if fi, err := f.Stat(); err == nil {
				fmt.Fprintf(&b, "%d:%d;", fi.Size(), fi.ModTime().UnixNano())
			}
			hdr := make([]byte, journalHeaderLen)
			if n, _ := f.ReadAt(hdr, 0); n == journalHeaderLen {
				c, _ := journalEntryCount(hdr)
				fmt.Fprintf(&b, "%d;", c)
			}
			_ = f.Close()
		}
	}
	return b.String()
}

// Refresh reloads when the metadata files or the mounted drives differ from what the last load
// read, reusing the mount scan and signature of that check; it reports whether it reloaded.
func (s *Store) Refresh() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	mounts := s.mounts.deviceMounts()
	sig := s.signature(mounts, s.tags)
	if sig == s.sig {
		return false
	}
	s.load(mounts, sig)
	return true
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

// Reload re-reads the trees, journals and palette unconditionally and publishes a new snapshot.
func (s *Store) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	mounts := s.mounts.deviceMounts()
	// Signature first: a write during the read below then shows up on the next Refresh.
	s.load(mounts, s.signature(mounts, s.tags))
}

type treeDB struct {
	refs map[string]iconRef
	tag  uint32
}

// load reads every tree in parallel (and the icon theme alongside), then applies entries root
// first, then mounted devices, then home, so the more specific tree wins. Caller holds s.mu.
func (s *Store) load(mounts []devMount, sig string) {
	names := []string{rootTree, homeTree}
	for _, m := range mounts {
		if !slices.Contains(names, m.tree) { // a filesystem mounted twice is read once
			names = append(names, m.tree)
		}
	}
	dbs := make([]treeDB, len(names))
	var iconTheme string
	var wg sync.WaitGroup
	for i, n := range names {
		wg.Go(func() { dbs[i].refs, dbs[i].tag = s.readTreeDB(n) })
	}
	wg.Go(func() { iconTheme = currentIconTheme() })
	pal := s.palette.load()
	wg.Wait()
	loaded := map[string]treeDB{}
	tags := map[string]uint32{}
	for i, n := range names {
		loaded[n], tags[n] = dbs[i], dbs[i].tag
	}

	sn := &snapshot{colors: map[string]tcell.Color{}, bases: map[string]string{}}
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
	for rel, ref := range loaded[rootTree].refs {
		add(filepath.Join("/", rel), ref)
	}
	for _, m := range mounts {
		for rel, ref := range loaded[m.tree].refs {
			if abs, ok := m.abs("/" + rel); ok {
				add(abs, ref)
			}
		}
	}
	for rel, ref := range loaded[homeTree].refs {
		add(filepath.Join(s.home, rel), ref)
	}
	sn.choices = pal.styles[pal.styleFor(iconTheme)]
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
