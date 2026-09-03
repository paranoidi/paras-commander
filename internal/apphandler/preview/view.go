package preview

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// toggleFilePreviewRawMarkdown flips the fullscreen preview of a markdown file between
// rendered markdown and raw Chroma-highlighted source. No-op for non-markdown files and
// while showing a git diff (IsDiff), per file.view.toggle-raw.
func (h *Handler) toggleFilePreviewRawMarkdown() {
	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	if !st.Open || st.Path == "" || st.IsDiff || !previewrun.IsMarkdownPath(st.Path) {
		return
	}
	h.model.FullscreenFilePreviewRawMarkdown = !h.model.FullscreenFilePreviewRawMarkdown
	h.patchFullscreenFilePreview(func(fp *ui.FilePreviewState) {
		fp.Scroll = 0
	})
	h.refreshFullscreenFilePreview()
	h.patchFullscreenFilePreview(func(fp *ui.FilePreviewState) {
		if fp.Search.Active {
			fp.RecomputeSearch()
		}
	})
}

// FilePreviewToggleRawFooterEligible reports whether the footer should show F6 Raw/Render:
// true when the fullscreen preview is open on a markdown file that isn't a git diff,
// mirroring toggleFilePreviewRawMarkdown's own no-op guard.
func (h *Handler) FilePreviewToggleRawFooterEligible() bool {
	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	return st.Open && st.Path != "" && !st.IsDiff && previewrun.IsMarkdownPath(st.Path)
}

// closeOrQuitFilePreview closes the fullscreen preview back to the browser, or quits the
// app instead when it was launched directly via `pc <file>` (no browser to fall back to).
func (h *Handler) closeOrQuitFilePreview() bool {
	if h.host.LaunchedAsFileViewer() {
		return h.host.HandleQuit()
	}
	h.CloseFilePreviewFullscreen()
	return false
}

// CloseFilePreviewFullscreen exits the F3 fullscreen preview view back to the browser.
func (h *Handler) CloseFilePreviewFullscreen() {
	h.mu.Lock()
	h.model.FullscreenFilePreview = ui.FilePreviewState{}
	h.model.FullscreenFilePreviewSearchField = dialog.FileDialogField{}
	h.mu.Unlock()
	h.closeFilePreviewThemePicker(false)
	h.clearFilePreviewHold(previewTargetFullscreen)
	h.model.ViewMode = ui.ViewBrowser
	h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
	h.host.FilePreviewFullscreenClosed()
}

