package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
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

func (a *App) toggleUserMenu() {
	if a.leaderMenuOpen(ui.LeaderMenuUser) {
		a.closeUserMenu()
		return
	}
	a.openUserMenu()
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
		if len(warns) > 0 {
			// A menu.toml exists but was skipped (untrusted): leave the warning up rather than
			// opening the editor on a file that would still be ignored afterwards.
			return
		}
		path, err := a.ensureGlobalUserMenuStub()
		if err != nil {
			a.setErrorMessage("User menu", err)
			return
		}
		a.openUserMenuEditor(path)
		return
	}

	visible, warnings, ok := a.loadUserMenuVisible(menuPath)
	if !ok {
		return
	}
	a.userMenuPath = menuPath
	a.userMenuWarnings = warnings
	a.userMenuStack = nil
	a.openUserMenuLevel(visible)
}

// loadUserMenuVisible loads menuPath, validates it, and resolves the entries visible for the
// current panel context — the shared block behind both openUserMenu and reloadLeaderMenu.
// On any failure it emits the appropriate message itself (setUserMenuCritical/
// setTransientMessage) and returns ok=false; the caller only needs to decide what to do next
// (return, or close the strip). warnings is mf.Warnings (e.g. a duplicate key= at one level).
func (a *App) loadUserMenuVisible(menuPath string) (visible []usermenu.MenuEntry, warnings []string, ok bool) {
	mf, err := usermenu.LoadFile(menuPath)
	if err != nil {
		a.setUserMenuCritical(err)
		return nil, nil, false
	}
	if err := mf.ValidatePoolRefs(usermenu.PoolNameSet(a.workPools.Names())); err != nil {
		a.setUserMenuCritical(err)
		return nil, nil, false
	}
	if len(mf.Entries) == 0 {
		a.setTransientMessage("User menu: no entries (edit with Shift+F2)", ui.MessageUrgencyWarn)
		return nil, nil, false
	}

	active := a.panelByID(a.model.ActivePanel)
	other := a.panelByID(a.inactivePanelID())
	ctx := &usermenu.EvalContext{Active: active, Other: other}
	v, _, err := usermenu.FilterVisible(mf, ctx)
	if err != nil {
		a.setUserMenuCritical(err)
		return nil, nil, false
	}
	if len(v) == 0 {
		a.setTransientMessage("User menu: no visible entries", ui.MessageUrgencyWarn)
		return nil, nil, false
	}
	return v, mf.Warnings, true
}

// openUserMenuLevel opens (or swaps the already-open strip to) one menu level: entries is
// the flat list of rows to show now, either the top-level menu or a submenu's children. This
// is the single place any user-menu level is shown (top level, submenu, back, reload), so
// it is also the single place a.userMenuWarnings is toasted. Picking a submenu row pushes the
// current level onto a.userMenuStack and recurses into it (usermenu.back pops it); picking a leaf clears the stack
// (leaving the whole menu, not just one level) and runs it.
func (a *App) openUserMenuLevel(entries []usermenu.MenuEntry) {
	a.userMenuVisible = entries
	items := userMenuLeaderMenuItems(entries, a.styles)
	opened := a.openLeaderMenuStrip(items, ui.LeaderMenuUser, "User menu", func(i int) bool {
		if i < 0 || i >= len(a.userMenuVisible) {
			return false
		}
		entry := a.userMenuVisible[i]
		if entry.IsSubmenu() {
			a.userMenuStack = append(a.userMenuStack, a.userMenuVisible)
			a.openUserMenuLevel(entry.Entries)
			return false
		}
		if entry.KeepOpen {
			// Reopen before running: the strip clears the transient message, which would eat the toast.
			level := a.userMenuVisible
			a.openUserMenuLevel(level)
			a.runUserMenuEntry(entry)
			return false
		}
		a.userMenuStack = nil
		a.runUserMenuEntry(entry)
		return false
	})
	if !opened {
		return
	}
	// openLeaderMenuStrip clears the transient message internally, so the warning toast
	// must be set after it returns.
	for _, w := range a.userMenuWarnings {
		a.setTransientMessage(w, ui.MessageUrgencyWarn)
	}
}

