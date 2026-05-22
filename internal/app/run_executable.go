package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// resolveExecutableOpenPath returns the absolute path to run when Enter should execute a file.
func (a *App) resolveExecutableOpenPath(p *panel.State) (string, bool) {
	if a.model.ViewMode != ui.ViewBrowser || !a.config.RunExecutablesOnEnter {
		return "", false
	}
	if p.Path.IsRemote() {
		return "", false
	}

	var path string
	var mode editTargetMode
	if a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip && p.SelectionsStripCount() > 0 {
		selPath, ok := p.SelectedPathAtStripIndex(p.SelectionsStripCursor)
		if !ok {
			return "", false
		}
		path, mode = classifyEditPath(filepath.Clean(selPath))
	} else {
		entry, ok := p.CurrentEntry()
		if !ok {
			return "", false
		}
		if entry.Type == localfs.EntryDirectory {
			return "", false
		}
		path = filepath.Clean(entry.Path)
		if entry.Type == localfs.EntrySymlink {
			exec, err := localfs.PathIsExecutable(path)
			if err != nil {
				return "", false
			}
			if !exec {
				return "", false
			}
			return path, true
		}
		if entry.Type == localfs.EntryFile && localfs.ModeIsExecutable(entry.Mode) {
			return path, true
		}
		exec, err := localfs.PathIsExecutable(path)
		if err != nil || !exec {
			return "", false
		}
		return path, true
	}

	switch mode {
	case editTargetDir, editTargetNone, editTargetMissing:
		return "", false
	case editTargetFile:
		exec, err := localfs.PathIsExecutable(path)
		if err != nil || !exec {
			return "", false
		}
		return path, true
	default:
		return "", false
	}
}

func formatExecuteCommandLine(panelDir, path string) string {
	panelDir = filepath.Clean(panelDir)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(panelDir, path)
	if err != nil {
		return path
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	if rel == "." {
		return filepath.Base(path)
	}
	return rel
}

func (a *App) runExecutableFromPanel(path string) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	p := a.activePanel()
	if p.Path.IsRemote() {
		a.setTransientMessage("Run is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	path = filepath.Clean(path)
	if path == "" || path == "." {
		a.setErrorMessage("Run", os.ErrInvalid)
		return
	}
	if _, err := os.Stat(path); err != nil {
		a.setErrorMessage("Run", err)
		return
	}

	workDir := p.PathString()
	cmdLine := formatExecuteCommandLine(workDir, path)
	argv := []string{path}

	rowIdx := a.appendFileExecuteCommandRow(cmdLine, path)
	a.openCommandsView()
	a.model.CommandsView.Selected = rowIdx
	a.model.CommandsView.FocusPane = 0
	a.model.CommandsView.ListScroll = 0
	a.model.CommandsView.StdoutScroll = 0
	a.model.CommandsView.StderrScroll = 0
	a.ensureCommandsViewSelectionVisible()

	a.commandsBatchesInflight.Add(1)
	go a.runFileExecuteCommand(a.commandsCtx, rowIdx, argv, workDir)
}

func (a *App) appendFileExecuteCommandRow(cmdLine, targetPath string) int {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	idx := len(a.model.CommandsList)
	a.model.CommandsList = append(a.model.CommandsList, ui.CommandRunEntry{
		ID:              cmdrun.NewRunID(),
		Kind:            ui.CommandRunKindFileExecute,
		UserCommandLine: cmdLine,
		TargetPath:      absPathClean(targetPath),
		Phase:           ui.CommandRunPending,
		ExitCode:        -1,
	})
	return idx
}

func (a *App) runFileExecuteCommand(ctx context.Context, idx int, argv []string, workDir string) {
	defer func() {
		a.commandsBatchesInflight.Add(-1)
		a.postCommandWake()
	}()
	select {
	case <-ctx.Done():
		a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunDone
			e.ExitCode = -1
			if e.ErrorMsg == "" {
				e.ErrorMsg = "Canceled"
			}
		})
		a.postCommandWake()
		return
	default:
	}
	a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
		e.Phase = ui.CommandRunRunning
	})
	a.postCommandWake()

	res := cmdrun.Run(ctx, argv, workDir, cmdrun.MaxStreamBytes)
	a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
		e.Phase = ui.CommandRunDone
		e.Stdout = string(res.Stdout)
		e.Stderr = string(res.Stderr)
		if res.LaunchErr != nil {
			e.ErrorMsg = res.LaunchErr.Error()
			e.ExitCode = -1
		} else {
			e.ExitCode = res.ExitCode
		}
	})
	a.postCommandWake()
}
