package app

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// commandWakePayload wakes PollEvent after asynchronous command-run mutations.
type commandWakePayload struct{}

func (a *App) postCommandWake() {
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(commandWakePayload{}))
}

func (a *App) openRunForEachDialog() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	src, err := ops.ResolveSource(a.activePanel())
	if err != nil {
		a.setErrorMessage("Run for each", err)
		return
	}
	paths := ops.SourcePaths(src)
	dir := a.activePanel().Path
	msg := "Runs once per selected item; each path is appended as a separate argument.\nUses POSIX-style quoting (no shell)."
	fields := []ui.FileDialogField{{Label: "Command", Value: "", Cursor: 0}}
	a.model.FileDialog = ui.FileDialogState{
		Open:            true,
		DialogType:      ui.FileDialogRunForEach,
		Fields:          fields,
		FocusedField:    0,
		Message:         msg,
		RunForEachPaths: append([]string(nil), paths...),
		RunForEachDir:   dir,
	}
	a.clearTransientMessage()
}

func (a *App) executeRunForEach() {
	fd := a.model.FileDialog
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	cmdLine := strings.TrimSpace(field.Value)
	paths := append([]string(nil), fd.RunForEachPaths...)
	workDir := fd.RunForEachDir
	a.closeFileDialog()
	if len(paths) == 0 {
		a.setTransientMessage("No paths to run", ui.MessageUrgencyWarn)
		return
	}
	prefix, err := cmdrun.ParseCommandArgv(cmdLine)
	if err != nil {
		a.openMessageDialog("Run for each", err.Error())
		return
	}
	if len(prefix) == 0 {
		a.setTransientMessage("Command is empty", ui.MessageUrgencyWarn)
		return
	}
	entries := make([]ui.CommandRunEntry, len(paths))
	for i := range paths {
		entries[i] = ui.CommandRunEntry{
			ID:              cmdrun.NewRunID(),
			Kind:            ui.CommandRunKindRunForEach,
			UserCommandLine: cmdLine,
			TargetPath:      absPathClean(paths[i]),
			Phase:           ui.CommandRunPending,
			ExitCode:        -1,
		}
	}
	a.commandsMu.Lock()
	start := len(a.model.CommandsList)
	a.model.CommandsList = append(a.model.CommandsList, entries...)
	a.commandsMu.Unlock()

	a.openCommandsView()
	a.model.CommandsView.Selected = start
	a.model.CommandsView.FocusPane = 0
	a.model.CommandsView.ListScroll = 0
	a.model.CommandsView.StdoutScroll = 0
	a.model.CommandsView.StderrScroll = 0
	a.ensureCommandsViewSelectionVisible()

	a.commandsBatchesInflight.Add(1)
	go a.runForEachBatch(a.commandsCtx, start, paths, prefix, workDir)
}

func (a *App) runForEachBatch(ctx context.Context, start int, paths []string, prefix []string, workDir string) {
	defer func() {
		a.commandsBatchesInflight.Add(-1)
		a.postCommandWake()
	}()
	for i := range paths {
		select {
		case <-ctx.Done():
			a.markCommandsCanceled(start+i, len(paths)-i)
			a.postCommandWake()
			return
		default:
		}
		idx := start + i
		abs := absPathClean(paths[i])
		argv := append(append([]string(nil), prefix...), abs)

		a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunRunning
			e.TargetPath = abs
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
}

func (a *App) markCommandsCanceled(fromIdx, count int) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	list := a.model.CommandsList
	for j := 0; j < count && fromIdx+j < len(list); j++ {
		e := &list[fromIdx+j]
		if e.Phase == ui.CommandRunDone {
			continue
		}
		e.Phase = ui.CommandRunDone
		e.ExitCode = -1
		if e.ErrorMsg == "" {
			e.ErrorMsg = "Canceled"
		}
	}
}

func (a *App) patchCommandEntry(idx int, fn func(*ui.CommandRunEntry)) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	if idx < 0 || idx >= len(a.model.CommandsList) {
		return
	}
	fn(&a.model.CommandsList[idx])
}

func (a *App) commandsLen() int {
	a.commandsMu.RLock()
	defer a.commandsMu.RUnlock()
	return len(a.model.CommandsList)
}

