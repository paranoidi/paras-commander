package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

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
	a.openUserMenuEditor(menuPath)
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
		a.setErrorMessage("User menu", err)
		return
	}
	if len(mf.Entries) == 0 {
		a.openUserMenuEditor(menuPath)
		return
	}

	active := a.panelByID(a.model.ActivePanel)
	other := a.panelByID(a.inactivePanelID())
	ctx := &usermenu.EvalContext{Active: active, Other: other}
	visible, defIdx, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	if len(visible) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		return
	}
	a.model.UserMenu = ui.UserMenuDialogState{
		Open:         true,
		Title:        "User menu — " + menuPath,
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
	form := ui.NewDialogLinearForm(n)
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.UserMenuListViewportRows(layout, n)

	if ui.AltDialogOK(event) {
		a.executeUserMenuSelection()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeUserMenu()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeUserMenu()
	case tcell.KeyEnter:
		switch st.Focus {
		case form.CancelIndex():
			a.closeUserMenu()
		case form.OKIndex():
			a.executeUserMenuSelection()
		default:
			if st.Focus >= 0 && st.Focus < n {
				st.Selected = st.Focus
				a.executeUserMenuSelection()
			}
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'o', 'O':
			a.executeUserMenuSelection()
			return
		case 'c', 'C':
			a.closeUserMenu()
			return
		case ' ':
			switch {
			case st.Focus >= 0 && st.Focus < n:
				st.Selected = st.Focus
			case st.Focus == form.OKIndex():
				a.executeUserMenuSelection()
			case st.Focus == form.CancelIndex():
				a.closeUserMenu()
			}
			return
		default:
			for i := range st.Entries {
				k := []rune(st.Entries[i].Key)
				if len(k) == 0 {
					continue
				}
				if k[0] == event.Rune() || (k[0] >= 'A' && k[0] <= 'Z' && k[0]+32 == event.Rune()) ||
					(k[0] >= 'a' && k[0] <= 'z' && k[0]-32 == event.Rune()) {
					st.Selected = i
					st.Focus = i
					dialog.UserMenuEnsureScroll(st, vr)
					return
				}
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

func (a *App) executeUserMenuSelection() {
	st := a.model.UserMenu
	if !st.Open || st.Selected < 0 || st.Selected >= len(st.Entries) {
		a.closeUserMenu()
		return
	}
	entry := st.Entries[st.Selected]
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

	cmdLine := entry.Command
	idx := a.appendUserMenuCommandRow(cmdLine, expanded)
	a.openCommandsView()
	a.model.CommandsView.Selected = idx
	a.model.CommandsView.FocusPane = 0
	a.ensureCommandsViewSelectionVisible()

	a.commandsBatchesInflight.Add(1)
	go a.runUserMenuCommand(a.commandsCtx, idx, argv, active.PathString())
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

func (a *App) runUserMenuCommand(ctx context.Context, idx int, argv []string, workDir string) {
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
