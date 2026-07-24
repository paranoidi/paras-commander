package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// resolveExecutableOpenPath returns the absolute path to run when Enter should execute a file.
func (a *App) resolveExecutableOpenPath(p *panel.State) (string, bool) {
	if a.model.ViewMode != ui.ViewBrowser || !a.config.Panels.RunExecutablesOnEnter {
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
		runnable, err := localfs.PathLooksRunnable(path)
		if err != nil || !runnable {
			return "", false
		}
		return path, true
	}

	switch mode {
	case editTargetDir, editTargetNone, editTargetMissing:
		return "", false
	case editTargetFile:
		runnable, err := localfs.PathLooksRunnable(path)
		if err != nil || !runnable {
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

	rowIdx := a.commandsCtrl.AppendEntry(ui.CommandRunEntry{
		ID:              cmdrun.NewRunID(),
		Kind:            ui.CommandRunKindFileExecute,
		UserCommandLine: cmdLine,
		TargetPath:      textutil.AbsPathClean(path),
		Phase:           ui.CommandRunPending,
		ExitCode:        -1,
	})
	a.commandsCtrl.OpenViewAt(rowIdx)

	a.commandsCtrl.BeginBatch()
	go a.runFileExecuteCommand(a.commandsCtrl.Context(), rowIdx, argv, workDir)
}

func (a *App) runFileExecuteCommand(ctx context.Context, idx int, argv []string, workDir string) {
	defer func() {
		a.commandsCtrl.EndBatch()
		a.commandsCtrl.PostRenderWake()
	}()
	select {
	case <-ctx.Done():
		a.commandsCtrl.PatchEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunDone
			e.ExitCode = -1
			if e.ErrorMsg == "" {
				e.ErrorMsg = "Canceled"
			}
		})
		a.commandsCtrl.PostRenderWake()
		return
	default:
	}
	a.commandsCtrl.PatchEntry(idx, func(e *ui.CommandRunEntry) {
		e.Phase = ui.CommandRunRunning
	})
	a.commandsCtrl.PostRenderWake()

	res := cmdrun.RunTracked(ctx, argv, workDir, cmdrun.MaxStreamBytes, func(p *os.Process) {
		a.commandsCtrl.SetProcess(idx, p)
	})
	a.commandsCtrl.UnregisterProc(idx)
	a.commandsCtrl.PatchEntry(idx, func(e *ui.CommandRunEntry) {
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
	a.commandsCtrl.PostRenderWake()
}
