package dialog

import (
	"fmt"
	"path"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// OpenFilterDialog opens the Filter modal with an empty pattern (no prefill from any filter
// already active on the panel). The target is the active panel, or the Compare / Find
// duplicates rows when that view is showing.
func (h *Handler) OpenFilterDialog() {
	metaCount := len(h.model.MetaResults[h.model.ActivePanel])
	target := dialog.FilterTargetPanel
	switch h.model.ViewMode {
	case ui.ViewCompare:
		target = dialog.FilterTargetCompare
	case ui.ViewDedup:
		target = dialog.FilterTargetDedup
	}
	if target != dialog.FilterTargetPanel {
		metaCount = 0 // compare and dedup rows have no meta columns
	}
	h.model.FilterDialog = dialog.FilterDialogState{
		Open:               true,
		Target:             target,
		PatternMode:        panel.GroupPatternShell,
		MetaColumnCount:    metaCount,
		IncludeMetaColumns: metaCount > 0,
		Focus:              dialog.PatternInput,
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
	if fd.Target != dialog.FilterTargetPanel {
		var n int
		var err error
		if fd.Target == dialog.FilterTargetDedup {
			n, err = h.dedupMatchCount(spec)
		} else {
			n, err = h.compareMatchCount(spec)
		}
		fd.PreviewShow = err == nil
		fd.PreviewFiles, fd.PreviewFolders = n, 0
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

// compareMatchCount counts the compare rows (after the category / empty-file filters) whose
// basename matches spec. Compare rows are all files.
func (h *Handler) compareMatchCount(spec panel.FilterSpec) (int, error) {
	match, err := spec.MatchFunc()
	if err != nil {
		return 0, err
	}
	v := h.model.CompareView
	return len(comparepkg.FilteredRows(h.model.CompareSnapshot, v.Filter, v.IgnoreEmpty, compareNameMatcher(match))), nil
}

func compareNameMatcher(match func(localfs.Entry, panel.GroupSelectMeta) bool) func(string) bool {
	return func(name string) bool { return match(localfs.Entry{Name: name}, panel.GroupSelectMeta{}) }
}

// dedupGroupFilter builds the Find duplicates predicate: a group is kept when any copy's
// basename matches spec, with Invert applied at group level (no copy matches).
func dedupGroupFilter(spec panel.FilterSpec) (func(comparepkg.DedupGroup) bool, error) {
	invert := spec.Invert
	spec.Invert = false
	match, err := spec.MatchFunc()
	if err != nil {
		return nil, err
	}
	name := compareNameMatcher(match)
	return func(g comparepkg.DedupGroup) bool {
		for _, f := range g.Files {
			if name(path.Base(f.Abs.String())) {
				return !invert
			}
		}
		return invert
	}, nil
}

// dedupMatchCount counts the files in the groups the view would show under spec.
func (h *Handler) dedupMatchCount(spec panel.FilterSpec) (int, error) {
	fn, err := dedupGroupFilter(spec)
	if err != nil {
		return 0, err
	}
	view := h.model.DedupView
	view.GroupFilter = fn
	n := 0
	for _, g := range ui.DedupActiveGroups(h.model.DedupSnapshot, view) {
		n += len(g.Files)
	}
	return n, nil
}

// executeDedupFilterDialog applies the dialog to the Find duplicates groups; an empty spec
// clears the filter.
func (h *Handler) executeDedupFilterDialog(spec panel.FilterSpec) {
	if spec.Empty() {
		h.dedup.SetGroupFilter(nil, "")
		h.CloseFilterDialog()
		return
	}
	fn, err := dedupGroupFilter(spec)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	n, _ := h.dedupMatchCount(spec)
	h.dedup.SetGroupFilter(fn, spec.Label())
	h.CloseFilterDialog()
	if n == 0 {
		h.host.SetTransientMessage("No matches", ui.MessageUrgencyWarn)
		return
	}
	h.host.SetTransientMessage(fmt.Sprintf("Filtered: %d files", n), ui.MessageUrgencyInfo)
}

// executeCompareFilterDialog applies the dialog to the Compare view's rows; an empty spec clears
// the name filter.
func (h *Handler) executeCompareFilterDialog(spec panel.FilterSpec) {
	v := &h.model.CompareView
	if spec.Empty() {
		v.NameFilter, v.NameFilterLabel = nil, ""
		h.CloseFilterDialog()
		return
	}
	match, err := spec.MatchFunc()
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	n, _ := h.compareMatchCount(spec)
	v.NameFilter, v.NameFilterLabel = compareNameMatcher(match), spec.Label()
	v.Selected, v.ListScroll = 0, 0
	h.CloseFilterDialog()
	if n == 0 {
		h.host.SetTransientMessage("No matches", ui.MessageUrgencyWarn)
		return
	}
	h.host.SetTransientMessage(fmt.Sprintf("Filtered: %d files", n), ui.MessageUrgencyInfo)
}

// filterSpec builds the panel.FilterSpec for the dialog's current state.
func (h *Handler) filterSpec() panel.FilterSpec {
	fd := &h.model.FilterDialog
	spec := panel.FilterSpec{
		Pattern: fd.Text, Mode: fd.PatternMode, CaseSensitive: fd.CaseSensitive,
		FilesOnly: fd.FilesOnly, DirsOnly: fd.DirsOnly, Invert: fd.Invert,
	}
	spec.Types, spec.TypeLabel = dialog.FilterTypesRegexp(fd.DirsOnly, fd.Types, h.host.Config().Filter.Types)
	return spec
}

// FilterDialogView is the Filter dialog's widget spec for the current terminal size.
func (h *Handler) FilterDialogView() dlg.Spec {
	termW, termH := h.screen.Size()
	return dialog.FilterDialogView(h.model.FilterDialog, dialog.Layout{Width: termW, Height: termH}, h.host.Styles())
}

func (h *Handler) filterDialogQueryWidth() int {
	termW, _ := h.screen.Size()
	return scrollquery.DialogInputWidthFromFrame(dialog.PatternDialogFrameWidth(termW))
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
	spec := h.filterSpec()
	if fd.Target != dialog.FilterTargetPanel && !spec.Empty() && h.tryRejectFilterDialogOK() {
		return
	}
	switch fd.Target {
	case dialog.FilterTargetCompare:
		h.executeCompareFilterDialog(spec)
		return
	case dialog.FilterTargetDedup:
		h.executeDedupFilterDialog(spec)
		return
	}
	p := h.host.ActivePanel()
	panelID := h.model.ActivePanel
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
	if fd.Focus == dialog.PatternInput {
		e := scrollquery.NewEdit(&fd.Text, &fd.TextCursor, &fd.TextScroll, h.filterDialogQueryWidth(), nil)
		e.Apply()
	}
	if h.tryRejectFilterDialogOK() {
		return
	}
	h.executeFilterDialog()
}

// HandleFilterDialogKey routes a key event for the open Filter dialog. Returns whether the app
// should quit (always false here). The engine owns focus and the mnemonics; the pattern input's
// text editing runs when the engine declines a key.
func (h *Handler) HandleFilterDialogKey(event *tcell.EventKey) bool {
	fd := &h.model.FilterDialog
	ev, ok := dlg.HandleKey(h.FilterDialogView(), &fd.Focus, event)
	switch {
	case !ok:
		h.editFilterPattern(event)
	case ev.ID == dlg.OK || ev.ID == dialog.PatternInput:
		h.confirmFilterDialogFromInput()
	case ev.ID == dlg.Cancel:
		h.CloseFilterDialog()
	case ev.ID == "":
	case event.Key() == tcell.KeyEnter && !dialog.PatternIsRadio(ev.ID):
		h.confirmFilterDialogFromInput() // Enter on a checkbox confirms; Space toggles it
	default:
		if fd.Apply(ev) {
			h.updateFilterDialogPreview()
		}
	}
	return false
}

// editFilterPattern feeds a key the engine declined to the pattern input's editor. Alt+letter
// keys that are mnemonics of the twin dialogs never edit, whether or not this dialog has them.
func (h *Handler) editFilterPattern(event *tcell.EventKey) {
	fd := &h.model.FilterDialog
	if fd.Focus != dialog.PatternInput {
		return
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) && dialog.PatternAltIsMnemonic(event.Rune()) {
		return
	}
	edit := scrollquery.NewEdit(&fd.Text, &fd.TextCursor, &fd.TextScroll, h.filterDialogQueryWidth(), h.updateFilterDialogPreview)
	scrollquery.HandleKey(h.keysDialogInput, event, true, edit)
}
