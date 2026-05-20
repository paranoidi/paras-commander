package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) closeFilePreviewFullscreen() {
	a.commandsMu.Lock()
	a.model.FullscreenFilePreview = ui.FilePreviewState{}
	a.commandsMu.Unlock()
	a.model.ViewMode = ui.ViewBrowser
	a.model.MenuDefinitions = a.browserMenuDefinitions()
	a.model.Menu.ActiveMenu = menu.DefaultIndex()
}

func (a *App) fullscreenFilePreviewScrollMetrics() (textW, contentH, lineCount int) {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return 1, 0, 0
	}
	union := ui.MergeTwinPanelRects(lay.Left, lay.Right)
	tw := union.Width - 4
	if tw < 1 {
		tw = 1
	}
	ch := ui.JobsPanelContentRows(union)
	textW, contentH = tw, ch
	a.commandsMu.RLock()
	t := a.model.FullscreenFilePreview.CombinedText
	ph := a.model.FullscreenFilePreview.Phase
	em := a.model.FullscreenFilePreview.ErrorMsg
	a.commandsMu.RUnlock()
	switch ph {
	case ui.FilePreviewPhasePending, ui.FilePreviewPhaseRunning:
		lineCount = 1
	case ui.FilePreviewPhaseDone:
		if strings.TrimSpace(em) != "" {
			lineCount = 1
			break
		}
		lineCount = ui.FilePreviewTotalLines(t, textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

func (a *App) clampFullscreenFilePreviewScroll() {
	_, ch, lc := a.fullscreenFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) fullscreenPreviewScrollBy(delta int) {
	_, ch, lc := a.fullscreenFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll += delta
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) fullscreenPreviewScrollTo(scroll int) {
	_, ch, lc := a.fullscreenFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll = scroll
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) patchFullscreenFilePreview(fn func(*ui.FilePreviewState)) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	fn(&a.model.FullscreenFilePreview)
}

// fullscreenFilePreviewKeyboardDispatchAllowed lists actions that may reach dispatch() while
// the fullscreen file view is active. Other bindings (nav.*, file ops, panel focus, etc.) are
// ignored so they cannot mutate the hidden file panes—see tryDispatchFilePreviewFocus for the
// analogous inactive-column preview policy.
func fullscreenFilePreviewKeyboardDispatchAllowed(id string) bool {
	switch id {
	case keymap.ActionPanelRefresh,
		keymap.ActionBookmarkOpen,
		keymap.ActionBookmarkAdd,
		keymap.ActionPanelExternalBrowser,
		keymap.ActionAppUserMenu,
		keymap.ActionAppUserMenuEdit,
		keymap.ActionFileEdit,
		keymap.ActionUIOpenTheme,
		keymap.ActionUIOpenConfig,
		keymap.ActionPanelDiskUsageAbortAll,
		keymap.ActionJobsOpen,
		keymap.ActionCommandsOpen,
		keymap.ActionMessagesOpen:
		return true
	default:
		return false
	}
}

// handleFilePreviewViewKey handles keys while ViewFilePreview is active (not blocked by transfer menu).
func (a *App) handleFilePreviewViewKey(event *tcell.EventKey) (quit bool) {
	switch event.Key() {
	case tcell.KeyEsc:
		a.closeFilePreviewFullscreen()
		return false
	}

	// Scroll using raw keys before action resolution. Up/Down (etc.) are normally bound to
	// nav.* and would otherwise dispatch to the file list behind the fullscreen view.
	_, ch, _ := a.fullscreenFilePreviewScrollMetrics()
	step := ch
	if step < 1 {
		step = 1
	}
	switch event.Key() {
	case tcell.KeyUp:
		a.fullscreenPreviewScrollBy(-1)
		return false
	case tcell.KeyDown:
		a.fullscreenPreviewScrollBy(1)
		return false
	case tcell.KeyPgUp:
		a.fullscreenPreviewScrollBy(-step)
		return false
	case tcell.KeyPgDn:
		a.fullscreenPreviewScrollBy(step)
		return false
	case tcell.KeyHome:
		a.fullscreenPreviewScrollTo(0)
		return false
	case tcell.KeyEnd:
		_, ch2, lc := a.fullscreenFilePreviewScrollMetrics()
		a.fullscreenPreviewScrollTo(max(0, lc-ch2))
		return false
	case tcell.KeyLeft, tcell.KeyRight:
		// Default bindings map these to nav.parent / nav.open; consume unmodified arrows so
		// chord bindings (e.g. history forward) still resolve below when modifiers are set.
		if event.Modifiers() == tcell.ModNone {
			return false
		}
	}

	nextAction := a.actionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate()
	}
	if nextAction != "" {
		if nextAction == keymap.ActionFileView || nextAction == keymap.ActionFileQuickView {
			return false
		}
		if !fullscreenFilePreviewKeyboardDispatchAllowed(nextAction) {
			return false
		}
		a.dispatch(nextAction)
		return false
	}
	return false
}

// openFilePreviewFullscreen opens the full-screen file view (F3 / file.view) for the active selection.
func (a *App) openFilePreviewFullscreen() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
		return
	}
	if a.inQuickFilterUI() {
		return
	}
	active := a.activePanel()
	entry, ok := active.CurrentEntry()
	if !ok || entry.Type == localfs.EntryDirectory {
		a.setTransientMessage("View: select a file", ui.MessageUrgencyWarn)
		return
	}
	path := filepath.Clean(entry.Path)
	if path == "" || path == "." {
		a.setErrorMessage("View", fmt.Errorf("no path"))
		return
	}
	if err := localfs.CheckFilePreviewable(path); err != nil {
		switch {
		case errors.Is(err, localfs.ErrFilePreviewBinary):
			a.setTransientMessage("View: not a text file", ui.MessageUrgencyWarn)
		case errors.Is(err, localfs.ErrFilePreviewIsDir):
			a.setTransientMessage("View: not a file", ui.MessageUrgencyWarn)
		default:
			a.setErrorMessage("View", err)
		}
		return
	}
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		a.setTransientMessage("View: terminal too small", ui.MessageUrgencyWarn)
		return
	}
	union := ui.MergeTwinPanelRects(lay.Left, lay.Right)
	tw := union.Width - 4
	if tw < 1 {
		tw = 1
	}
	argv, err := cmdrun.BuildFilePreviewArgv(a.config.Preview.Command, path, tw)
	if err != nil {
		a.setErrorMessage("Preview command", err)
		return
	}
	titleBase := filepath.Base(path)
	a.model.ViewMode = ui.ViewFilePreview
	a.model.Menu.Open = false
	a.model.Menu.PulldownOpen = false
	a.model.MenuDefinitions = a.browserMenuDefinitions()
	a.model.Menu.ActiveMenu = menu.DefaultIndex()
	a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhasePending
		st.Path = path
		st.TitleBase = titleBase
		st.CombinedText = ""
		st.Scroll = 0
		st.ExitCode = 0
		st.ErrorMsg = ""
	})
	a.postCommandWake()
	go a.runFilePreview(a.commandsCtx, path, argv, active.PathString(), true)
}
