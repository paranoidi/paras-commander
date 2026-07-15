package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// toggleFilePreviewRawMarkdown flips the fullscreen preview of a markdown file between
// rendered markdown and raw Chroma-highlighted source. No-op for non-markdown files and
// while showing a git diff (IsDiff), per file.view.toggle-raw.
func (a *App) toggleFilePreviewRawMarkdown() {
	a.commandsMu.RLock()
	st := a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if !st.Open || st.Path == "" || st.IsDiff || !preview.IsMarkdownPath(st.Path) {
		return
	}
	a.model.FullscreenFilePreviewRawMarkdown = !a.model.FullscreenFilePreviewRawMarkdown
	a.patchFullscreenFilePreview(func(fp *ui.FilePreviewState) {
		fp.Scroll = 0
	})
	a.refreshFullscreenFilePreview()
	a.patchFullscreenFilePreview(func(fp *ui.FilePreviewState) {
		if fp.Search.Active {
			fp.RecomputeSearch()
		}
	})
}

func (a *App) closeFilePreviewFullscreen() {
	a.commandsMu.Lock()
	a.model.FullscreenFilePreview = ui.FilePreviewState{}
	a.model.FullscreenFilePreviewSearchField = dialog.FileDialogField{}
	a.commandsMu.Unlock()
	a.closeFilePreviewThemePicker(false)
	a.clearFilePreviewHold(previewTargetFullscreen)
	a.model.ViewMode = ui.ViewBrowser
	a.model.MenuDefinitions = a.browserMenuDefinitions()
	a.model.Menu.ActiveMenu = menu.DefaultIndex()
}

func (a *App) fullscreenFilePreviewScrollMetrics() (textW, contentH, lineCount int) {
	tw, ch, layOK := a.fullscreenFilePreviewLayoutMetrics()
	if !layOK {
		return tw, ch, 0
	}
	textW, contentH = tw, ch
	a.commandsMu.RLock()
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
		lineCount = a.fullscreenFilePreviewLineCount(textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

func (a *App) clampFullscreenFilePreviewScroll() {
	a.commandsMu.RLock()
	ph := a.model.FullscreenFilePreview.Phase
	a.commandsMu.RUnlock()
	if ph == ui.FilePreviewPhasePending || ph == ui.FilePreviewPhaseRunning {
		// Content is being (re)rendered (e.g. theme switch in progress) and its
		// line count is a placeholder — clamping now would zero a valid scroll
		// position before the real content lands.
		return
	}
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
// the fullscreen file view is active. Preview-local bindings (scroll, search, edit, etc.) are
// handled in handleFilePreviewViewKey; only auxiliary full-screen views may fall through here.
func fullscreenFilePreviewKeyboardDispatchAllowed(id string) bool {
	switch id {
	case keymap.ActionPanelExternalBrowser,
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
	if a.model.FullscreenFilePreview.Search.Editing {
		return a.handleFilePreviewSearchTypingKey(event)
	}
	nextAction := a.actionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate()
	}
	if nextAction == keymap.ActionFileViewThemePicker {
		a.toggleFilePreviewThemePicker()
		return false
	}
	if nextAction == keymap.ActionFileViewToggleRaw {
		a.toggleFilePreviewRawMarkdown()
		return false
	}
	if nextAction == keymap.ActionFileViewDiffNextHunk {
		a.hunkNavigate(previewTargetFullscreen, 1)
		return false
	}
	if nextAction == keymap.ActionFileViewDiffPrevHunk {
		a.hunkNavigate(previewTargetFullscreen, -1)
		return false
	}
	if nextAction == keymap.ActionFileViewSearchStart {
		a.startFilePreviewSearch()
		return false
	}
	if nextAction == keymap.ActionFileViewSearchNext {
		a.filePreviewSearchNav(1)
		return false
	}
	if nextAction == keymap.ActionFileViewSearchPrev {
		a.filePreviewSearchNav(-1)
		return false
	}
	if nextAction == keymap.ActionFileEdit {
		a.editFullscreenPreviewFile()
		return false
	}
	if nextAction == keymap.ActionFileDelete {
		if a.model.FullscreenFilePreview.Path != "" {
			a.openDeleteDialogForPreviewedFile()
		}
		return false
	}
	if nextAction == keymap.ActionFileQuickViewPreviewPageUp || nextAction == keymap.ActionFileQuickViewPreviewPageDown {
		_, ch, _ := a.fullscreenFilePreviewScrollMetrics()
		step := ch
		if step < 1 {
			step = 1
		}
		if nextAction == keymap.ActionFileQuickViewPreviewPageUp {
			a.fullscreenPreviewScrollBy(-step)
		} else {
			a.fullscreenPreviewScrollBy(step)
		}
		return false
	}
	if a.model.FilePreviewThemePicker.Open {
		if a.handleFilePreviewThemePickerKey(event) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		if a.model.FullscreenFilePreview.Search.Active {
			a.clearFilePreviewSearchField()
			a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) { st.CancelSearch() })
			return false
		}
		a.closeFilePreviewFullscreen()
		return false
	case tcell.KeyLeft:
		// Left also exits the view (Esc is the primary key); modified Left falls through to chord bindings.
		if event.Modifiers() == tcell.ModNone {
			a.closeFilePreviewFullscreen()
			return false
		}
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
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			a.fullscreenPreviewScrollBy(step)
			return false
		}
	case tcell.KeyHome:
		a.fullscreenPreviewScrollTo(0)
		return false
	case tcell.KeyEnd:
		_, ch2, lc := a.fullscreenFilePreviewScrollMetrics()
		a.fullscreenPreviewScrollTo(max(0, lc-ch2))
		return false
	case tcell.KeyRight:
		// Default binding maps Right to nav.open; consume the unmodified arrow so chord
		// bindings (e.g. history forward) still resolve below when modifiers are set.
		if event.Modifiers() == tcell.ModNone {
			return false
		}
	}

	nextAction = a.actionFromKeyEvent(event)
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
	if err := a.openFullscreenFilePreviewAt(path); err != nil {
		a.setTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
	}
}

