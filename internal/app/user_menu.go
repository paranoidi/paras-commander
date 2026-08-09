package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func (a *App) openUserMenuEditor(path string) bool {
	changed, err := usermenu.RefreshDocumentation(path)
	if err != nil {
		a.setErrorMessage("User menu", err)
		return false
	}
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("User menu", err)
		return false
	}
	if changed {
		a.setTransientMessage("User menu: updated documentation in "+path, ui.MessageUrgencyInfo)
	} else {
		a.setTransientMessage("User menu: edited "+path, ui.MessageUrgencyInfo)
	}
	a.render()
	return true
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
	if err := mf.ValidatePoolRefs(usermenu.PoolNameSet(a.workPools.Names())); err != nil {
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
	visible, _, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setUserMenuCritical(err)
		return
	}
	if len(visible) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		return
	}
	a.userMenuPath = menuPath
	a.userMenuStack = nil
	a.openUserMenuLevel(visible)
}

// openUserMenuLevel opens (or swaps the already-open strip to) one menu level: entries is
// the flat list of rows to show now, either the top-level menu or a submenu's children.
// Picking a submenu row pushes the current level onto a.userMenuStack and recurses into it;
// picking a leaf clears the stack (leaving the whole menu, not just one level) and runs it.
func (a *App) openUserMenuLevel(entries []usermenu.MenuEntry) {
	a.userMenuVisible = entries
	items := userMenuLeaderMenuItems(entries, a.styles)
	a.openLeaderMenuStrip(items, true, false, "User menu", func(i int) bool {
		if i < 0 || i >= len(a.userMenuVisible) {
			return false
		}
		entry := a.userMenuVisible[i]
		if entry.IsSubmenu() {
			a.userMenuStack = append(a.userMenuStack, a.userMenuVisible)
			a.openUserMenuLevel(entry.Entries)
			return false
		}
		a.userMenuStack = nil
		a.runUserMenuEntry(entry)
		return false
	})
}

// userMenuLeaderMenuItems maps visible user-menu entries to leader-menu rows. Submenu rows
// get the tree-expand glyph appended to their label so they're visually distinguishable
// from runnable ones.
func userMenuLeaderMenuItems(entries []usermenu.MenuEntry, styles theme.Theme) []ui.LeaderMenuItem {
	items := make([]ui.LeaderMenuItem, len(entries))
	for i, e := range entries {
		label := e.Title
		if e.IsSubmenu() {
			label += " " + string(styles.SymbolTreeExpand())
		}
		items[i] = ui.LeaderMenuItem{Key: dialog.ConfiguredKeyRune(e.Key), Label: label}
	}
	return items
}

// runUserMenuEntry executes a resolved F2 user-menu entry (dialog already closed).
func (a *App) runUserMenuEntry(entry usermenu.MenuEntry) {
	active := a.activePanel()
	other := a.inactivePanel()
	if len(entry.RunForEach) > 0 {
		a.executeUserMenuRunForEach(entry, active, other)
		return
	}

	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template:   entry.Command,
		Mode:       cmdrun.ModeAuto,
		ForceShell: entry.Shell,
		Ctx:        usermenu.MacroContext(active, other, "", ""),
	})
	if err != nil {
		if strings.Contains(err.Error(), "cmdmacro:") || strings.Contains(err.Error(), "user menu:") {
			a.setErrorMessage("User menu", err)
			return
		}
		a.openMessageDialog("User menu", err.Error())
		return
	}
	argv := built.Argv
	expanded := built.Expanded

	workDir := active.PathString()
	switch {
	case entry.Dialog:
		a.commandsCtrl.BeginBatch()
		go a.commandsCtrl.RunUserMenuCommandDialog(a.commandsCtrl.Context(), argv, workDir, entry.Title, entry.DialogWidth, entry.DialogHeight)
	case entry.Interactive:
		a.runUserMenuInteractive(argv, workDir, entry.Toast)
	case entry.Detach:
		a.runUserMenuDetached(argv, workDir, entry.Toast)
	default:
		cmdLine := entry.Command
		rowIdx := a.appendUserMenuCommandRow(cmdLine, expanded)
		if !entry.Background {
			a.commandsCtrl.OpenViewAt(rowIdx)
		}

		a.commandsCtrl.BeginBatch()
		go a.runUserMenuCommand(a.commandsCtrl.Context(), rowIdx, argv, workDir, entry.Background, entry.Title, entry.Pool, entry.Toast)
	}
}

