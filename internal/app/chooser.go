package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func expandUserPath(raw, homeDir string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}
	switch {
	case raw == "~" && homeDir != "":
		return filepath.Clean(homeDir), nil
	case strings.HasPrefix(raw, "~/") && homeDir != "":
		return filepath.Clean(filepath.Join(homeDir, strings.TrimPrefix(raw, "~/"))), nil
	default:
		return filepath.Clean(raw), nil
	}
}

func writeChooserSelection(chooserFile, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(chooserFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chooser directory: %w", err)
	}
	return os.WriteFile(chooserFile, []byte(abs+"\n"), 0o600)
}

func (a *App) chooserMode() bool {
	return a.chooserFile != ""
}

func (a *App) applyChooserSelect(raw string) error {
	path, err := expandUserPath(raw, a.model.UserHomeDir)
	if err != nil {
		return err
	}
	vr := a.panelViewportRows(ui.LeftPanel)
	info, statErr := os.Stat(path)
	if statErr == nil {
		if info.IsDir() {
			return a.model.Left.NavigateTo(path, "", vr)
		}
		return a.model.Left.NavigateTo(filepath.Dir(path), filepath.Base(path), vr)
	}
	if !os.IsNotExist(statErr) {
		return statErr
	}
	dir := filepath.Dir(path)
	if err := a.model.Left.NavigateTo(dir, "", vr); err != nil {
		return err
	}
	if base := filepath.Base(path); base != "" && base != "." {
		a.model.Left.SelectVisibleEntry(base)
	}
	return nil
}

// handleNavOpen enters directories via panel.Enter; when the selection is a POSIX-executable
// regular file and run_executables_on_enter is enabled, runs it in the Commands view; otherwise
// when open_files_externally is enabled, launches xdg-open on the path. In chooser mode,
// Enter on a regular file writes the absolute path to the chooser file and quits.
func (a *App) handleNavOpen(activePanel *panel.State, viewportRows int) bool {
	entered, err := activePanel.Enter(viewportRows)
	if err != nil {
		a.setErrorMessage("Enter failed", err)
		return false
	}
	if entered {
		return false
	}
	if a.chooserMode() {
		return a.handleChooserOpen(activePanel)
	}
	if a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	if path, ok := a.resolveExecutableOpenPath(activePanel); ok {
		a.runExecutableFromPanel(path)
		return false
	}
	if !a.config.OpenFilesExternally {
		return false
	}
	entry, ok := activePanel.CurrentEntry()
	if !ok || entry.Type == localfs.EntryDirectory {
		return false
	}
	p := filepath.Clean(entry.Path)
	if p == "" || p == "." {
		a.setErrorMessage("External open", fmt.Errorf("no path"))
		return false
	}
	if _, err := os.Stat(p); err != nil {
		a.setErrorMessage("External open", err)
		return false
	}
	if err := runDetachedXDGOpen(p); err != nil {
		a.setErrorMessage("External open", err)
		return false
	}
	a.setTransientMessage("Opened file externally", ui.MessageUrgencyInfo)
	return false
}

func (a *App) handleChooserOpen(activePanel *panel.State) bool {
	if a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	entry, ok := activePanel.CurrentEntry()
	if !ok || entry.Type == localfs.EntryDirectory {
		return false
	}
	p := filepath.Clean(entry.Path)
	if p == "" || p == "." {
		a.setErrorMessage("Chooser", fmt.Errorf("no path"))
		return false
	}
	if _, err := os.Stat(p); err != nil {
		a.setErrorMessage("Chooser", err)
		return false
	}
	if err := writeChooserSelection(a.chooserFile, p); err != nil {
		a.setErrorMessage("Chooser", err)
		return false
	}
	return a.handleQuitImmediate()
}
