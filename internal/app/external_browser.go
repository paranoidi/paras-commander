package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

// openPanelPathInExternalBrowser runs xdg-open on the given panel's directory (freedesktop GUI file manager).
func (a *App) openPanelPathInExternalBrowser(panelID int) {
	a.openDirInExternalBrowser(a.panelByID(panelID).PathString())
}

// openDirInExternalBrowser is the single entry point for "open this directory in the GUI file manager".
func (a *App) openDirInExternalBrowser(dir string) {
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
	a.setTransientMessage("Opened folder in external browser", ui.MessageUrgencyInfo)
}

// openFileExternally opens a regular file with the default opener when open_files_externally is on.
// When xdg-open is not on PATH the file opens in the fullscreen preview instead; its async preview
// gate shows the reason in the pane for files it cannot render.
func (a *App) openFileExternally(path string) {
	if !a.config.Panels.OpenFilesExternally {
		return
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
	a.setTransientMessage("Opened file externally", ui.MessageUrgencyInfo)
}
