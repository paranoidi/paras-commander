package app

import (
	"path/filepath"
	"slices"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Directory watching (ADR 0025): inotify only says "this directory changed". The wake feeds the
// same schedulePanelListingRefresh the periodic ticker uses, so listing, backoff, epoch checks,
// diff-before-apply and the volume-contention gate stay in one place.

// dirWatchWakePayload is posted (throttled) after the watcher reported changes; the changed
// directories wait in asyncWakeQueues.dirWatch.
type dirWatchWakePayload struct{}

// dirWatchSig is what decides the watch set of one panel: its directory, its listing (the expanded
// tree dirs are re-rooted with it), the expanded set between listings, and the layout. Comparing
// it is O(1); the set is recomputed only when it changes.
type dirWatchSig struct {
	path   pathloc.Path
	epoch  uint64
	expand uint64
	tree   bool
}

// queueDirWatchWake records a changed directory and, for the first one of a window, schedules the
// wake. It is a throttle rather than a trailing debounce: a continuous stream of changes still
// yields one wake per window. Safe from any goroutine.
func (a *App) queueDirWatchWake(dir string) {
	if !asyncWakes(a).dirWatch.push(dir) {
		return
	}
	time.AfterFunc(config.DefaultDirWatchDebounce, func() {
		select {
		case <-a.jobStopCh:
			return
		default:
		}
		postGuaranteedWake(a.screen, dirWatchWakePayload{})
	})
}

// handleDirWatchWake re-lists the panels showing a directory that changed. Under the same gate as
// the periodic tick; while gated the changes are dropped and the next tick heals them.
func (a *App) handleDirWatchWake() {
	dirs := asyncWakes(a).dirWatch.drain()
	// A directory replaced under its path (rm -rf x; mkdir x) lost its watch and may list the same,
	// so no signature changes: re-send the set, the watcher adds only what is missing.
	if a.dirWatcher != nil {
		a.dirWatcher.Set(a.desiredWatchDirs())
	}
	if len(dirs) == 0 || a.model.ViewMode != ui.ViewBrowser || a.model.ModalDialogOpen() {
		return
	}
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		ids, matched := matchChangedDirs(p.Path, watchTreeIDs(p), dirs)
		if len(matched) == 0 {
			continue
		}
		if a.schedulePanelListingRefresh(id, ids...) {
			for _, d := range matched {
				a.queueDirWatchWake(d) // transient skip: ask again after one more window
			}
		}
	}
}

// watchTreeIDs are the expanded tree directories of a panel (the set the periodic plan can
// re-list); nil outside tree layout.
func watchTreeIDs(p *panel.State) []string {
	if p.ListLayout != panel.ListLayoutTree {
		return nil
	}
	return p.TreePrefetchIDs(p.Path)
}

// localDirHost is the cleaned host path of a local location.
func localDirHost(loc pathloc.Path) (string, bool) {
	if !loc.IsLocal() {
		return "", false
	}
	host, err := loc.FilePath()
	if err != nil {
		return "", false
	}
	return filepath.Clean(host), true
}

// panelWatchDirs lists the host directories to watch for a panel: its root first, then its
// expanded tree directories, at most limit in all. Only local (file://) locations are watched, so
// SFTP and direct fileindex:// panels yield nothing.
func panelWatchDirs(root pathloc.Path, treeIDs []string, limit int) []string {
	host, ok := localDirHost(root)
	if !ok || limit <= 0 {
		return nil
	}
	dirs := []string{host}
	for _, id := range treeIDs {
		if len(dirs) >= limit {
			break
		}
		if loc, err := pathloc.Parse(id); err == nil {
			if h, ok := localDirHost(loc); ok {
				dirs = append(dirs, h)
			}
		}
	}
	return dirs
}

// matchChangedDirs picks, from the changed host directories, the ones a panel shows: its root
// and its expanded tree directories. It returns the tree node ids to re-list and every matched
// directory (for a retry).
func matchChangedDirs(root pathloc.Path, treeIDs []string, changed []string) (ids, matched []string) {
	rootHost, rootOK := localDirHost(root)
	var treeHost map[string]string
	for _, d := range changed {
		if slices.Contains(matched, d) {
			continue
		}
		if rootOK && d == rootHost {
			matched = append(matched, d)
			continue
		}
		if treeHost == nil {
			treeHost = make(map[string]string, len(treeIDs))
			for _, id := range treeIDs {
				if loc, err := pathloc.Parse(id); err == nil {
					if h, ok := localDirHost(loc); ok {
						treeHost[h] = id
					}
				}
			}
		}
		if id, ok := treeHost[d]; ok {
			matched = append(matched, d)
			ids = append(ids, id)
		}
	}
	return ids, matched
}

// reconcileDirWatch keeps the watcher's directory set in step with what the panels show. It runs
// after every event, so it does nothing unless a panel's signature changed.
func (a *App) reconcileDirWatch() {
	if a.dirWatcher == nil {
		return
	}
	changed := false
	for i, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		sig := dirWatchSig{p.Path, p.ListingEpoch, p.TreeExpandGen, p.ListLayout == panel.ListLayoutTree}
		if sig != a.dirWatchSig[i] {
			a.dirWatchSig[i] = sig
			changed = true
		}
	}
	if !changed {
		return
	}
	a.dirWatcher.Set(a.desiredWatchDirs())
}

// desiredWatchDirs merges both panels' watch directories, deduplicated.
func (a *App) desiredWatchDirs() []string {
	var dirs []string
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		p := a.panelByID(id)
		for _, d := range panelWatchDirs(p.Path, watchTreeIDs(p), config.DefaultDirWatchMaxPerPanel) {
			if !slices.Contains(dirs, d) {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}