// userMenuLeaderMenuItems maps visible user-menu entries to leader-menu rows. Submenu rows
// get the tree-expand icon appended to their label so they're visually distinguishable
// from runnable ones.
func userMenuLeaderMenuItems(entries []usermenu.MenuEntry, styles theme.Theme) []ui.LeaderMenuItem {
	items := make([]ui.LeaderMenuItem, len(entries))
	for i, e := range entries {
		label := e.Title
		if e.IsSubmenu() {
			label += " " + string(styles.IconTreeExpand())
		}
		items[i] = ui.LeaderMenuItem{Key: dialog.ConfiguredKeyRune(e.Key), Label: label}
	}
	return items
}

// runUserMenuEntry executes a resolved F2 user-menu entry (dialog already closed).
func (a *App) runUserMenuEntry(entry usermenu.MenuEntry) {
	active := a.activePanel()
	other := a.inactivePanel()
	if entry.KeepOpen && entry.Toast == "" {
		prefix := "Ran"
		if a.styles.UseNerdfontIcons {
			prefix = a.styles.IconJobsList("completed")
		}
		entry.Toast = prefix + " " + backgroundRunTitle(entry.Title)
	}
	if len(entry.RunForEach) > 0 {
		if a.executeUserMenuRunForEach(entry, active, other) && entry.KeepOpen {
			a.setTransientMessage(entry.Toast, ui.MessageUrgencyInfo)
		}
		return
	}

	macroCtx := usermenu.MacroContext(active, other, "", "")
	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template:   entry.Command,
		Mode:       cmdrun.ModeAuto,
		ForceShell: entry.Shell,
		Ctx:        macroCtx,
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
	macros := cmdmacro.MacroDisplayValues(entry.Command, macroCtx)

	workDir := active.PathString()
	switch {
	case entry.Dialog:
		clearConsumedSelection(entry.Command, active, other)
		rowIdx, rowID := a.appendUserMenuCommandRow(entry.Command, macros)
		ctx, cancel := context.WithCancel(a.commandsCtrl.Context())
		a.commandsCtrl.OpenRunningOutputDialog(rowID, entry.Title, strings.TrimSpace(entry.Pool) != "", cancel, entry.DialogWidth, entry.DialogHeight)
		a.commandsCtrl.BeginBatch()
		go func() {
			defer cancel()
			a.runUserMenuCommand(ctx, rowIdx, rowID, argv, workDir, entry)
		}()
	case entry.Interactive:
		if a.runUserMenuInteractive(argv, workDir, entry.Toast) {
			clearConsumedSelection(entry.Command, active, other)
		}
	case entry.Detach:
		if a.runUserMenuDetached(argv, workDir, entry.Toast) {
			clearConsumedSelection(entry.Command, active, other)
		}
	default:
		clearConsumedSelection(entry.Command, active, other)
		rowIdx, rowID := a.appendUserMenuCommandRow(entry.Command, macros)
		if !entry.Background {
			a.commandsCtrl.OpenViewAt(rowIdx)
		}

		a.commandsCtrl.BeginBatch()
		go a.runUserMenuCommand(a.commandsCtrl.Context(), rowIdx, rowID, argv, workDir, entry)
	}
}

// clearConsumedSelection clears the tags the command read: %t uses the active
// panel's, %T the other panel's.
func clearConsumedSelection(cmd string, active, other *panel.State) {
	if cmdmacro.CommandRequiresMacro(cmd, 't') {
		active.ClearSelection()
	}
	if cmdmacro.CommandRequiresMacro(cmd, 'T') {
		other.ClearSelection()
	}
}

// executeUserMenuRunForEach reports whether the batch was started.
func (a *App) executeUserMenuRunForEach(entry usermenu.MenuEntry, active, other *panel.State) bool {
	src, err := ops.ResolveSource(active)
	if err != nil {
		a.setErrorMessage("User menu", err)
		return false
	}
	if len(src.Entries) == 0 {
		a.setTransientMessage("User menu: no paths to run", ui.MessageUrgencyWarn)
		return false
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
		PTY:         entry.PTY,
		NotifyLabel: notifyLabel,
		BuildItem: func(ent localfs.Entry) (commandsctrl.RunForEachBuiltItem, error) {
			return commandsctrl.BuildRunForEachItem(cmdTemplate, ent, active, other, entry.Shell, true)
		},
	})
	return true
}