func (a *App) toggleCommandsView() {
	if a.model.ViewMode == ui.ViewCommands {
		a.closeCommandsView()
		return
	}
	a.openCommandsView()
}

func (a *App) openCommandsView() {
	a.model.ViewMode = ui.ViewCommands
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.model.MenuDefinitions = menu.CommandsDefinitions(a.keys, a.keysCommands)
	a.model.Menu.ActiveMenu = menu.DefaultIndexCommands()
	if a.model.CommandsView.FocusPane < 0 || a.model.CommandsView.FocusPane > 2 {
		a.model.CommandsView.FocusPane = 0
	}
	a.ensureCommandsViewSelectionVisible()
}

func (a *App) closeCommandsView() {
	a.model.ViewMode = ui.ViewBrowser
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.model.MenuDefinitions = menu.BrowserDefinitions(a.keys)
	a.model.Menu.ActiveMenu = menu.DefaultIndex()
	a.model.CommandsView = ui.CommandsViewState{}
}

func (a *App) ensureCommandsViewSelectionVisible() {
	n := a.commandsLen()
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		a.model.CommandsView.EnsureSelectionVisible(n, 0)
		return
	}
	visible := ui.PanelListRows(layout.Left)
	a.model.CommandsView.EnsureSelectionVisible(n, visible)
}

func (a *App) tryDispatchCommands(actionID string) bool {
	switch actionID {
	case keymap.ActionCommandsOpen:
		a.toggleCommandsView()
		return true
	case keymap.ActionCommandsClose:
		if a.model.ViewMode == ui.ViewCommands {
			a.closeCommandsView()
		}
		return true
	default:
		return false
	}
}

func (a *App) hasRunningCommands() bool {
	return a.commandsBatchesInflight.Load() > 0
}

