package dedup

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// ReturnDialogCancel dismisses the kept-results / scan-scope dialog (its Cancel button).
func (h *Handler) ReturnDialogCancel() { h.returnDialogActivate(dlg.Cancel) }

// HandleReturnDialogKey routes keys for the open kept-results / scan-scope dialog.
func (h *Handler) HandleReturnDialogKey(event *tcell.EventKey) {
	st := &h.model.DedupReturnDialog
	// The summary only shapes the message lines, not the buttons or their mnemonics.
	if ev, ok := dlg.HandleKey(dialog.DedupReturnView(*st, "", 0, 0, ""), &st.Focus, event); ok && ev.ID != "" {
		h.returnDialogActivate(ev.ID)
	}
}

// returnDialogActivate closes the dialog and runs button id: First is Show (Selected in scope
// mode), Second is Rescan (Directory), dlg.Cancel just closes.
func (h *Handler) returnDialogActivate(id dlg.ID) {
	dirs := h.model.DedupReturnDialog.ScopeDirs
	h.model.DedupReturnDialog = dialog.DedupReturnDialogState{}
	if len(dirs) > 0 {
		switch id {
		case dialog.DedupReturnFirst:
			root, only := scanScope(dirs)
			h.openRoot(root, only)
		case dialog.DedupReturnSecond:
			h.openCurrent()
		}
		return
	}
	switch id {
	case dialog.DedupReturnFirst:
		h.VerifyAndShowKept()
	case dialog.DedupReturnSecond:
		h.openRoot(h.activePanelPath(), nil)
	}
}

// scanScope roots a selected-directories scan at the dirs' deepest common ancestor and
// limits it to the dirs (relative to that root); a single dir is scanned as the root.
func scanScope(dirs []string) (pathloc.Path, []string) {
	dirs = panel.PruneNestedPaths(dirs)
	root := dirs[0]
	for _, d := range dirs[1:] {
		for root != filepath.Dir(root) && !strings.HasPrefix(d, root+string(filepath.Separator)) {
			root = filepath.Dir(root)
		}
	}
	var only []string
	if len(dirs) > 1 {
		for _, d := range dirs {
			if rel, err := filepath.Rel(root, d); err == nil {
				only = append(only, filepath.ToSlash(rel))
			}
		}
	}
	loc, _ := pathloc.File(root)
	return loc, only
}
