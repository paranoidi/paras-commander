package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func runDetachedXDGOpen(path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// openPanelPathInExternalBrowser runs xdg-open on the given panel's directory (freedesktop GUI file manager).
func (a *App) openPanelPathInExternalBrowser(panelID int) {
	p := filepath.Clean(a.panelByID(panelID).PathString())
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
