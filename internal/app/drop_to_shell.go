package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/shell"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"github.com/paranoidi/paras-commander/internal/textutil"
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
	fresh, ok := a.ensureSubshell(panelDir)
	if !ok {
		return false
	}
	chdirBusy := false
	if !fresh && panelDir != "" {
		chdirBusy = errors.Is(a.subshellChdirIfNeeded(panelDir), subshell.ErrBusy)
	}
	a.runShellVisible(chdirBusy)
	return true
}

// ensureSubshell lazily (re)starts the persistent session; a fresh child starts in panelDir.
// ok is false when the persistent path is unavailable (a warning is shown unless the platform
// simply does not support it).
func (a *App) ensureSubshell(panelDir string) (fresh, ok bool) {
	if a.subshell != nil && !a.subshell.Alive() {
		a.closeSubshell()
	}
	if a.subshell != nil {
		return false, true
	}
	sub, err := subshell.Start(subshell.StartOptions{Dir: panelDir})
	if err != nil {
		if !errors.Is(err, subshell.ErrUnsupportedPlatform) {
			a.setTransientMessage(fmt.Sprintf("Shell: persistent session unavailable (%v)", err), ui.MessageUrgencyWarn)
		}
		return false, false
	}
	a.subshell = sub
	// Start the emulator feed with the shell (not with the panel) so no output
	// is ever lost: a full-screen session's content is captured even when the
	// embedded panel has never been opened.
	if cols, rows := a.screen.Size(); cols > 0 && rows > 0 {
		if feed, feedErr := sub.StartPanelFeed(cols, rows, a.postTerminalWake); feedErr == nil {
			a.terminalFeed = feed
		}
	}
	return true, true
}

// runShellVisible shows the persistent shell session and reconciles the TUI afterwards.
func (a *App) runShellVisible(chdirBusy bool) {
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
	} else {
		a.syncPanelFromSubshellCwd()
		// Restore the embedded panel's grid after the full-screen session (RunVisible
		// restarted the feed reader at full-screen dims; the resize WINCH repaints).
		if a.terminalFeed != nil {
			if cols, rows, ok := a.terminalPanelContentDims(); ok {
				a.terminalFeed.Resize(cols, rows)
			}
		}
	}
	a.refreshAfterDropToShell()
	a.render()
}

// shellInsertPaths (app.shell-insert-paths, Alt+Enter) puts the selected — or focused — paths
// on the persistent shell's command line as quoted absolute paths and enters the shell.
func (a *App) shellInsertPaths() {
	if a.model.ModalDialogOpen() || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if !a.config.Shell.Persistent || strings.TrimSpace(a.config.Shell.Command) != "" {
		a.setTransientMessage("Send paths to shell requires the persistent shell", ui.MessageUrgencyWarn)
		return
	}
	p := a.activePanel()
	if p.Path.IsRemote() {
		a.setTransientMessage("Send paths to shell is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	paths := shellInsertPathList(p)
	if len(paths) == 0 {
		a.setTransientMessage("Send paths to shell: no file selected", ui.MessageUrgencyWarn)
		return
	}
	panelDir := a.localActivePanelDir()
	fresh, ok := a.ensureSubshell(panelDir)
	if !ok {
		// ensureSubshell already warned about real failures; non-Linux is out of scope.
		return
	}
	if !fresh && panelDir != "" {
		if errors.Is(a.subshellChdirIfNeeded(panelDir), subshell.ErrBusy) {
			a.setTransientMessage("Shell is busy — paths not sent", ui.MessageUrgencyWarn)
			return
		}
	}
	quoted := make([]string, len(paths))
	for i, pth := range paths {
		quoted[i] = subshell.QuoteArg(pth)
	}
	if err := a.subshell.InsertText(strings.Join(quoted, " ") + " "); err != nil {
		if errors.Is(err, subshell.ErrBusy) {
			a.setTransientMessage("Shell is busy — paths not sent", ui.MessageUrgencyWarn)
		} else {
			a.setErrorMessage("Shell", err)
		}
		return
	}
	if a.model.TerminalPanel.Visible && a.terminalFeed != nil {
		a.model.TerminalPanel.Focused = true
		a.render()
		return
	}
	a.runShellVisible(false)
}

// shellInsertPathList returns the sorted selected paths, or the focused entry, as absolute
// cleaned paths. The parent (..) row yields nothing.
func shellInsertPathList(p *panel.State) []string {
	if len(p.SelectedPaths) > 0 {
		paths := make([]string, 0, len(p.SelectedPaths))
		for sel := range p.SelectedPaths {
			paths = append(paths, textutil.AbsPathClean(sel))
		}
		sort.Strings(paths)
		return paths
	}
	entry, ok := p.CurrentEntry()
	if !ok || entry.Name == ".." {
		return nil
	}
	return []string{textutil.AbsPathClean(entry.Path)}
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
// The embedded terminal panel is a view of that session, so it closes with it.
func (a *App) closeSubshell() {
	a.closeTerminalPanel()
	a.terminalFeed = nil // subshell.Close() kills the feed internally
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
