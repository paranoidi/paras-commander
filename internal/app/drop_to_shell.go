package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/shell"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"github.com/paranoidi/paras-commander/internal/ui"
)

var dropToShellRunner = func(ctx context.Context, argv []string) error {
	return shell.RunInteractive(ctx, argv)
}

func (a *App) dropToShell() {
	if a.model.ModalDialogOpen() {
		return
	}
	if a.config.Shell.Persistent && a.persistentShellToggle() {
		return
	}
	p := a.activePanel()
	if p.Path.IsRemote() {
		a.setErrorMessage("Shell", fmt.Errorf("shell is not available on remote panels"))
		return
	}
	panelDir, err := p.Path.FilePath()
	if err != nil {
		a.setErrorMessage("Shell", err)
		return
	}
	panelDir = strings.TrimSpace(panelDir)
	if panelDir == "" || panelDir == "." {
		a.setErrorMessage("Shell", fmt.Errorf("no panel path"))
		return
	}
	if fi, statErr := os.Stat(panelDir); statErr != nil {
		a.setErrorMessage("Shell", statErr)
		return
	} else if !fi.IsDir() {
		a.setErrorMessage("Shell", fmt.Errorf("not a directory: %s", panelDir))
		return
	}

	argv, err := a.shellArgv()
	if err != nil {
		a.setErrorMessage("Shell", err)
		return
	}

	if err := a.withTerminalReleased(func() error {
		prevWd, wdErr := os.Getwd()
		if wdErr != nil {
			return fmt.Errorf("get working directory: %w", wdErr)
		}
		if chdirErr := os.Chdir(panelDir); chdirErr != nil {
			return fmt.Errorf("chdir to panel: %w", chdirErr)
		}
		defer func() { _ = os.Chdir(prevWd) }()

		runErr := dropToShellRunner(context.Background(), argv)
		if a.config.Shell.SyncCwdOnReturn {
			a.syncPanelCwdAfterShell()
		}
		return runErr
	}); err != nil {
		a.setErrorMessage("Shell", err)
	}
	a.refreshAfterDropToShell()
}

func (a *App) shellArgv() ([]string, error) {
	cmdLine := strings.TrimSpace(a.config.Shell.Command)
	if cmdLine != "" {
		return cmdrun.ParseCommandArgv(cmdLine)
	}
	return shell.ShellArgv(shell.ResolveShell()), nil
}

func (a *App) syncPanelCwdAfterShell() {
	shellWd, err := os.Getwd()
	if err != nil {
		return
	}
	a.syncActivePanelToDir(shellWd)
}

// syncActivePanelToDir navigates the active panel to shellWd unless it already shows it.
// The panel path is also compared symlink-resolved: /proc/<pid>/cwd is fully resolved while
// the panel may display a path through a symlink.
func (a *App) syncActivePanelToDir(shellWd string) {
	active := a.activePanel()
	panelPath := panel.CleanPathString(active.PathString())
	if panelPath == panel.CleanPathString(shellWd) {
		return
	}
	if resolved, err := filepath.EvalSymlinks(panelPath); err == nil &&
		panel.CleanPathString(resolved) == panel.CleanPathString(shellWd) {
		return
	}
	if fi, statErr := os.Stat(shellWd); statErr != nil || !fi.IsDir() {
		return
	}
	if navErr := a.navigatePanelToDirectory(a.model.ActivePanel, shellWd, ""); navErr != nil {
		a.setErrorMessage("Shell", navErr)
	}
}

// persistentShellToggle shows the MC-style persistent subshell session. It reports false when
// the persistent path is unavailable (custom shell.command, non-Linux, PTY start failure) so
// dropToShell falls back to the one-shot shell.
func (a *App) persistentShellToggle() bool {
	if strings.TrimSpace(a.config.Shell.Command) != "" {
		return false
	}
	panelDir := a.localActivePanelDir()
	chdirBusy := false
	if a.subshell != nil && !a.subshell.Alive() {
		a.closeSubshell()
	}
	if a.subshell == nil {
		sub, err := subshell.Start(subshell.StartOptions{Dir: panelDir})
		if err != nil {
			if !errors.Is(err, subshell.ErrUnsupportedPlatform) {
				a.setTransientMessage(fmt.Sprintf("Shell: persistent session unavailable (%v)", err), ui.MessageUrgencyWarn)
			}
			return false
		}
		a.subshell = sub
	} else if panelDir != "" {
		chdirBusy = errors.Is(a.subshellChdirIfNeeded(panelDir), subshell.ErrBusy)
	}

	_, err := a.subshell.RunVisible(a.screen)
	// Same post-Resume housekeeping as withTerminalReleased: force a full repaint.
	a.lastScreenContentHash = 0
	a.screen.HideCursor()
	if err != nil {
		a.setErrorMessage("Shell", err)
	}
	if chdirBusy {
		// Only visible now: transient messages render in the TUI, not in the shell.
		a.setTransientMessage("Shell is busy — panel directory was not sent to the shell", ui.MessageUrgencyWarn)
	}
	if !a.subshell.Alive() {
		a.closeSubshell()
	} else if a.config.Shell.SyncCwdOnReturn && !a.activePanel().Path.IsRemote() {
		if cwd, cwdErr := a.subshell.Cwd(); cwdErr == nil {
			a.syncActivePanelToDir(cwd)
		}
	}
	a.refreshAfterDropToShell()
	a.render()
	return true
}

// localActivePanelDir returns the active panel directory, or "" when it is remote or
// unavailable (the persistent shell then keeps its current cwd).
func (a *App) localActivePanelDir() string {
	p := a.activePanel()
	if p.Path.IsRemote() {
		return ""
	}
	dir, err := p.Path.FilePath()
	if err != nil {
		return ""
	}
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == "." {
		return ""
	}
	if fi, statErr := os.Stat(dir); statErr != nil || !fi.IsDir() {
		return ""
	}
	return dir
}

// subshellChdirIfNeeded injects cd into the idle subshell when its cwd differs from dir.
// A busy shell keeps its cwd (same as MC: no injection while a foreground command runs);
// that surfaces as [subshell.ErrBusy].
func (a *App) subshellChdirIfNeeded(dir string) error {
	if cwd, err := a.subshell.Cwd(); err == nil {
		if panel.CleanPathString(cwd) == panel.CleanPathString(dir) {
			return nil
		}
		if resolved, rerr := filepath.EvalSymlinks(dir); rerr == nil &&
			panel.CleanPathString(cwd) == panel.CleanPathString(resolved) {
			return nil
		}
	}
	return a.subshell.Chdir(dir)
}

// closeSubshell terminates the persistent shell session; the next toggle starts a fresh one.
func (a *App) closeSubshell() {
	if a.subshell == nil {
		return
	}
	_ = a.subshell.Close()
	a.subshell = nil
}

func (a *App) refreshAfterDropToShell() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Shell", err)
	}
}