func (h *Handler) fullscreenFilePreviewScrollMetrics() (textW, contentH, lineCount int) {
	tw, ch, layOK := h.fullscreenFilePreviewLayoutMetrics()
	if !layOK {
		return tw, ch, 0
	}
	textW, contentH = tw, ch
	h.mu.RLock()
	ph := h.model.FullscreenFilePreview.Phase
	em := h.model.FullscreenFilePreview.ErrorMsg
	h.mu.RUnlock()
	switch ph {
	case ui.FilePreviewPhasePending, ui.FilePreviewPhaseRunning:
		lineCount = 1
	case ui.FilePreviewPhaseDone:
		if strings.TrimSpace(em) != "" {
			lineCount = 1
			break
		}
		lineCount = h.fullscreenFilePreviewLineCount(textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

// ClampFullscreenFilePreviewScroll clamps the F3 fullscreen preview scroll to the valid range,
// e.g. after a resize changes the visible content height. No-op while content is (re)rendering.
func (h *Handler) ClampFullscreenFilePreviewScroll() {
	h.mu.RLock()
	ph := h.model.FullscreenFilePreview.Phase
	h.mu.RUnlock()
	if ph == ui.FilePreviewPhasePending || ph == ui.FilePreviewPhaseRunning {
		// Content is being (re)rendered (e.g. theme switch in progress) and its
		// line count is a placeholder — clamping now would zero a valid scroll
		// position before the real content lands.
		return
	}
	_, ch, lc := h.fullscreenFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) fullscreenPreviewScrollBy(delta int) {
	_, ch, lc := h.fullscreenFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll += delta
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) fullscreenPreviewScrollTo(scroll int) {
	_, ch, lc := h.fullscreenFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll = scroll
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) patchFullscreenFilePreview(fn func(*ui.FilePreviewState)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fn(&h.model.FullscreenFilePreview)
}

// fullscreenFilePreviewKeyboardDispatchAllowed lists actions that may reach dispatch() while
// the fullscreen file view is active. Preview-local bindings (scroll, search, edit, etc.) are
// handled in HandleFilePreviewViewKey; only auxiliary full-screen views may fall through here.
func fullscreenFilePreviewKeyboardDispatchAllowed(id string) bool {
	switch id {
	case keymap.ActionPanelExternalBrowser,
		keymap.ActionJobsOpen,
		keymap.ActionCommandsOpen,
		keymap.ActionMessagesOpen,
		keymap.ActionPreviewImageCapabilityDialog:
		return true
	default:
		return false
	}
}

// HandleFilePreviewViewKey handles keys while ViewFilePreview is active (not blocked by transfer menu).
func (h *Handler) HandleFilePreviewViewKey(event *tcell.EventKey) (quit bool) {
	if h.model.FullscreenFilePreview.Search.Editing {
		return h.handleFilePreviewSearchTypingKey(event)
	}
	nextAction := h.host.ActionFromKeyEvent(event)
	if quit, handled := h.tryFilePreviewAction(nextAction); handled {
		return quit
	}
	if h.model.FilePreviewThemePicker.Open {
		if h.handleFilePreviewThemePickerKey(event) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		if h.model.FullscreenFilePreview.Search.Active {
			h.clearFilePreviewSearchField()
			h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) { st.CancelSearch() })
			return false
		}
		return h.closeOrQuitFilePreview()
	case tcell.KeyLeft:
		// Left also exits the view (Esc is the primary key); modified Left falls through to chord bindings.
		if event.Modifiers() == tcell.ModNone {
			return h.closeOrQuitFilePreview()
		}
	}

	// Scroll using raw keys before action resolution. Up/Down (etc.) are normally bound to
	// nav.* and would otherwise dispatch to the file list behind the fullscreen view.
	if h.handleFilePreviewScrollKey(event) {
		return false
	}

	nextAction = h.host.ActionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return h.host.HandleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return h.host.HandleQuitImmediate()
	}
	if nextAction != "" {
		if nextAction == keymap.ActionFileView || nextAction == keymap.ActionFileQuickView {
			return false
		}
		if !fullscreenFilePreviewKeyboardDispatchAllowed(nextAction) {
			return false
		}
		h.host.Dispatch(nextAction)
		return false
	}
	return false
}

