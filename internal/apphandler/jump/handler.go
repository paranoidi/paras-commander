package jump

import (
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const errTitle = "Jump to directory"

// Open opens the dialog on panelID: a disk walk on a local panel, an index query on a
// fileindex:// panel (the caller makes sure the index opens first). Every open starts afresh.
func (h *Handler) Open(panelID int) {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	h.Close()
	p := h.host.PanelByID(panelID)
	src := dialog.JumpDisk
	if p.Path.Scheme() == pathloc.SchemeFileIndex {
		src = dialog.JumpIndex
	}
	root, err := hostPath(p.Path)
	if err != nil {
		h.host.SetErrorMessage(errTitle, err)
		return
	}
	h.model.JumpDialog = dialog.JumpDialogState{
		Open: true, PanelID: panelID, Source: src, Root: root, StayOnVolume: true, Focus: dialog.JumpQuery,
	}
	if src == dialog.JumpIndex {
		return // an empty query lists nothing; typing runs the first query
	}
	st := &h.model.JumpDialog
	if h.host.GitignoreCache().DirIgnored(root) {
		// The panel directory itself is git-ignored: a plain walk would find nothing.
		st.IncludeHidden = true
		h.host.SetTransientMessage("Explicit search on gitignore allowed", ui.MessageUrgencyInfo)
	}
	h.startScan()
}

func hostPath(p pathloc.Path) (string, error) {
	if p.Scheme() == pathloc.SchemeFileIndex {
		return pathloc.IndexHostPath(p)
	}
	return p.FilePath()
}

// Close closes the dialog, stops its walk and drops any match or query in flight.
func (h *Handler) Close() {
	h.stopScan()
	h.stopRankTimer()
	h.rankGen++
	h.model.JumpDialog = dialog.JumpDialogState{}
}

// DialogSpec is the open dialog's widget spec for the current screen.
func (h *Handler) DialogSpec() dlg.Spec {
	return dialog.JumpDialogView(&h.model.JumpDialog, h.viewContext())
}

func (h *Handler) viewContext() dialog.JumpViewContext {
	w, hh := h.screen.Size()
	return dialog.JumpViewContext{
		Layout: h.host.LayoutForTerminalSize(w, hh), Styles: h.host.Styles(),
		ShowIcons: h.model.UseNerdfontIcons, ScrollbarStyle: h.model.PanelScrollbar,
	}
}

func (h *Handler) listRows() int {
	return max(dialog.JumpListRows(h.viewContext().Layout, &h.model.JumpDialog), 1)
}

func (h *Handler) queryWidth() int {
	w, hh := h.screen.Size()
	return max(dlg.InputTextWidth(w, hh, h.DialogSpec(), dialog.JumpQuery), 1)
}

// HandleKey handles one key while the dialog is open: the opener key closes it, pin and the
// open-in-panel keys act on the cursor row, then the widget engine, the depth digits and the
// query editing.
func (h *Handler) HandleKey(ev *tcell.EventKey) {
	st := &h.model.JumpDialog
	if id, ok := h.keys.Lookup(ev); ok {
		switch id {
		case keymap.ActionPanelJumpDialog:
			h.Close()
			return
		case keymap.ActionPanelPinToggle:
			h.pinCursor()
			return
		}
	}
	if id, ok := h.keysJump.Lookup(ev); ok {
		switch id {
		case keymap.ActionOpenInPrimary:
			h.openCursorIn(h.model.FirstSlotPanel())
			return
		case keymap.ActionOpenInSecondary:
			h.openCursorIn(h.model.SecondSlotPanel())
			return
		}
	}
	if st.Focus == dialog.JumpDepth {
		switch ev.Key() {
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyUp, tcell.KeyDown:
			h.commitDepth()
		case tcell.KeyLeft:
			h.commitDepth()
			st.Focus = dialog.JumpQuery
			return
		}
	}
	if e, ok := dlg.HandleKey(h.DialogSpec(), &st.Focus, ev); ok {
		h.ApplyEvent(e)
		return
	}
	if st.Focus == dialog.JumpDepth {
		h.handleDepthKey(ev)
		return
	}
	if st.Focus == dialog.JumpQuery {
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, h.queryWidth(), func() {
			st.Selected, st.ListScroll = 0, 0
			h.queryChanged()
		})
		scrollquery.HandleKey(h.keysDialogInput, ev, true, edit)
	}
}

// ApplyEvent acts on a widget event (key or click).
func (h *Handler) ApplyEvent(ev dlg.Event) {
	st := &h.model.JumpDialog
	switch {
	case ev.Kind == dlg.EventCaret:
		if ev.ID == dialog.JumpQuery {
			st.QueryCursor = ev.Index
		}
	case ev.ID == dialog.JumpList && ev.Kind == dlg.EventListMove:
		st.Selected, st.ListScroll = ev.Index, ev.Offset
	case ev.ID == "":
	case ev.ID == dialog.JumpDepth:
		h.commitDepth()
		st.Focus = dialog.JumpQuery
	case ev.ID == dialog.JumpQuery, ev.ID == dlg.OK:
		h.Activate()
	case ev.ID == dlg.Cancel:
		h.Close()
	case ev.ID == dialog.JumpHidden:
		h.toggleHidden()
	case ev.ID == dialog.JumpStayOnVol:
		h.toggleStayOnVolume()
	}
}

