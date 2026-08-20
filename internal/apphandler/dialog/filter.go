package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenFilterDialog opens the panel Filter modal with an empty pattern (no prefill from any
// filter already active on the panel).
func (h *Handler) OpenFilterDialog() {
	h.model.FilterDialog = dialog.FilterDialogState{
		Open:        true,
		PatternMode: panel.GroupPatternShell,
		Focus:       dialog.FilterFocusPattern,
	}
}

// CloseFilterDialog closes the Filter modal without changing the panel's active filter.
func (h *Handler) CloseFilterDialog() {
	h.model.FilterDialog.Open = false
	h.model.FilterDialog.Text = ""
	h.model.FilterDialog.PreviewShow = false
}

// updateFilterDialogPreview recomputes the Filter dialog's live match-count preview from the
// current pattern and options. No-op when the dialog is closed; hides the preview when the
// pattern is empty or fails to compile.
func (h *Handler) updateFilterDialogPreview() {
	fd := &h.model.FilterDialog
	if !fd.Open || fd.Text == "" {
		fd.PreviewShow = false
		return
	}
	files, dirs, err := h.host.ActivePanel().CountPatternMatches(fd.Text, fd.PatternMode, fd.CaseSensitive, fd.FilesOnly, fd.DirsOnly)
	if err != nil {
		fd.PreviewShow = false
		return
	}
	fd.PreviewFiles = files
	fd.PreviewFolders = dirs
	fd.PreviewShow = true
}

func (h *Handler) filterDialogForm() dialog.DialogLinearForm {
	return dialog.NewDialogLinearForm(7)
}

func (h *Handler) filterDialogQueryWidth() int {
	termW, _ := h.screen.Size()
	width := 54
	if width > termW-4 {
		width = termW - 4
	}
	if width < 30 {
		width = 30
	}
	return scrollquery.DialogInputWidthFromFrame(width)
}

func (h *Handler) applyFilterDialogModeFromFocus() {
	fd := &h.model.FilterDialog
	switch fd.Focus {
	case dialog.FilterFocusShellRadio:
		fd.PatternMode = panel.GroupPatternShell
	case dialog.FilterFocusRegexRadio:
		fd.PatternMode = panel.GroupPatternRegex
	case dialog.FilterFocusSimpleRadio:
		fd.PatternMode = panel.GroupPatternSimple
	}
	h.filterDialogClampCaseFocus()
	h.updateFilterDialogPreview()
}

func (h *Handler) filterDialogClampCaseFocus() {
	fd := &h.model.FilterDialog
	if fd.PatternMode == panel.GroupPatternRegex && fd.Focus == dialog.FilterFocusCase {
		fd.Focus = dialog.FilterFocusDirsOnly
	}
}

// tryRejectFilterDialogOK reports whether OK should be rejected because the (non-empty) pattern
// fails to compile, showing a transient error message when so.
func (h *Handler) tryRejectFilterDialogOK() bool {
	fd := &h.model.FilterDialog
	if fd.Text == "" {
		return false
	}
	_, err := panel.NewGroupMatcher(fd.Text, fd.PatternMode, fd.CaseSensitive)
	if err == nil {
		return false
	}
	h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
	return true
}

// executeFilterDialog applies the Filter dialog's OK action. An empty pattern clears the
// active filter (there is no separate "Clear filter" button); a non-empty pattern installs a
// panel.PatternFilter, replacing whatever filter (e.g. a git-status filter) was active before.
func (h *Handler) executeFilterDialog() {
	fd := &h.model.FilterDialog
	p := h.host.ActivePanel()
	if fd.Text == "" {
		p.SetEntryFilter(nil)
		h.CloseFilterDialog()
		return
	}
	if h.tryRejectFilterDialogOK() {
		return
	}
	filter, err := panel.PatternFilter(fd.Text, fd.PatternMode, fd.CaseSensitive, fd.FilesOnly, fd.DirsOnly)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	files, dirs, _ := p.CountPatternMatches(fd.Text, fd.PatternMode, fd.CaseSensitive, fd.FilesOnly, fd.DirsOnly)
	p.SetEntryFilter(filter)
	h.CloseFilterDialog()
	if files == 0 && dirs == 0 {
		h.host.SetTransientMessage("No matches", ui.MessageUrgencyWarn)
		return
	}
	h.host.SetTransientMessage(fmt.Sprintf("Filtered: %d files, %d folders", files, dirs), ui.MessageUrgencyInfo)
}

// confirmFilterDialogFromInput applies the pattern row then runs OK (Enter / Alt+O).
func (h *Handler) confirmFilterDialogFromInput() {
	fd := &h.model.FilterDialog
	if fd.Focus == dialog.FilterFocusPattern {
		e := scrollquery.NewEdit(&fd.Text, &fd.TextCursor, &fd.TextScroll, h.filterDialogQueryWidth(), nil)
		e.Apply()
	}
	if fd.Focus >= dialog.FilterFocusShellRadio && fd.Focus <= dialog.FilterFocusSimpleRadio {
		h.applyFilterDialogModeFromFocus()
		return
	}
	if h.tryRejectFilterDialogOK() {
		return
	}
	h.executeFilterDialog()
}

// HandleFilterDialogKey routes a key event for the open Filter dialog. Returns whether the app
// should quit (always false here).
func (h *Handler) HandleFilterDialogKey(event *tcell.EventKey) bool {
	fd := &h.model.FilterDialog
	form := h.filterDialogForm()

	if dialog.AltDialogOK(event) {
		h.confirmFilterDialogFromInput()
		return false
	}
	if dialog.AltDialogCancel(event) {
		h.CloseFilterDialog()
		return false
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		h.CloseFilterDialog()
		return false
	case tcell.KeyEnter:
		if fd.Focus == form.CancelIndex() {
			h.CloseFilterDialog()
			return false
		}
		if fd.Focus >= dialog.FilterFocusShellRadio && fd.Focus <= dialog.FilterFocusSimpleRadio {
			h.applyFilterDialogModeFromFocus()
			return false
		}
		h.confirmFilterDialogFromInput()
		return false
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			h.toggleFilterDialogField(fd, dialog.FilterFocusShellRadio)
			return false
		case 'r', 'R':
			h.toggleFilterDialogField(fd, dialog.FilterFocusRegexRadio)
			return false
		case 'i', 'I':
			h.toggleFilterDialogField(fd, dialog.FilterFocusSimpleRadio)
			return false
		}
	}

	if fd.Focus == dialog.FilterFocusPattern {
		skipScrolling := event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) && filterAltIsDialogMnemonic(event.Rune())
		edit := scrollquery.NewEdit(&fd.Text, &fd.TextCursor, &fd.TextScroll, h.filterDialogQueryWidth(), h.updateFilterDialogPreview)
		if !skipScrolling && scrollquery.HandleKey(h.keysDialogInput, event, true, edit) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyRune:
		if keymap.AltLetterModifiers(event.Modifiers()) {
			switch event.Rune() {
			case 'f', 'F':
				h.toggleFilterDialogField(fd, dialog.FilterFocusFilesOnly)
				fd.Focus = dialog.FilterFocusFilesOnly
			case 'd', 'D':
				h.toggleFilterDialogField(fd, dialog.FilterFocusDirsOnly)
				fd.Focus = dialog.FilterFocusDirsOnly
			case 'e', 'E':
				if h.toggleFilterDialogField(fd, dialog.FilterFocusCase) {
					fd.Focus = dialog.FilterFocusCase
				}
			}
			return false
		}
		mod := event.Modifiers()
		if mod != tcell.ModNone && mod != tcell.ModShift {
			break
		}
		if dialog.DialogButtonRune(event.Rune()) == dialog.ButtonRuneToggle {
			switch fd.Focus {
			case form.OKIndex():
				h.confirmFilterDialogFromInput()
			case form.CancelIndex():
				h.CloseFilterDialog()
			default:
				h.toggleFilterDialogField(fd, fd.Focus)
			}
			return false
		}
	}
	if focus, ok := dialog.FilterMoveFocus(fd.Focus, event.Key(), fd.PatternMode); ok {
		fd.Focus = focus
	}
	return false
}

