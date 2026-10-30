package foldercolor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// iconsDir is where icon themes live; a var so tests can point it at a fixture.
var iconsDir = "/usr/share/icons"

// Apply sets dirs to colour c, or clears their colour when c is nil, the way folder-color-switcher
// does: through `gio set` (the gvfsd-metadata daemon owns the database; never write it directly),
// then re-touching the directory with its own times so Nemo re-renders it. It blocks on child
// processes and belongs on a background goroutine. Call Reload afterwards.
func (s *Store) Apply(dirs []string, c *Choice) error {
	gio, err := exec.LookPath("gio")
	if err != nil {
		return errors.New("gio not found")
	}
	sn := s.snap.Load()
	uris := map[string]string{} // icon basename -> URI: iconURI's stats run once per basename
	cmds := make([][][]string, len(dirs))
	errs := make([]error, len(dirs))
	for i, dir := range dirs {
		if c == nil {
			// Both keys: Nautilus-style tools set custom-icon-name instead. Unsetting a key that is
			// not set is harmless, so the second error is ignored.
			cmds[i] = [][]string{{"set", "-t", "unset", dir, "metadata::custom-icon"}, {"set", "-t", "unset", dir, "metadata::custom-icon-name"}}
			continue
		}
		base := "folder"
		if sn != nil && sn.bases[dir] != "" {
			base = sn.bases[dir]
		}
		uri, ok := uris[base]
		if !ok {
			uri = iconURI(c.Theme, base)
			uris[base] = uri
		}
		if uri == "" {
			errs[i] = fmt.Errorf("no folder icon in theme %s", c.Theme)
			continue
		}
		cmds[i] = [][]string{{"set", dir, "metadata::custom-icon", uri}}
	}
	sem := make(chan struct{}, runtime.NumCPU()) // ponytail: a fork per dir, capped at NumCPU in flight
	var wg sync.WaitGroup
	for i, dir := range dirs {
		if cmds[i] == nil {
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			fi, err := os.Stat(dir)
			if err != nil {
				errs[i] = err
				return
			}
			for j, args := range cmds[i] {
				if err := exec.Command(gio, args...).Run(); err != nil && j == 0 {
					errs[i] = fmt.Errorf("%s: %w", dir, err)
					return
				}
			}
			// ponytail: atime is set to the mtime (touch -r would keep the real atime).
			_ = os.Chtimes(dir, fi.ModTime(), fi.ModTime())
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// iconURI finds <iconsDir>/<theme>/places/<size>/<base>.{png,svg} (size 48 first, then the other
// sizes), falling back to the plain "folder" icon, and returns it as a file:// URI ("" if none).
func iconURI(theme, base string) string {
	places := filepath.Join(iconsDir, theme, "places")
	sizes := []string{"48"}
	if ents, err := os.ReadDir(places); err == nil {
		for _, e := range ents {
			if e.IsDir() && e.Name() != "48" {
				sizes = append(sizes, e.Name())
			}
		}
	}
	names := []string{base}
	if base != "folder" {
		names = append(names, "folder")
	}
	for _, n := range names {
		for _, sz := range sizes {
			for _, ext := range []string{".png", ".svg"} {
				p := filepath.Join(places, sz, n+ext)
				if _, err := os.Stat(p); err == nil {
					return "file://" + p
				}
			}
		}
	}
	return ""
}