// Activate is OK / Enter: the panel changes to the directory under the cursor and the dialog
// closes.
func (h *Handler) Activate() {
	panelID := h.model.JumpDialog.PanelID
	if _, ok := h.navigateTo(panelID); !ok {
		return
	}
	h.model.ActivePanel = panelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.Close()
}

// navigateTo points panelID at the directory under the cursor (a fileindex:// path when the
// dialog searches the index); it does not close the dialog.
func (h *Handler) navigateTo(panelID int) (string, bool) {
	st := &h.model.JumpDialog
	r, ok := st.Cursor()
	if !ok {
		return "", false
	}
	if st.Source == dialog.JumpIndex {
		if !host.ShowInPanel(h.host, panelID, r.Path, "", true, errTitle) {
			return "", false
		}
	} else {
		if err := h.host.NavigatePanelToPath(panelID, r.Path, ""); err != nil {
			h.host.SetErrorMessage(errTitle, err)
			return "", false
		}
		h.host.PanelByID(panelID).EnsureCursorVisible(h.host.PanelViewportRows(panelID))
	}
	return filepath.Base(r.Path), true
}

func (h *Handler) openCursorIn(panelID int) {
	if name, ok := h.navigateTo(panelID); ok {
		host.ReportOpenedInPanel(h.host, name, panelID)
	}
}

func (h *Handler) pinCursor() {
	r, ok := h.model.JumpDialog.Cursor()
	if !ok {
		return
	}
	h.host.PinTogglePath(filepath.Base(r.Path), h.model.JumpDialog.PanelPath(r), true)
}

// commitDepth applies the depth edit buffer (one digit 1-9, "" = unlimited).
func (h *Handler) commitDepth() {
	st := &h.model.JumpDialog
	next := 0
	if st.DepthInput != "" {
		next = int(st.DepthInput[0] - '0')
	}
	old := st.MaxDepth
	if next == old {
		return
	}
	st.MaxDepth = next
	if st.Source == dialog.JumpIndex {
		h.queryNow()
		return
	}
	if next != 0 && (old == 0 || next < old) {
		h.scan.SetMaxDepth(next) // prunes deeper directories; IndexReplaced re-ranks
		return
	}
	h.startScan() // a looser limit rescans
}

// handleDepthKey edits the depth field: 1-9 sets it, 0 / Backspace / Delete clear it, and
// either commits it and returns focus to the query.
func (h *Handler) handleDepthKey(ev *tcell.EventKey) {
	st := &h.model.JumpDialog
	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		st.DepthInput = ""
	case tcell.KeyRune:
		if ev.Modifiers() != tcell.ModNone {
			return
		}
		switch r := ev.Rune(); {
		case r >= '1' && r <= '9':
			st.DepthInput = string(r)
		case r == '0':
			st.DepthInput = ""
		default:
			return
		}
	default:
		return
	}
	h.commitDepth()
	st.Focus = dialog.JumpQuery
}

func (h *Handler) toggleHidden() {
	st := &h.model.JumpDialog
	st.IncludeHidden = !st.IncludeHidden
	if st.Source == dialog.JumpIndex {
		h.queryNow()
		return
	}
	st.Scanning = true
	h.scan.SetIncludeHidden(st.IncludeHidden)
}

func (h *Handler) toggleStayOnVolume() {
	st := &h.model.JumpDialog
	if st.Source != dialog.JumpDisk {
		return
	}
	st.StayOnVolume = !st.StayOnVolume
	h.startScan()
}

// queryChanged reacts to an edited query after the [ui.find] query debounce.
func (h *Handler) queryChanged() {
	h.rankGen++ // results for the previous query are stale from now on
	h.model.JumpDialog.Searching = true
	h.armRankTimer(time.Duration(h.config.UI.Find.QueryDebounceMS) * time.Millisecond)
}

// queryNow runs the query at once (an option changed).
func (h *Handler) queryNow() {
	h.stopRankTimer()
	h.rankGen++
	h.dispatch()
}

func (h *Handler) armRankTimer(delay time.Duration) {
	h.stopRankTimer()
	h.rankSeq++
	seq := h.rankSeq
	h.rankTimer = time.AfterFunc(delay, func() { h.post(RankDuePayload{Seq: seq}) })
}

func (h *Handler) stopRankTimer() {
	if h.rankTimer != nil {
		h.rankTimer.Stop()
		h.rankTimer = nil
	}
	h.rankSeq++ // a wake a stopped timer already posted is dropped
}

// HandleRankDue runs the due match or query; it reports whether the dialog needs a repaint.
func (h *Handler) HandleRankDue(p RankDuePayload) bool {
	if p.Seq != h.rankSeq || !h.model.JumpDialog.Open {
		return false
	}
	h.rankTimer = nil
	h.dispatch()
	return true
}

// dispatch asks the current source for the current query's results.
func (h *Handler) dispatch() {
	if h.model.JumpDialog.Source == dialog.JumpIndex {
		h.queryIndex()
		return
	}
	h.requestMatch()
}

// keepCursor moves the cursor to the row of path in the new results, else keeps it in range.
func (h *Handler) keepCursor(path string) {
	st := &h.model.JumpDialog
	if path != "" {
		for i, r := range st.Results {
			if r.Path == path {
				st.Selected = i
				break
			}
		}
	}
	dialog.EnsureJumpListScroll(st, h.listRows())
}
