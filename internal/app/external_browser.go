package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// runDetachedXDGOpen launches xdg-open on path without waiting for it. Tests replace this.
var runDetachedXDGOpen = func(path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// defaultAppForPath returns the desktop id of path's default application (xdg-mime). Tests replace this.
var defaultAppForPath = func(path string) (string, error) {
	mime, err := exec.Command("xdg-mime", "query", "filetype", path).Output()
	if err != nil {
		return "", err
	}
	mt := strings.TrimSpace(string(mime))
	id, err := exec.Command("xdg-mime", "query", "default", mt).Output()
	if err != nil {
		return "", err
	}
	if d := strings.TrimSpace(string(id)); d != "" {
		return d, nil
	}
	return "", fmt.Errorf("no default application for %s (%s)", filepath.Base(path), mt)
}

// runDetachedLaunch launches the desktop application desktopID with paths (gtk-launch) without
// waiting for it. Tests replace this.
var runDetachedLaunch = func(desktopID string, paths []string) error {
	cmd := exec.Command("gtk-launch", append([]string{desktopID}, paths...)...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// openFilesResultPayload reports the outcome of openFilesExternally back to the UI goroutine.
type openFilesResultPayload struct {
	count int
	err   error
}

// openFilesExternally opens every path in their shared default application with one launch.
// Files whose default applications differ launch nothing and report an error.
func (a *App) openFilesExternally(paths []string) {
	hosts := make([]string, len(paths))
	for i, p := range paths {
		if host, ok := hostPathOf(p); ok {
			p = host
		}
		hosts[i] = filepath.Clean(p)
	}
	screen := a.screen
	go func() {
		res := openFilesResultPayload{count: len(hosts), err: launchSharedApp(hosts)}
		_ = screen.PostEvent(tcell.NewEventInterrupt(res))
	}()
}

func launchSharedApp(paths []string) error {
	byApp := map[string][]string{}
	for _, p := range paths {
		id, err := defaultAppForPath(p)
		if err != nil {
			return err
		}
		byApp[id] = append(byApp[id], p)
	}
	if len(byApp) > 1 {
		parts := make([]string, 0, len(byApp))
		for id, ps := range byApp {
			parts = append(parts, fmt.Sprintf("%s (%d)", id, len(ps)))
		}
		sort.Strings(parts)
		return fmt.Errorf("selected files open with different applications: %s", strings.Join(parts, ", "))
	}
	for id, ps := range byApp {
		return runDetachedLaunch(id, ps)
	}
	return nil
}

// openPanelPathInExternalBrowser runs xdg-open on the given panel's directory (freedesktop GUI file manager).
func (a *App) openPanelPathInExternalBrowser(panelID int) {
	a.openDirInExternalBrowser(a.panelByID(panelID).PathString())
}

// openDirInExternalBrowser is the single entry point for "open this directory in the GUI file manager".
// A fileindex:// dir opens its real directory on disk.
func (a *App) openDirInExternalBrowser(dir string) {
	if host, ok := hostPathOf(dir); ok {
		dir = host
	}
	p := filepath.Clean(dir)
	if p == "" || p == "." {
		a.setErrorMessage("External browser", fmt.Errorf("no panel path"))
		return
	}
	fi, err := os.Stat(p)
	if err != nil {
		a.setErrorMessage("External browser", err)
		return
	}
	if !fi.IsDir() {
		a.setErrorMessage("External browser", fmt.Errorf("not a directory: %s", p))
		return
	}
	if err := runDetachedXDGOpen(p); err != nil {
		a.setErrorMessage("External browser", err)
		return
	}
	a.setTransientMessage("Opened folder in external browser", ui.MessageUrgencySuccess)
}

// openFileExternally opens a regular file with the default opener when open_files_externally is on.
// When xdg-open is not on PATH the file opens in the fullscreen preview instead; its async preview
// gate shows the reason in the pane for files it cannot render. A fileindex:// path opens its
// real file on disk.
func (a *App) openFileExternally(path string) {
	if !a.config.Panels.OpenFilesExternally {
		return
	}
	if host, ok := hostPathOf(path); ok {
		path = host
	}
	p := filepath.Clean(path)
	if p == "" || p == "." {
		a.setErrorMessage("External open", fmt.Errorf("no path"))
		return
	}
	if _, err := os.Stat(p); err != nil {
		a.setErrorMessage("External open", err)
		return
	}
	if err := runDetachedXDGOpen(p); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			if err := a.previewCtrl.OpenFullscreenFilePreviewAt(p, false); err != nil {
				a.setErrorMessage("External open", err)
			}
			return
		}
		a.setErrorMessage("External open", err)
		return
	}
	a.setTransientMessage("Opened file externally", ui.MessageUrgencySuccess)
}
