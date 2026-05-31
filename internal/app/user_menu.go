package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

var userMenuInteractiveRunner = func(ctx context.Context, argv []string, dir string) error {
	return cmdrun.RunInteractive(ctx, argv, dir)
}

var userMenuDetachRunner = func(argv []string, dir string) error {
	return cmdrun.StartDetached(argv, dir)
}

func (a *App) userMenuConfigDir() string {
	return strings.TrimSpace(a.paths.ConfigDir)
}

func (a *App) resolveUserMenuContext() (menuPath string, warns []string) {
	return usermenu.ResolveMenuTOML(a.config, a.model.UserHomeDir, a.userMenuConfigDir(), a.activePanel().PathString())
}

func (a *App) ensureGlobalUserMenuStub() (path string, err error) {
	path = usermenu.ResolveUserMenuGlobalPath(a.config, a.model.UserHomeDir, a.userMenuConfigDir())
	if path == "" {
		return "", fmt.Errorf("user menu: no global menu path configured")
	}
	if _, err := usermenu.WriteMenuStub(path); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) setUserMenuCritical(err error) {
	if err == nil {
		return
	}
	a.setTransientMessage(usermenu.ShortLoadError(err), ui.MessageUrgencyCritical)
}

func (a *App) openUserMenuEditor(path string) {
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	a.setTransientMessage("User menu: edited "+path, ui.MessageUrgencyInfo)
	a.render()
}

func (a *App) editUserMenu() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	path, err := a.resolveUserMenuEditPath()
	if err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	a.openUserMenuEditor(path)
}

func (a *App) openUserMenu() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	menuPath, warns := a.resolveUserMenuContext()
	for _, w := range warns {
		a.setTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if menuPath == "" {
		path, err := a.ensureGlobalUserMenuStub()
		if err != nil {
			a.setErrorMessage("User menu", err)
			return
		}
		a.openUserMenuEditor(path)
		return
	}

	mf, err := usermenu.LoadFile(menuPath)
	if err != nil {
		a.setUserMenuCritical(err)
		return
	}
	if len(mf.Entries) == 0 {
		a.setTransientMessage("User menu: no entries (edit with Shift+F2)", ui.MessageUrgencyWarn)
		return
	}

	active := a.panelByID(a.model.ActivePanel)
	other := a.panelByID(a.inactivePanelID())
	ctx := &usermenu.EvalContext{Active: active, Other: other}
	visible, defIdx, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setUserMenuCritical(err)
		return
	}
	if len(visible) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		return
	}
	a.model.UserMenu = ui.UserMenuDialogState{
		Open:         true,
		Title:        "User menu",
		Entries:      visible,
		Selected:     defIdx,
		Focus:        defIdx,
		ScrollOffset: 0,
		SourcePath:   menuPath,
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.UserMenuListViewportRows(layout, len(visible))
	dialog.UserMenuEnsureScroll(&a.model.UserMenu, vr)
	a.clearTransientMessage()
}

func (a *App) closeUserMenu() {
	a.model.UserMenu = ui.UserMenuDialogState{}
}

func (a *App) handleUserMenuDialogKey(event *tcell.EventKey) {
	st := &a.model.UserMenu
	n := len(st.Entries)
	if n == 0 {
		a.closeUserMenu()
		return
	}
	form := dialog.NewUserMenuDialogForm(n)
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.UserMenuListViewportRows(layout, n)

	if ui.AltDialogCancel(event) {
		a.closeUserMenu()
		return
	}

	switch event.Key() {
	case tcell.KeyF4:
		a.editUserMenuConfigFromDialog()
		return
	case tcell.KeyEsc:
		a.closeUserMenu()
	case tcell.KeyEnter:
		if st.Focus == form.CancelIndex() {
			a.closeUserMenu()
			return
		}
		if st.Focus >= 0 && st.Focus < n {
			a.executeUserMenuEntry(st.Focus)
			return
		}
	case tcell.KeyRune:
		if keymap.AltLetterModifiers(event.Modifiers()) {
			if i, ok := ui.UserMenuEntryIndexForAltShortcut(st.Entries, event.Rune()); ok {
				a.executeUserMenuEntry(i)
				return
			}
		}
	}
	if focus, ok := form.MoveFocus(st.Focus, event.Key()); ok {
		st.Focus = focus
		if st.Focus >= 0 && st.Focus < n {
			st.Selected = st.Focus
		}
		dialog.UserMenuEnsureScroll(st, vr)
	}
}