// openFullscreenFilePreviewAt opens the full-screen file view for path.
// Caller must ensure path is a previewable regular file.
func (a *App) openFullscreenFilePreviewAt(path string) error {
	path = filepath.Clean(path)
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return fmt.Errorf("terminal too small")
	}
	union := ui.MergeTwinPanelRects(lay.Primary, lay.Secondary, a.effectivePaneSplitOrientation())
	tw := union.Width - 4
	if tw < 1 {
		tw = 1
	}
	panelPath := filepath.Dir(path)
	if active := a.activePanel(); active != nil && active.PathString() != "" {
		panelPath = active.PathString()
	}
	titleBase := filepath.Base(path)
	a.captureFilePreviewHold(previewTargetFullscreen)
	a.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{}
	a.model.FullscreenFilePreviewRawMarkdown = false
	a.model.ViewMode = ui.ViewFilePreview
	a.clearFilePreviewSearchField()
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
		st.SetHighlightedCells(nil)
		st.Source = ui.PreviewSourceExternalANSI
		st.Scroll = 0
		st.ExitCode = 0
		st.ErrorMsg = ""
		st.IsDiff = false
		st.DiffHunkLines = nil
		st.GitStatusText = ""
		st.GitStatusThemeKey = ""
		st.CancelSearch()
	})
	gen := a.filePreviewRunGen.Add(1)
	a.postCommandWake()
	go a.runPreview(
		a.commandsCtx,
		a.previewRequest(path, tw, panelPath, a.model.PanelsChromeBlocked(), a.gitStatusForPath(path), previewTargetFullscreen),
		previewTargetFullscreen,
		gen,
	)
	return nil
}