func (a *App) handleCommandsViewKey(event *tcell.EventKey) bool {
	a.clampCommandsFocusPane()
	switch event.Key() {
	case tcell.KeyEsc:
		a.closeCommandsView()
		return false
	case tcell.KeyTab:
		n := a.commandsViewFocusPaneCount()
		a.model.CommandsView.FocusPane = (a.model.CommandsView.FocusPane + 1) % n
		return false
	case tcell.KeyBacktab:
		n := a.commandsViewFocusPaneCount()
		a.model.CommandsView.FocusPane = (a.model.CommandsView.FocusPane + n - 1) % n
		return false
	}

	nextAction := a.actionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate()
	}
	if nextAction == keymap.ActionAppOpenMenu {
		a.openMenu()
		return false
	}
	if nextAction != "" && a.tryDispatchCommands(nextAction) {
		return false
	}
	if nextAction == keymap.ActionPanelExternalBrowser {
		a.dispatch(nextAction)
		return false
	}

	fp := a.model.CommandsView.FocusPane
	n := a.commandsLen()
	switch fp {
	case 0:
		beforeSel := a.model.CommandsView.Selected
		switch event.Key() {
		case tcell.KeyUp:
			if a.model.CommandsView.Selected > 0 {
				a.model.CommandsView.Selected--
			}
			if beforeSel != a.model.CommandsView.Selected {
				a.model.CommandsView.StdoutScroll = 0
				a.model.CommandsView.StderrScroll = 0
			}
			a.ensureCommandsViewSelectionVisible()
		case tcell.KeyDown:
			if n > 0 && a.model.CommandsView.Selected < n-1 {
				a.model.CommandsView.Selected++
			}
			if beforeSel != a.model.CommandsView.Selected {
				a.model.CommandsView.StdoutScroll = 0
				a.model.CommandsView.StderrScroll = 0
			}
			a.ensureCommandsViewSelectionVisible()
		case tcell.KeyPgUp:
			a.model.CommandsView.Selected = max(0, a.model.CommandsView.Selected-5)
			if beforeSel != a.model.CommandsView.Selected {
				a.model.CommandsView.StdoutScroll = 0
				a.model.CommandsView.StderrScroll = 0
			}
			a.ensureCommandsViewSelectionVisible()
		case tcell.KeyPgDn:
			if n > 0 {
				a.model.CommandsView.Selected = min(n-1, a.model.CommandsView.Selected+5)
			}
			if beforeSel != a.model.CommandsView.Selected {
				a.model.CommandsView.StdoutScroll = 0
				a.model.CommandsView.StderrScroll = 0
			}
			a.ensureCommandsViewSelectionVisible()
		case tcell.KeyHome:
			a.model.CommandsView.Selected = 0
			if beforeSel != a.model.CommandsView.Selected {
				a.model.CommandsView.StdoutScroll = 0
				a.model.CommandsView.StderrScroll = 0
			}
			a.ensureCommandsViewSelectionVisible()
		case tcell.KeyEnd:
			if n > 0 {
				a.model.CommandsView.Selected = n - 1
			}
			if beforeSel != a.model.CommandsView.Selected {
				a.model.CommandsView.StdoutScroll = 0
				a.model.CommandsView.StderrScroll = 0
			}
			a.ensureCommandsViewSelectionVisible()
		}
	case 1:
		maxS := a.maxCommandsStdoutScroll()
		switch event.Key() {
		case tcell.KeyUp:
			if a.model.CommandsView.StdoutScroll > 0 {
				a.model.CommandsView.StdoutScroll--
			}
		case tcell.KeyDown:
			if a.model.CommandsView.StdoutScroll < maxS {
				a.model.CommandsView.StdoutScroll++
			}
		case tcell.KeyPgUp:
			a.model.CommandsView.StdoutScroll = max(0, a.model.CommandsView.StdoutScroll-5)
		case tcell.KeyPgDn:
			a.model.CommandsView.StdoutScroll = min(maxS, a.model.CommandsView.StdoutScroll+5)
		}
	case 2:
		maxS := a.maxCommandsStderrScroll()
		switch event.Key() {
		case tcell.KeyUp:
			if a.model.CommandsView.StderrScroll > 0 {
				a.model.CommandsView.StderrScroll--
			}
		case tcell.KeyDown:
			if a.model.CommandsView.StderrScroll < maxS {
				a.model.CommandsView.StderrScroll++
			}
		case tcell.KeyPgUp:
			a.model.CommandsView.StderrScroll = max(0, a.model.CommandsView.StderrScroll-5)
		case tcell.KeyPgDn:
			a.model.CommandsView.StderrScroll = min(maxS, a.model.CommandsView.StderrScroll+5)
		}
	}
	return false
}

func (a *App) clampCommandsFocusPane() {
	maxP := a.commandsViewFocusPaneCount() - 1
	if a.model.CommandsView.FocusPane > maxP {
		a.model.CommandsView.FocusPane = maxP
	}
}

func (a *App) commandsViewFocusPaneCount() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 2
	}
	_, stderrRect := ui.SplitJobsRightColumnFlexTop(layout.Right, 8)
	if stderrRect.Height == 0 {
		return 2
	}
	return 3
}

func (a *App) selectedCommandEntry() ui.CommandRunEntry {
	a.commandsMu.RLock()
	defer a.commandsMu.RUnlock()
	if a.model.CommandsView.Selected >= 0 && a.model.CommandsView.Selected < len(a.model.CommandsList) {
		return a.model.CommandsList[a.model.CommandsView.Selected]
	}
	return ui.CommandRunEntry{}
}

func commandOutputLineCount(text string) int {
	t := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(t) == "" {
		return 1
	}
	return len(strings.Split(t, "\n"))
}

func (a *App) maxCommandsStdoutScroll() int {
	sel := a.selectedCommandEntry()
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	stdoutRect, _ := ui.SplitJobsRightColumnFlexTop(layout.Right, 8)
	contentH := ui.JobsPanelContentRows(stdoutRect)
	lines := commandOutputLineCount(sel.Stdout)
	return max(0, lines-contentH)
}

func (a *App) maxCommandsStderrScroll() int {
	sel := a.selectedCommandEntry()
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	_, stderrRect := ui.SplitJobsRightColumnFlexTop(layout.Right, 8)
	contentH := ui.JobsPanelContentRows(stderrRect)
	lines := commandOutputLineCount(sel.Stderr)
	return max(0, lines-contentH)
}