// tryFilePreviewAction dispatches actions handled directly by the fullscreen file preview
// (theme picker, raw-markdown toggle, diff hunk nav, search, edit/delete, quick-view paging,
// and the two quit actions). handled is true when the caller must return immediately from
// HandleFilePreviewViewKey with quit as its return value.
func (h *Handler) tryFilePreviewAction(nextAction string) (quit bool, handled bool) {
	switch nextAction {
	case keymap.ActionAppQuit:
		return h.host.HandleQuit(), true
	case keymap.ActionAppQuitImmediate:
		return h.host.HandleQuitImmediate(), true
	case keymap.ActionFileViewClose:
		return h.closeOrQuitFilePreview(), true
	case keymap.ActionFileViewMenu:
		h.host.OpenPreviewLeaderMenu()
		return false, true
	case keymap.ActionFileViewThemePicker:
		h.toggleFilePreviewThemePicker()
		return false, true
	case keymap.ActionFileViewToggleRaw:
		h.toggleFilePreviewRawMarkdown()
		return false, true
	case keymap.ActionFileViewReload:
		h.refreshFullscreenFilePreview()
		return false, true
	case keymap.ActionFileViewDiffNextHunk:
		h.hunkNavigate(previewTargetFullscreen, 1)
		return false, true
	case keymap.ActionFileViewDiffPrevHunk:
		h.hunkNavigate(previewTargetFullscreen, -1)
		return false, true
	case keymap.ActionFileViewSearchStart:
		h.startFilePreviewSearch()
		return false, true
	case keymap.ActionFileViewSearchNext:
		h.filePreviewSearchNav(1)
		return false, true
	case keymap.ActionFileViewSearchPrev:
		h.filePreviewSearchNav(-1)
		return false, true
	case keymap.ActionFileEdit:
		h.host.EditFullscreenPreviewFile()
		return false, true
	case keymap.ActionFileDelete:
		if h.model.FullscreenFilePreview.Path != "" {
			h.host.OpenDeleteDialogForPreviewedFile()
		}
		return false, true
	case keymap.ActionFileQuickViewPreviewPageUp, keymap.ActionFileQuickViewPreviewPageDown:
		_, ch, _ := h.fullscreenFilePreviewScrollMetrics()
		step := ch
		if step < 1 {
			step = 1
		}
		if nextAction == keymap.ActionFileQuickViewPreviewPageUp {
			h.fullscreenPreviewScrollBy(-step)
		} else {
			h.fullscreenPreviewScrollBy(step)
		}
		return false, true
	default:
		return false, false
	}
}

// TryFilePreviewMenuAction dispatches actionID (one of the `:` preview-menu entries) through
// tryFilePreviewAction, the same path used by fullscreen preview's own direct keys. This is
// deliberately not the generic action dispatcher: that path targets the inactive-column preview
// for diff-hunk navigation and doesn't handle Reload/Search-start at all, which would silently
// misbehave for a menu scoped to the fullscreen view. Returns the quit signal for callers wired
// like a leader-menu onActivate callback.
func (h *Handler) TryFilePreviewMenuAction(actionID string) bool {
	quit, _ := h.tryFilePreviewAction(actionID)
	return quit
}

// handleFilePreviewScrollKey handles raw scroll keys (arrows, PgUp/PgDn, space, Home/End) for
// the fullscreen file preview. Returns true if the key was consumed, in which case the caller
// must return false immediately (these keys would otherwise resolve via nav.* actions and
// dispatch to the file list behind the fullscreen view).
func (h *Handler) handleFilePreviewScrollKey(event *tcell.EventKey) bool {
	_, ch, _ := h.fullscreenFilePreviewScrollMetrics()
	step := ch
	if step < 1 {
		step = 1
	}
	switch event.Key() {
	case tcell.KeyUp:
		h.fullscreenPreviewScrollBy(-1)
		return true
	case tcell.KeyDown:
		h.fullscreenPreviewScrollBy(1)
		return true
	case tcell.KeyPgUp:
		h.fullscreenPreviewScrollBy(-step)
		return true
	case tcell.KeyPgDn:
		h.fullscreenPreviewScrollBy(step)
		return true
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			h.fullscreenPreviewScrollBy(step)
			return true
		}
	case tcell.KeyHome:
		h.fullscreenPreviewScrollTo(0)
		return true
	case tcell.KeyEnd:
		_, ch2, lc := h.fullscreenFilePreviewScrollMetrics()
		h.fullscreenPreviewScrollTo(max(0, lc-ch2))
		return true
	case tcell.KeyRight:
		// Default binding maps Right to nav.open; consume the unmodified arrow so chord
		// bindings (e.g. history forward) still resolve below when modifiers are set.
		if event.Modifiers() == tcell.ModNone {
			return true
		}
	}
	return false
}

