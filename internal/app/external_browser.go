package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
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

// handleNavOpen enters directories via panel.Enter; when the selection is a POSIX-executable
// regular file and run_executables_on_enter is enabled, runs it in the Commands view; otherwise
// when open_files_externally is enabled, launches xdg-open on the path.
func (a *App) handleNavOpen(activePanel *panel.State, viewportRows int) {
	entered, err := activePanel.Enter(viewportRows)
	if err != nil {
		a.setErrorMessage("Enter failed", err)
		return
	}
	if entered {
		return
	}
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if path, ok := a.resolveExecutableOpenPath(activePanel); ok {
		a.runExecutableFromPanel(path)
		return
	}
	if !a.config.OpenFilesExternally {
		return
	}
	entry, ok := activePanel.CurrentEntry()
	if !ok || entry.Type == localfs.EntryDirectory {
		return
	}
	p := filepath.Clean(entry.Path)
	if p == "" || p == "." {
		a.setErrorMessage("External open", fmt.Errorf("no path"))
		return
	}
	if _, err := os.Stat(p); err != nil {
		a.setErrorMessage("External open", err)
		return
	}
	if err := runDetachedXDGOpen(p); err != nil {
		a.setErrorMessage("External open", err)
		return
	}
	a.setTransientMessage("Opened file externally", ui.MessageUrgencyInfo)
}
