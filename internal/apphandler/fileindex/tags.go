package fileindex

import (
	"fmt"
	"slices"

	"github.com/gdamore/tcell/v2"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// TagsLoadedPayload carries what the tag editor shows for Targets.
type TagsLoadedPayload struct {
	Targets []string
	State   fi.TagState
	Err     error
}

// TagsPayload carries the outcome of a tag edit: the tags added and removed, each with its result.
type TagsPayload struct {
	Added, Removed    []string
	AddRes, RemoveRes fi.TagEdit
	Recursive         bool
	Err               error
}

// OpenTagDialog reads the tags of the selection, else the cursor entry, else the panel directory,
// off the UI thread; HandleTagsLoaded opens the editor over them.
func (h *Handler) OpenTagDialog() {
	if !h.localOrIndex() {
		return
	}
	targets := h.targets()
	go func() {
		state, err := h.index.TagState(targets)
		h.post(TagsLoadedPayload{Targets: targets, State: state, Err: err})
	}()
}

// HandleTagsLoaded opens the tag editor, unless a dialog opened while the tags loaded.
func (h *Handler) HandleTagsLoaded(p TagsLoadedPayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	if h.model.AnyModalOpen() {
		return
	}
	h.model.IndexTagsDialog = dialog.NewIndexTagsDialog(p.Targets, p.State.Names, p.State.Counts, p.State.Targets, p.State.HasDir)
}

// TagDialogSpec is the tag editor's widget spec for the current screen.
func (h *Handler) TagDialogSpec() dlg.Spec {
	return dialog.IndexTagsDialogView(&h.model.IndexTagsDialog, h.viewContext().Layout, h.host.Styles())
}

// HandleTagKey handles one key while the tag editor is open: the grid's jump keys, the widget
// engine, then text editing in the input.
func (h *Handler) HandleTagKey(ev *tcell.EventKey) {
	st := &h.model.IndexTagsDialog
	if st.Focus == dialog.IndexTagsGrid && tagJumpKey(st, ev) {
		return
	}
	if e, ok := dlg.HandleKey(h.TagDialogSpec(), &st.Focus, ev); ok {
		h.ApplyTagEvent(e)
		if st.Focus != dialog.IndexTagsGrid {
			st.ClearJump()
		}
		return
	}
	if st.Focus == dialog.IndexTagsInput {
		w, hh := h.screen.Size()
		width := max(dlg.InputTextWidth(w, hh, h.TagDialogSpec(), dialog.IndexTagsInput), 1)
		edit := scrollquery.NewEdit(&st.Input, &st.InputCursor, &st.InputScroll, width, nil)
		scrollquery.HandleKey(h.keysDialogInput, ev, true, edit)
	}
}

// tagJumpKey applies the keys the grid keeps from the engine: Space toggles and ends the jump, a
// printable rune extends the jump text, and while there is jump text Up/Down cycle several matches
// and Backspace trims it. It reports whether the key was one of them.
func tagJumpKey(st *dialog.IndexTagsDialogState, ev *tcell.EventKey) bool {
	switch {
	case keymap.IsPlainPrintableRune(ev) && ev.Rune() == ' ':
		st.ToggleCursor()
		st.ClearJump()
	case keymap.IsPlainPrintableRune(ev):
		st.TypeJump(ev.Rune())
	case st.JumpQuery == "":
		return false
	case ev.Key() == tcell.KeyUp && st.CanCycleJump():
		st.CycleJump(-1)
	case ev.Key() == tcell.KeyDown && st.CanCycleJump():
		st.CycleJump(1)
	case ev.Key() == tcell.KeyBackspace, ev.Key() == tcell.KeyBackspace2:
		st.BackspaceJump()
	default:
		return false
	}
	return true
}

// ApplyTagEvent applies a key or mouse event of the tag editor.
func (h *Handler) ApplyTagEvent(ev dlg.Event) {
	st := &h.model.IndexTagsDialog
	switch {
	case ev.Kind == dlg.EventCaret:
		if ev.ID == dialog.IndexTagsInput {
			st.InputCursor = ev.Index
		}
	case ev.ID == dialog.IndexTagsGrid && ev.Kind == dlg.EventListMove:
		st.Cursor, st.Col, st.Offset = ev.Index, ev.Col, ev.Offset
		st.ClearJump()
	case ev.ID == dialog.IndexTagsGrid:
		if !ev.Enter { // a click: it lands on the cell it toggles
			st.Cursor, st.Col = ev.Index, ev.Col
		}
		st.ToggleCell(ev.Index, ev.Col)
		st.ClearJump()
	case ev.ID == dialog.IndexTagsInput:
		h.commitTagInput()
	case ev.ID == dialog.IndexTagsRecursive:
		st.Recursive = !st.Recursive
	case ev.ID == dlg.OK:
		h.TagDialogOK()
	case ev.ID == dlg.Cancel:
		h.CloseTagDialog()
	}
}

// commitTagInput stages the tags typed in the input and clears it; empty input does nothing.
func (h *Handler) commitTagInput() {
	st := &h.model.IndexTagsDialog
	tags := fi.ParseTags(st.Input)
	if len(tags) == 0 {
		return
	}
	st.AddTags(tags)
	st.Input, st.InputCursor, st.InputScroll = "", 0, 0
}

// TagDialogEsc clears the grid's jump text, else closes the editor.
func (h *Handler) TagDialogEsc() {
	st := &h.model.IndexTagsDialog
	if st.JumpQuery != "" {
		st.ClearJump()
		return
	}
	h.CloseTagDialog()
}

// CloseTagDialog closes the tag editor and drops its staged edits.
func (h *Handler) CloseTagDialog() { h.model.IndexTagsDialog = dialog.IndexTagsDialogState{} }

// TagDialogOK stages what is still in the input, closes the editor and writes the staged edits off
// the UI thread. With no edits it just closes.
func (h *Handler) TagDialogOK() {
	h.commitTagInput()
	st := h.model.IndexTagsDialog
	add, remove := st.Edits()
	h.CloseTagDialog()
	if len(add) == 0 && len(remove) == 0 {
		return
	}
	go func() {
		p := TagsPayload{Added: add, Removed: remove, Recursive: st.Recursive}
		if len(add) > 0 {
			p.AddRes, p.Err = h.index.EditTags(st.Targets, add, true, st.Recursive)
		}
		if len(remove) > 0 && p.Err == nil {
			p.RemoveRes, p.Err = h.index.EditTags(st.Targets, remove, false, st.Recursive)
		}
		h.post(p)
	}()
}

// HandleTags reports a finished tag edit and refreshes the tags column. When adding, a tag scan
// is queued that records and tags the paths the index lacks and samples the files of known paths
// that have neither a full hash nor a sample; a remove alone counts the unknown paths.
func (h *Handler) HandleTags(p TagsPayload) {
	h.host.RefreshMetaColumns()
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	msg, targeted := "", p.AddRes.Targeted
	if len(p.Added) > 0 {
		msg = fmt.Sprintf("Added %d %s", p.AddRes.Changed, jobbridge.Plural(p.AddRes.Changed, "tag", "tags"))
	}
	if len(p.Removed) > 0 {
		n := p.RemoveRes.Changed
		if msg == "" {
			msg = fmt.Sprintf("Removed %d %s", n, jobbridge.Plural(n, "tag", "tags"))
		} else {
			msg += fmt.Sprintf(", removed %d %s", n, jobbridge.Plural(n, "tag", "tags"))
		}
		targeted = max(targeted, p.RemoveRes.Targeted)
	}
	msg += fmt.Sprintf(" on %d %s", targeted, jobbridge.Plural(targeted, "entry", "entries"))
	if n := max(p.AddRes.OtherCopies, p.RemoveRes.OtherCopies); n > 0 {
		msg += fmt.Sprintf("; also %d other %s", n, jobbridge.Plural(n, "copy", "copies"))
	}
	if n := p.AddRes.SkippedEmpty; n > 0 {
		msg += fmt.Sprintf("; skipped %d empty %s", n, jobbridge.Plural(n, "file", "files"))
	}
	urgency := ui.MessageUrgencySuccess
	switch {
	case len(p.AddRes.Unknown) > 0 || len(p.AddRes.Unsampled) > 0:
		paths := slices.Concat(p.AddRes.Unknown, p.AddRes.Unsampled)
		h.queueTask("fileindex tag scan", h.index.ScanTagTask(paths, p.Added, p.Recursive), paths)
		if n := len(p.AddRes.Unknown); n > 0 {
			scanning := fmt.Sprintf("canning %d %s to tag", n, jobbridge.Plural(n, "entry", "entries"))
			if targeted == 0 {
				msg = "S" + scanning
			} else {
				msg += "; s" + scanning
			}
		}
		if n := len(p.AddRes.Unsampled); n > 0 {
			msg += fmt.Sprintf("; sampling %d %s", n, jobbridge.Plural(n, "entry", "entries"))
		}
		urgency = ui.MessageUrgencyInfo
	case len(p.RemoveRes.Unknown) > 0:
		msg += fmt.Sprintf("; %d not in the index", len(p.RemoveRes.Unknown))
		urgency = ui.MessageUrgencyWarn
	}
	h.host.SetTransientMessage(msg, urgency)
}
