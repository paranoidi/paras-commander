package dialog

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenFilterDialog opens the panel Filter modal with an empty pattern (no prefill from any
// filter already active on the panel).
func (h *Handler) OpenFilterDialog() {
	metaCount := len(h.model.MetaResults[h.model.ActivePanel])
	h.model.FilterDialog = dialog.FilterDialogState{
		Open:               true,
		PatternMode:        panel.GroupPatternShell,
		MetaColumnCount:    metaCount,
		IncludeMetaColumns: metaCount > 0,
		Focus:              dialog.FilterFocusPattern,
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
	if !fd.Open {
		fd.PreviewShow = false
		return
	}
	spec := h.filterSpec()
	if spec.Empty() {
		fd.PreviewShow = false
		return
	}
	meta := ui.MetaMatchData(h.model.MetaResults[h.model.ActivePanel], fd.IncludeMetaColumns, fd.OnlyMetaColumns)
	files, dirs, err := h.host.ActivePanel().CountPatternMatches(spec, meta)
	if err != nil {
		fd.PreviewShow = false
		return
	}
	fd.PreviewFiles = files
	fd.PreviewFolders = dirs
	fd.PreviewShow = true
}

// filterTypeRegexps returns the [filter.types] regexps in dialog.FilterType* order.
func filterTypeRegexps(t config.FilterTypesConfig) [dialog.FilterTypeCount]string {
	return [dialog.FilterTypeCount]string{t.Videos, t.Pictures, t.Archives, t.Audio, t.Documents}
}

// filterSpec builds the panel.FilterSpec for the dialog's current state: checked types are
// OR-joined into one regexp from the (already validated) config.
func (h *Handler) filterSpec() panel.FilterSpec {
	fd := &h.model.FilterDialog
	spec := panel.FilterSpec{
		Pattern: fd.Text, Mode: fd.PatternMode, CaseSensitive: fd.CaseSensitive,
		FilesOnly: fd.FilesOnly, DirsOnly: fd.DirsOnly, Invert: fd.Invert,
	}
	var alts, labels []string
	rx := filterTypeRegexps(h.host.Config().Filter.Types)
	for i, on := range fd.Types {
		if on {
			alts = append(alts, "(?:"+rx[i]+")")
			labels = append(labels, dialog.FilterTypeLabels[i])
		}
	}
	if len(alts) > 0 {
		// Config is validated, so this compiles; on failure the type restriction is dropped.
		if re, err := regexp.Compile(strings.Join(alts, "|")); err == nil {
			spec.Types = re
			spec.TypeLabel = strings.Join(labels, ", ")
		}
	}
	return spec
}

func (h *Handler) filterDialogForm() dialog.DialogLinearForm {
	return dialog.NewDialogLinearForm(dialog.FilterContentCount(h.model.FilterDialog.MetaColumnCount))
}

func (h *Handler) filterDialogQueryWidth() int {
	termW, _ := h.screen.Size()
	width := dialog.FilterDialogWidth
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
		fd.Focus = dialog.FilterFocusFilesOnly
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
	panelID := h.model.ActivePanel
	spec := h.filterSpec()
	if spec.Empty() {
		p.SetEntryFilter(nil)
		h.CloseFilterDialog()
		return
	}
	if h.tryRejectFilterDialogOK() {
		return
	}
	var metaFn func() panel.GroupSelectMeta
	if fd.IncludeMetaColumns {
		only := fd.OnlyMetaColumns
		// Closure re-reads live MetaResults on every filter rebuild rather than snapshotting: meta
		// results arrive asynchronously and the underlying maps are replaced on a directory change.
		metaFn = func() panel.GroupSelectMeta {
			return ui.MetaMatchData(h.model.MetaResults[panelID], true, only)
		}
	}
	filter, err := panel.PatternFilter(spec, metaFn)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	meta := ui.MetaMatchData(h.model.MetaResults[panelID], fd.IncludeMetaColumns, fd.OnlyMetaColumns)
	files, dirs, _ := p.CountPatternMatches(spec, meta)
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
		case 'l', 'L':
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
			case 'i', 'I':
				h.toggleFilterDialogField(fd, dialog.FilterFocusInvert)
				fd.Focus = dialog.FilterFocusInvert
			case 'e', 'E':
				if h.toggleFilterDialogField(fd, dialog.FilterFocusCase) {
					fd.Focus = dialog.FilterFocusCase
				}
			case 'm', 'M':
				if h.toggleFilterDialogField(fd, dialog.FilterFocusIncludeMeta) {
					fd.Focus = dialog.FilterFocusIncludeMeta
				}
			case 'n', 'N':
				if h.toggleFilterDialogField(fd, dialog.FilterFocusOnlyMeta) {
					fd.Focus = dialog.FilterFocusOnlyMeta
				}
			default:
				for i, m := range dialog.FilterTypeMnemonics {
					if unicode.ToUpper(event.Rune()) == m {
						fd.Focus = dialog.FilterFocusTypeFirst + i
						h.toggleFilterDialogField(fd, fd.Focus)
					}
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
	if focus, ok := dialog.FilterMoveFocus(fd.Focus, event.Key(), fd.PatternMode, fd.MetaColumnCount); ok {
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
	case dialog.FilterFocusInvert:
		fd.Invert = !fd.Invert
	case dialog.FilterFocusCase:
		if !dialog.FilterShowsCaseSensitive(*fd) {
			return false
		}
		fd.CaseSensitive = !fd.CaseSensitive
	case dialog.FilterFocusIncludeMeta:
		if fd.MetaColumnCount <= 0 {
			return false
		}
		fd.IncludeMetaColumns = !fd.IncludeMetaColumns
		if !fd.IncludeMetaColumns {
			fd.OnlyMetaColumns = false
		}
	case dialog.FilterFocusOnlyMeta:
		if fd.MetaColumnCount <= 0 {
			return false
		}
		fd.OnlyMetaColumns = !fd.OnlyMetaColumns
		if fd.OnlyMetaColumns {
			fd.IncludeMetaColumns = true
		}
	default:
		if focus < dialog.FilterFocusTypeFirst || focus >= dialog.FilterFocusPattern {
			return false
		}
		fd.Types[focus-dialog.FilterFocusTypeFirst] = !fd.Types[focus-dialog.FilterFocusTypeFirst]
	}
	h.updateFilterDialogPreview()
	return true
}

func filterAltIsDialogMnemonic(r rune) bool {
	switch r {
	case 'd', 'D', 'f', 'F', 'e', 'E', 'r', 'R', 's', 'S', 'l', 'L', 'i', 'I', 'm', 'M', 'n', 'N', 'v', 'V', 'p', 'P', 'a', 'A', 'u', 'U', 't', 'T':
		return true
	default:
		return false
	}
}
