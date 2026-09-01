package pin

import (
	"fmt"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// Toggle adds path to the pin list (prepended, most-recent-first) if it isn't already
// pinned, or removes it if it is (toggle semantics — an accidental double-pin can't create
// duplicates). Returns true when the item was added, false when removed.
func (h *Handler) Toggle(path string, isDir bool) bool {
	path = filepath.Clean(path)
	for i, it := range h.model.PinnedItems {
		if it.Path == path {
			h.model.PinnedItems = append(h.model.PinnedItems[:i], h.model.PinnedItems[i+1:]...)
			return false
		}
	}
	h.model.PinnedItems = append([]ui.PinnedItem{{Path: path, IsDir: isDir}}, h.model.PinnedItems...)
	return true
}

// TogglePath toggles path and shows a transient "Pinned"/"Unpinned" status message using
// name (the entry's display basename).
func (h *Handler) TogglePath(name, path string, isDir bool) {
	if h.Toggle(path, isDir) {
		h.host.SetTransientMessage(fmt.Sprintf("Pinned %s", name), ui.MessageUrgencyInfo)
	} else {
		h.host.SetTransientMessage(fmt.Sprintf("Unpinned %s", name), ui.MessageUrgencyInfo)
	}
}

// ToggleActivePanelCursor pins/unpins the active panel's cursor entry.
func (h *Handler) ToggleActivePanelCursor() {
	entry, ok := h.host.ActivePanel().CurrentEntry()
	if !ok {
		h.host.SetTransientMessage("No entry to pin", ui.MessageUrgencyInfo)
		return
	}
	h.TogglePath(entry.Name, entry.Path, entry.IsDir())
}

// ToggleCompareSelection pins/unpins the Compare view's focused-column selection. Compare
// rows are always files.
func (h *Handler) ToggleCompareSelection() {
	path, ok := h.compare.SelectedColumnPinTarget()
	if !ok {
		h.host.SetTransientMessage("No entry to pin", ui.MessageUrgencyInfo)
		return
	}
	h.TogglePath(filepath.Base(path), path, false)
}

// ToggleDedupSelection pins/unpins the Dedup view's focused-pane selection (file or dir row).
func (h *Handler) ToggleDedupSelection() {
	path, isDir, ok := h.dedup.SelectedPinTarget()
	if !ok {
		h.host.SetTransientMessage("No entry to pin", ui.MessageUrgencyInfo)
		return
	}
	h.TogglePath(filepath.Base(path), path, isDir)
}
