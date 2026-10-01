package dedup

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const dedupReturnButtons = 3 // Show/Selected, Rescan/Directory, Cancel

// HandleReturnDialogKey routes keys for the open kept-results / scan-scope dialog.
func (h *Handler) HandleReturnDialogKey(event *tcell.EventKey) {
	st := &h.model.DedupReturnDialog

	if dialog.AltDialogCancel(event) {
		h.returnDialogActivate(2)
		return
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			h.returnDialogActivate(0)
		case 'r', 'R':
			if len(st.ScopeDirs) == 0 {
				h.returnDialogActivate(1)
			}
		case 'd', 'D':
			if len(st.ScopeDirs) > 0 {
				h.returnDialogActivate(1)
			}
		}
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		h.returnDialogActivate(2)
	case tcell.KeyLeft:
		if st.ButtonFocus > 0 {
			st.ButtonFocus--
		}
	case tcell.KeyRight:
		if st.ButtonFocus < dedupReturnButtons-1 {
			st.ButtonFocus++
		}
	case tcell.KeyEnter:
		h.returnDialogActivate(st.ButtonFocus)
	}
}

// returnDialogActivate closes the dialog and runs button i: 0=Show, 1=Rescan, 2=Cancel, or
// in scope mode 0=Selected, 1=Directory, 2=Cancel.
func (h *Handler) returnDialogActivate(i int) {
	dirs := h.model.DedupReturnDialog.ScopeDirs
	h.model.DedupReturnDialog = dialog.DedupReturnDialogState{}
	if len(dirs) > 0 {
		switch i {
		case 0:
			root, only := scanScope(dirs)
			h.openRoot(root, only)
		case 1:
			h.openCurrent()
		}
		return
	}
	switch i {
	case 0:
		h.VerifyAndShowKept()
	case 1:
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
