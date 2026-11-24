package fileindex

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// BrowseTagsLoadedPayload carries the tag names Browse by tags offers.
type BrowseTagsLoadedPayload struct {
	Names []string
	Err   error
}

// OpenBrowseTags reads the tag names off the UI thread; HandleBrowseTagsLoaded opens Browse by
// tags over them. It works from any panel: a tag view covers the whole index.
func (h *Handler) OpenBrowseTags() {
	go func() {
		names, err := h.index.TagNames(context.Background())
		h.post(BrowseTagsLoadedPayload{Names: names, Err: err})
	}()
}

// HandleBrowseTagsLoaded opens Browse by tags, unless a dialog opened while the names loaded. In a
// tag view the dialog opens with that view's query; otherwise with the last Match and View used
// (All and Flat the first time). Tags the index no longer has drop out of the query.
func (h *Handler) HandleBrowseTagsLoaded(p BrowseTagsLoadedPayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	if h.model.AnyModalOpen() {
		return
	}
	if len(p.Names) == 0 {
		h.host.SetTransientMessage("No tags yet", ui.MessageUrgencyWarn)
		return
	}
	var checked []string
	anyTag, structure := h.browseAny, h.browseStructure
	if q, ok := h.host.FocusedPanel().Path.TagQuery(); ok {
		checked, anyTag, structure = q.Tags, q.Any, !q.Flat
	}
	h.model.IndexTagBrowseDialog = dialog.NewIndexTagBrowseDialog(p.Names, checked, anyTag, structure)
}

// BrowseTagsSpec is Browse by tags' widget spec for the current screen.
func (h *Handler) BrowseTagsSpec() dlg.Spec {
	return dialog.IndexTagBrowseDialogView(&h.model.IndexTagBrowseDialog, h.viewContext().Layout, h.host.Styles())
}

// HandleBrowseTagsKey handles one key while Browse by tags is open: the grid's jump keys, then the
// widget engine.
func (h *Handler) HandleBrowseTagsKey(ev *tcell.EventKey) {
	st := &h.model.IndexTagBrowseDialog
	if st.Focus == dialog.IndexTagsGrid && tagJumpKey(&st.IndexTagGrid, ev) {
		return
	}
	if e, ok := dlg.HandleKey(h.BrowseTagsSpec(), &st.Focus, ev); ok {
		h.ApplyBrowseTagsEvent(e)
		if st.Focus != dialog.IndexTagsGrid {
			st.ClearJump()
		}
	}
}

// ApplyBrowseTagsEvent applies a key or mouse event of Browse by tags.
func (h *Handler) ApplyBrowseTagsEvent(ev dlg.Event) {
	st := &h.model.IndexTagBrowseDialog
	switch {
	case st.ApplyEvent(ev):
	case ev.ID == dlg.OK:
		h.BrowseTagsOK()
	case ev.ID == dlg.Cancel:
		h.CloseBrowseTags()
	default:
		st.Apply(ev)
	}
}

// BrowseTagsEsc clears the grid's jump text, else closes the dialog.
func (h *Handler) BrowseTagsEsc() {
	st := &h.model.IndexTagBrowseDialog
	if st.JumpQuery != "" {
		st.ClearJump()
		return
	}
	h.CloseBrowseTags()
}

// CloseBrowseTags closes Browse by tags.
func (h *Handler) CloseBrowseTags() {
	h.model.IndexTagBrowseDialog = dialog.IndexTagBrowseDialogState{}
}

// BrowseTagsOK keeps the dialog open with a warning while no tag is checked. Otherwise it closes it,
// remembers Match and View for the session and looks the matches up off the UI thread; the focused
// panel then shows the tag view, or a warning says nothing is tagged so.
func (h *Handler) BrowseTagsOK() {
	st := h.model.IndexTagBrowseDialog
	tags := st.Checked()
	if len(tags) == 0 {
		h.host.SetTransientMessage("Check at least one tag", ui.MessageUrgencyWarn)
		return
	}
	h.browseAny, h.browseStructure = st.Any, st.Structure
	h.CloseBrowseTags()
	q := pathloc.TagQuery{Flat: !st.Structure, Any: st.Any, Tags: tags}
	panelID := h.model.FocusedPanelID()
	go func() {
		p := NavigatePayload{PanelID: panelID}
		start, matches, err := h.index.TagViewStart(context.Background(), q)
		switch {
		case err != nil:
			p.Err = err
		case matches == 0:
			p.Warn = "No entries tagged " + tagsText(q)
		default:
			if q.Flat {
				start = "/"
			}
			var loc pathloc.Path
			loc, p.Err = pathloc.TagView(q, start)
			p.Dir = loc.String()
		}
		h.post(p)
	}()
}

// tagsText names the tags of q the way the query reads: "beach & holiday", "beach | holiday".
func tagsText(q pathloc.TagQuery) string {
	sep := " & "
	if q.Any {
		sep = " | "
	}
	return strings.Join(q.Tags, sep)
}

// ToggleTagViewMode switches the active panel's tag view between a flat list and the directory
// structure. From the flat list the cursor row shows in its real directory with the row selected;
// from inside a row it is the same directory in the structure, cursor kept. From the structure it
// goes to the list with the cursor on the row of the cursor entry, which stays put when that entry
// is no match. Nothing here touches the disk.
func (h *Handler) ToggleTagViewMode() {
	p := h.host.FocusedPanel()
	q, ok := p.Path.TagQuery()
	if !ok {
		h.host.SetTransientMessage("Flat or structure works in a tag view", ui.MessageUrgencyWarn)
		return
	}
	entry, hasEntry := p.CurrentEntry()
	hasEntry = hasEntry && entry.Name != ".."
	entryHost := ""
	if hasEntry {
		_, entryHost, _ = pathloc.CutIndexPrefix(entry.Path)
	}
	wasList := p.Path.IsTagList()
	q.Flat = !q.Flat
	host, sel := "/", ""
	switch {
	case q.Flat: // from the structure: the list, the cursor on the row of the entry
		if hasEntry {
			if row, err := pathloc.TagView(q, entryHost); err == nil {
				sel = row.String()
			}
		}
	case wasList: // the row in its real directory
		if hasEntry {
			host, sel = filepath.Dir(entryHost), filepath.Base(entryHost)
		}
	default: // inside a row: the same directory
		_, host, _ = pathloc.CutIndexPrefix(p.Path.String())
		if hasEntry {
			sel = entry.Name
		}
	}
	loc, err := pathloc.TagView(q, host)
	if err == nil {
		err = h.host.NavigatePanelToPath(h.model.FocusedPanelID(), loc.String(), sel)
	}
	if err != nil {
		h.host.SetErrorMessage(messageTitle, err)
	}
}
