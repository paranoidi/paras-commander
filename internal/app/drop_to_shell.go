package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/shell"
	"github.com/paranoidi/paras-commander/internal/ui"
)

var dropToShellRunner = func(ctx context.Context, argv []string) error {
	return shell.RunInteractive(ctx, argv)
}

func (a *App) dropToShell() {
	if a.model.ModalDialogOpen() {
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
	active := a.activePanel()
	if panel.CleanPathString(active.PathString()) == panel.CleanPathString(shellWd) {
		return
	}
	if fi, statErr := os.Stat(shellWd); statErr != nil || !fi.IsDir() {
		return
	}
	if navErr := a.navigatePanelToDirectory(a.model.ActivePanel, shellWd, ""); navErr != nil {
		a.setErrorMessage("Shell", navErr)
	}
}

func (a *App) refreshAfterDropToShell() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Shell", err)
	}
}