// OpenFilePreviewFullscreen opens the full-screen file view (F3 / file.view) for the active selection.
// When the selections strip has keyboard focus, the highlighted strip row is the target
// (same resolution as quick view / EditActiveFile).
func (h *Handler) OpenFilePreviewFullscreen() {
	if h.model.ViewMode != ui.ViewBrowser {
		return
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return
	}
	if h.host.InQuickFilterUI() {
		return
	}
	path, _, mode := h.quickViewWantFile()
	if mode == quickViewWantDir {
		if dirPath, ok := h.host.SyncFollowTargetPath(h.host.ActivePanel()); ok &&
			previewrun.MatchAnyCommandRule(h.host.Config().Preview, dirPath, true, filepath.Dir(dirPath)) {
			// ponytail: unlike quick view, fullscreen has no directory-overlay fallback. If
			// every matching rule declines, this falls through into the internal preview path
			// below and errors reading the directory as a file (EISDIR) — shown as a plain
			// error message in the pane. Add a proper fallback if that proves annoying in practice.
			if err := h.OpenFullscreenFilePreviewAt(dirPath); err != nil {
				h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
			}
			return
		}
		h.host.SetTransientMessage("View: select a file", ui.MessageUrgencyWarn)
		return
	}
	if mode != quickViewWantFile && mode != quickViewWantEmpty {
		h.host.SetTransientMessage("View: select a file", ui.MessageUrgencyWarn)
		return
	}
	err := localfs.CheckFilePreviewable(path)
	isImage := errors.Is(err, localfs.ErrFilePreviewImage)
	isMedia := errors.Is(err, localfs.ErrFilePreviewMedia)
	if err != nil && !isImage && !isMedia {
		switch {
		case errors.Is(err, localfs.ErrFilePreviewBinary):
			h.host.SetTransientMessage("View: not a text file", ui.MessageUrgencyWarn)
		case errors.Is(err, localfs.ErrFilePreviewIsDir):
			h.host.SetTransientMessage("View: not a file", ui.MessageUrgencyWarn)
		default:
			h.host.SetErrorMessage("View", err)
		}
		return
	}
	if err := h.OpenFullscreenFilePreviewAt(path); err != nil {
		h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
	}
}

// OpenFullscreenFilePreviewAt opens the full-screen file view for path.
// Caller must ensure path is a previewable regular file.
func (h *Handler) OpenFullscreenFilePreviewAt(path string) error {
	path = filepath.Clean(path)
	w, ht := h.screen.Size()
	lay := h.host.LayoutForTerminalSize(w, ht)
	if lay.TooSmall {
		return fmt.Errorf("terminal too small")
	}
	union := ui.MergeTwinPanelRects(lay.Primary, lay.Secondary, h.host.EffectivePaneSplitOrientation())
	info, statErr := os.Stat(path)
	isDir := statErr == nil && info.IsDir()
	panelPath := filepath.Dir(path)
	if active := h.host.ActivePanel(); active != nil && active.PathString() != "" {
		panelPath = active.PathString()
	}
	if isDir {
		// So a rule command like "eza --tree ." works without needing %f.
		panelPath = path
	}
	titleBase := filepath.Base(path)
	h.captureFilePreviewHold(previewTargetFullscreen)
	h.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{}
	h.model.FullscreenFilePreviewRawMarkdown = false
	h.model.ViewMode = ui.ViewFilePreview
	h.clearFilePreviewSearchField()
	h.model.Menu.Open = false
	h.model.Menu.PulldownOpen = false
	h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
	h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhasePending
		st.Path = path
		st.TitleBase = titleBase
		st.IsDir = isDir
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
		// Keep ImagePayload* until the new encode finishes (stale-while-revalidate).
		st.CancelSearch()
	})
	tw, contentH, layOK := h.fullscreenFilePreviewLayoutMetrics()
	if !layOK {
		tw = union.Width - 1
		if tw < 1 {
			tw = 1
		}
		contentH = union.Height - 1
		if contentH < 0 {
			contentH = 0
		}
	}
	gen := h.filePreviewRunGen.Add(1)
	h.postRenderWake()
	go h.runPreview(
		h.ctx,
		h.previewRequest(path, tw, contentH, panelPath, h.model.PanelsChromeBlocked(), h.gitStatusForPath(path), previewTargetFullscreen, isDir),
		previewTargetFullscreen,
		gen,
	)
	return nil
}