// toggleFilterDialogField applies the toggle/radio-set action addressed by focus (one of the
// dialog.FilterFocus* constants). Returns false when focus doesn't address a toggleable field,
// or the field is conditionally hidden (case-sensitivity when not shown), so nothing changed.
func (h *Handler) toggleFilterDialogField(fd *dialog.FilterDialogState, focus int) bool {
	switch focus {
	case dialog.FilterFocusShellRadio:
		fd.PatternMode = panel.GroupPatternShell
		h.filterDialogClampCaseFocus()
	case dialog.FilterFocusRegexRadio:
		fd.PatternMode = panel.GroupPatternRegex
		h.filterDialogClampCaseFocus()
	case dialog.FilterFocusSimpleRadio:
		fd.PatternMode = panel.GroupPatternSimple
	case dialog.FilterFocusFilesOnly:
		fd.FilesOnly = !fd.FilesOnly
		if fd.FilesOnly {
			fd.DirsOnly = false
		}
	case dialog.FilterFocusDirsOnly:
		fd.DirsOnly = !fd.DirsOnly
		if fd.DirsOnly {
			fd.FilesOnly = false
		}
	case dialog.FilterFocusCase:
		if !dialog.FilterShowsCaseSensitive(*fd) {
			return false
		}
		fd.CaseSensitive = !fd.CaseSensitive
	default:
		return false
	}
	h.updateFilterDialogPreview()
	return true
}

func filterAltIsDialogMnemonic(r rune) bool {
	switch r {
	case 'f', 'F', 'd', 'D', 'e', 'E', 'r', 'R', 's', 'S', 'i', 'I':
		return true
	default:
		return false
	}
}
