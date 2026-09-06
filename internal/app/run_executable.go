package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
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

// resolveExecuteBackground reports whether path should run in background mode: tracked as a
// Commands-view row but without switching ViewMode there, per the first matching
// [[panels.execute_rules]] entry (see PanelsConfig.ExecuteRules). No match means foreground
// (today's default: switch to Commands view immediately).
func resolveExecuteBackground(rules []config.ExecuteRule, shellPatterns bool, name string, mode fs.FileMode) bool {
	if len(rules) == 0 {
		return false
	}
	ctx := &entrymatch.Context{
		Row:           &localfs.Entry{Name: name, Type: localfs.EntryFile, Mode: mode},
		ShellPatterns: shellPatterns,
	}
	for _, rule := range rules {
		if ok, err := entrymatch.EvalWhenAny(rule.When, ctx); err == nil && ok {
			return rule.Background
		}
	}
	return false
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
	fi, err := os.Stat(path)
	if err != nil {
		a.setErrorMessage("Run", err)
		return
	}

	workDir := p.PathString()
	cmdLine := formatExecuteCommandLine(workDir, path)
	argv := []string{path}
	background := resolveExecuteBackground(a.config.Panels.ExecuteRules, a.config.Panels.ShellPatterns, filepath.Base(path), fi.Mode())

	rowIdx := a.commandsCtrl.AppendEntry(ui.CommandRunEntry{
		ID:              cmdrun.NewRunID(),
		Kind:            ui.CommandRunKindFileExecute,
		UserCommandLine: cmdLine,
		TargetPath:      textutil.AbsPathClean(path),
		Phase:           ui.CommandRunPending,
		ExitCode:        -1,
	})
	if !background {
		a.commandsCtrl.OpenViewAt(rowIdx)
	}

	a.commandsCtrl.BeginBatch()
	go a.runFileExecuteCommand(a.commandsCtrl.Context(), rowIdx, argv, workDir, cmdLine, background)
}

func (a *App) runFileExecuteCommand(ctx context.Context, idx int, argv []string, workDir, cmdLine string, background bool) {
	defer a.commandsCtrl.EndBatch()
	select {
	case <-ctx.Done():
		a.commandsCtrl.PatchEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunDone
			e.ExitCode = -1
			if e.ErrorMsg == "" {
				e.ErrorMsg = "Canceled"
			}
		})
		if background {
			a.commandsCtrl.PostWake(commandsctrl.WakePayload{RefreshBrowserPanel: true})
		} else {
			a.commandsCtrl.PostRenderWake()
		}
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
	if !background {
		a.commandsCtrl.PostRenderWake()
		return
	}
	wp := commandsctrl.WakePayload{RefreshBrowserPanel: true}
	if log, banner, urg, ok := backgroundRunNotify("Run: "+cmdLine, res); ok {
		wp.NotifyLog = log
		wp.NotifyBanner = banner
		wp.NotifyUrg = urg
	}
	a.commandsCtrl.PostWake(wp)
}