// runUserMenuInteractive reports whether the command ran without error.
func (a *App) runUserMenuInteractive(argv []string, workDir, toast string) bool {
	if err := a.withTerminalReleased(func() error {
		return userMenuInteractiveRunner(context.Background(), argv, workDir)
	}); err != nil {
		a.setErrorMessage("User menu", err)
		return false
	}
	a.refreshAfterBackgroundCommand()
	if toast != "" {
		a.setTransientMessage(toast, ui.MessageUrgencyInfo)
	}
	return true
}

// runUserMenuDetached reports whether the process started.
func (a *App) runUserMenuDetached(argv []string, workDir, toast string) bool {
	if err := userMenuDetachRunner(argv, workDir); err != nil {
		a.setErrorMessage("User menu", err)
		return false
	}
	if toast != "" {
		a.setTransientMessage(toast, ui.MessageUrgencyInfo)
	} else {
		a.setTransientMessage("Started "+cmdrun.FormatArgvDisplay(argv), ui.MessageUrgencyInfo)
	}
	return true
}

func (a *App) refreshAfterBackgroundCommand() {
	if a.model.ViewMode == ui.ViewBrowser {
		if err := a.activePanel().Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("User menu", err)
		}
	}
}

func (a *App) appendUserMenuCommandRow(cmdLine string, macros []cmdmacro.MacroUse) (int, string) {
	id := cmdrun.NewRunID()
	return a.commandsCtrl.AppendEntry(ui.CommandRunEntry{
		ID:              id,
		Kind:            ui.CommandRunKindUserMenu,
		UserCommandLine: cmdLine,
		Macros:          macros,
		Phase:           ui.CommandRunPending,
		ExitCode:        -1,
	}), id
}

// runUserMenuCommand runs a captured user-menu command recorded in Commands row idx/rowID.
// With entry.Dialog, ctx cancellation means the user pressed Cancel in the output dialog.
func (a *App) runUserMenuCommand(ctx context.Context, idx int, rowID string, argv []string, workDir string, entry usermenu.MenuEntry) {
	defer a.commandsCtrl.EndBatch()
	background, title, toast := entry.Background, entry.Title, entry.Toast

	postBackgroundFinal := func(res cmdrun.RunResult) {
		if !background && !entry.Dialog {
			a.commandsCtrl.PostRenderWake()
			return
		}
		p := commandsctrl.WakePayload{RefreshBrowserPanel: true}
		if entry.Dialog {
			// Applied only if the dialog still shows this run; otherwise it is backgrounded.
			out := commandsctrl.OutputDialogResult(rowID, title, res, entry.DialogWidth, entry.DialogHeight, workDir)
			p.OpenOutputDialog = &out
		}
		if log, banner, urg, ok := backgroundRunNotify("User menu: "+backgroundRunTitle(title), res); ok {
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
		if background && !entry.Dialog {
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
	if strings.TrimSpace(entry.Pool) != "" {
		var err error
		release, err = a.workPools.Acquire(ctx, entry.Pool)
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
			if ctx.Err() != nil && entry.Dialog {
				a.commandsCtrl.PostRenderWake()
				return
			}
			postBackgroundFinal(cmdrun.RunResult{LaunchErr: err})
			return
		}
		defer release()
	}

	a.commandsCtrl.PatchEntry(idx, func(e *ui.CommandRunEntry) {
		e.Phase = ui.CommandRunRunning
	})
	a.commandsCtrl.PostWake(commandsctrl.WakePayload{OutputDialogStartedRunID: rowID})

	live := &cmdrun.Live{}
	stopLive := a.commandsCtrl.StreamLive(ctx, idx, rowID, live)
	res := cmdrun.RunTracked(ctx, argv, a.commandsCtrl.ChildEnv(), workDir, cmdrun.MaxStreamBytes, func(p *os.Process) {
		a.commandsCtrl.SetProcess(idx, p)
	}, live)
	stopLive()
	a.commandsCtrl.UnregisterProc(idx)
	if entry.Dialog && ctx.Err() != nil {
		markCanceled()
		return
	}
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

// backgroundRunTitle defaults an empty user-menu entry title to "command" for notify text.
func backgroundRunTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "command"
	}
	return title
}

// backgroundRunNotify returns status text when a backgrounded run (user-menu or file-execute)
// should alert the user: prefix identifies the run (e.g. "User menu: <title>" or "Run: <cmdLine>").
func backgroundRunNotify(prefix string, res cmdrun.RunResult) (log, banner string, urg ui.MessageUrgency, ok bool) {
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