func (a *App) executeUserMenuRunForEach(entry usermenu.MenuEntry, active, other *panel.State) {
	src, err := ops.ResolveSource(active)
	if err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	if len(src.Entries) == 0 {
		a.setTransientMessage("User menu: no paths to run", ui.MessageUrgencyWarn)
		return
	}

	var allowFiles, allowDirs bool
	for _, v := range entry.RunForEach {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "files":
			allowFiles = true
		case "dirs":
			allowDirs = true
		}
	}

	cmdTemplate := entry.Command
	workDir := active.PathString()
	notifyLabel := "User menu: " + strings.TrimSpace(entry.Title)
	if notifyLabel == "User menu:" {
		notifyLabel = "User menu"
	}
	a.commandsCtrl.StartRunForEachBatch(commandsctrl.RunForEachBatchSpec{
		Kind:        ui.CommandRunKindRunForEach,
		Entries:     append([]localfs.Entry(nil), src.Entries...),
		AllowFiles:  allowFiles,
		AllowDirs:   allowDirs,
		WorkDir:     workDir,
		PoolName:    strings.TrimSpace(entry.Pool),
		Background:  entry.Background,
		NotifyLabel: notifyLabel,
		BuildItem: func(ent localfs.Entry) (commandsctrl.RunForEachBuiltItem, error) {
			return commandsctrl.BuildRunForEachItem(cmdTemplate, ent, active, other, entry.Shell, true)
		},
	})
}

func (a *App) runUserMenuInteractive(argv []string, workDir, toast string) {
	if err := a.withTerminalReleased(func() error {
		return userMenuInteractiveRunner(context.Background(), argv, workDir)
	}); err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	a.activePanel().ClearSelection()
	a.refreshAfterUserMenuCommand()
	if toast != "" {
		a.setTransientMessage(toast, ui.MessageUrgencyInfo)
	}
}

func (a *App) runUserMenuDetached(argv []string, workDir, toast string) {
	if err := userMenuDetachRunner(argv, workDir); err != nil {
		a.setErrorMessage("User menu", err)
		return
	}
	if toast != "" {
		a.setTransientMessage(toast, ui.MessageUrgencyInfo)
	} else {
		a.setTransientMessage("Started "+cmdrun.FormatArgvDisplay(argv), ui.MessageUrgencyInfo)
	}
}

func (a *App) refreshAfterUserMenuCommand() {
	if a.model.ViewMode == ui.ViewBrowser {
		if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("User menu", err)
		}
	}
}

func (a *App) appendUserMenuCommandRow(cmdLine, expanded string) int {
	return a.commandsCtrl.AppendEntry(ui.CommandRunEntry{
		ID:              cmdrun.NewRunID(),
		Kind:            ui.CommandRunKindUserMenu,
		UserCommandLine: cmdLine + " → " + expanded,
		TargetPath:      "",
		Phase:           ui.CommandRunPending,
		ExitCode:        -1,
	})
}

func (a *App) runUserMenuCommand(ctx context.Context, idx int, argv []string, workDir string, background bool, title, poolName, toast string) {
	defer a.commandsCtrl.EndBatch()

	postBackgroundFinal := func(res cmdrun.RunResult) {
		if !background {
			a.commandsCtrl.PostWake(commandsctrl.WakePayload{ClearActiveSelection: true})
			return
		}
		p := commandsctrl.WakePayload{RefreshBrowserPanel: true, ClearActiveSelection: true}
		if log, banner, urg, ok := userMenuBackgroundNotify(title, res); ok {
			p.NotifyLog = log
			p.NotifyBanner = banner
			p.NotifyUrg = urg
		} else if toast != "" && res.LaunchErr == nil && res.ExitCode == 0 {
			p.NotifyLog = toast
			p.NotifyBanner = toast
			p.NotifyUrg = ui.MessageUrgencyInfo
		}
		a.commandsCtrl.PostWake(p)
	}
	markCanceled := func() {
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
			a.commandsCtrl.PatchEntry(idx, func(e *ui.CommandRunEntry) {
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
		banner = prefix + ": " + textutil.TruncateBannerRunes(textutil.FirstLine(detail), textutil.BannerMaxRunes)
		return log, banner, ui.MessageUrgencyError, true
	case res.ExitCode != 0:
		if hasStderr {
			line := textutil.FirstLine(stderrText)
			log = prefix + " (exit " + fmt.Sprint(res.ExitCode) + "): " + stderrText
			banner = prefix + ": " + textutil.TruncateBannerRunes(line, textutil.BannerMaxRunes)
		} else {
			log = prefix + ": exit " + fmt.Sprint(res.ExitCode)
			banner = log
		}
		return log, banner, ui.MessageUrgencyError, true
	case hasStderr:
		line := textutil.FirstLine(stderrText)
		log = prefix + ": " + stderrText
		banner = prefix + ": " + textutil.TruncateBannerRunes(line, textutil.BannerMaxRunes)
		return log, banner, ui.MessageUrgencyWarn, true
	default:
		return "", "", ui.MessageUrgencyInfo, false
	}
}