func (a *App) executeUserMenuEntry(idx int) {
	st := a.model.UserMenu
	if !st.Open || idx < 0 || idx >= len(st.Entries) {
		a.closeUserMenu()
		return
	}
	entry := st.Entries[idx]
	a.closeUserMenu()

	active := a.activePanel()
	other := a.inactivePanel()
	expanded, err := usermenu.ExpandCommand(entry.Command, active, other)
	if err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	argv, err := cmdrun.ParseCommandArgv(expanded)
	if err != nil {
		a.openMessageDialog("User menu", err.Error())
		return
	}
	if len(argv) == 0 {
		a.setTransientMessage("User menu: command is empty", ui.MessageUrgencyWarn)
		return
	}

	workDir := active.PathString()
	switch {
	case entry.Interactive:
		a.runUserMenuInteractive(argv, workDir)
	case entry.Detach:
		a.runUserMenuDetached(argv, workDir)
	default:
		cmdLine := entry.Command
		rowIdx := a.appendUserMenuCommandRow(cmdLine, expanded)
		if !entry.Background {
			a.openCommandsView()
			a.model.CommandsView.Selected = rowIdx
			a.model.CommandsView.FocusPane = 0
			a.ensureCommandsViewSelectionVisible()
		}

		a.commandsBatchesInflight.Add(1)
		go a.runUserMenuCommand(a.commandsCtx, rowIdx, argv, workDir, entry.Background, entry.Title, entry.Pool)
	}
}

func (a *App) runUserMenuInteractive(argv []string, workDir string) {
	if err := a.withTerminalReleased(func() error {
		return userMenuInteractiveRunner(context.Background(), argv, workDir)
	}); err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	a.refreshAfterUserMenuCommand()
}

func (a *App) runUserMenuDetached(argv []string, workDir string) {
	if err := userMenuDetachRunner(argv, workDir); err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	a.setTransientMessage("Started "+cmdrun.FormatArgvDisplay(argv), ui.MessageUrgencyInfo)
}

func (a *App) refreshAfterUserMenuCommand() {
	if a.model.ViewMode == ui.ViewBrowser {
		if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("User menu", err)
		}
	}
}

func (a *App) appendUserMenuCommandRow(cmdLine, expanded string) int {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	idx := len(a.model.CommandsList)
	a.model.CommandsList = append(a.model.CommandsList, ui.CommandRunEntry{
		ID:              cmdrun.NewRunID(),
		Kind:            ui.CommandRunKindUserMenu,
		UserCommandLine: cmdLine + " → " + expanded,
		TargetPath:      "",
		Phase:           ui.CommandRunPending,
		ExitCode:        -1,
	})
	return idx
}

func (a *App) runUserMenuCommand(ctx context.Context, idx int, argv []string, workDir string, background bool, title, poolName string) {
	defer a.commandsBatchesInflight.Add(-1)

	postBackgroundFinal := func(res cmdrun.RunResult) {
		if !background {
			a.postCommandWake()
			return
		}
		p := commandWakePayload{refreshBrowserPanel: true}
		if log, banner, urg, ok := userMenuBackgroundNotify(title, res); ok {
			p.notifyLog = log
			p.notifyBanner = banner
			p.notifyUrg = urg
		}
		a.postCommandWakePayload(p)
	}
	markCanceled := func() {
		a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunDone
			e.ExitCode = -1
			if e.ErrorMsg == "" {
				e.ErrorMsg = "Canceled"
			}
		})
		if background {
			a.postCommandWakePayload(commandWakePayload{refreshBrowserPanel: true})
		} else {
			a.postCommandWake()
		}
	}

	select {
	case <-ctx.Done():
		markCanceled()
		return
	default:
	}

	var release func()
	if strings.TrimSpace(poolName) != "" {
		var err error
		release, err = a.workPools.Acquire(ctx, poolName)
		if err != nil {
			a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
				e.Phase = ui.CommandRunDone
				e.ExitCode = -1
				if ctx.Err() != nil {
					if e.ErrorMsg == "" {
						e.ErrorMsg = "Canceled"
					}
				} else {
					e.ErrorMsg = err.Error()
				}
			})
			postBackgroundFinal(cmdrun.RunResult{})
			return
		}
		defer release()
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
	postBackgroundFinal(res)
}

// userMenuBackgroundNotify returns status text when a background user-menu run should alert the user.
func userMenuBackgroundNotify(title string, res cmdrun.RunResult) (log, banner string, urg ui.MessageUrgency, ok bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "command"
	}
	prefix := "User menu: " + title

	stderrText := strings.TrimSpace(string(res.Stderr))
	hasStderr := stderrText != ""

	switch {
	case res.LaunchErr != nil:
		detail := res.LaunchErr.Error()
		log = prefix + ": " + detail
		banner = prefix + ": " + truncateStatusBannerRunes(firstMessageLine(detail), jobFailureBannerMaxRunes)
		return log, banner, ui.MessageUrgencyError, true
	case res.ExitCode != 0:
		if hasStderr {
			line := firstMessageLine(stderrText)
			log = prefix + " (exit " + fmt.Sprint(res.ExitCode) + "): " + stderrText
			banner = prefix + ": " + truncateStatusBannerRunes(line, jobFailureBannerMaxRunes)
		} else {
			log = prefix + ": exit " + fmt.Sprint(res.ExitCode)
			banner = log
		}
		return log, banner, ui.MessageUrgencyError, true
	case hasStderr:
		line := firstMessageLine(stderrText)
		log = prefix + ": " + stderrText
		banner = prefix + ": " + truncateStatusBannerRunes(line, jobFailureBannerMaxRunes)
		return log, banner, ui.MessageUrgencyWarn, true
	default:
		return "", "", ui.MessageUrgencyInfo, false
	}
}
