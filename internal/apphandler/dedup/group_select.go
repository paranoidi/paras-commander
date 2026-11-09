package dedup

import "github.com/paranoidi/paras-commander/internal/ui"

// eachGroupSelectTarget calls fn for every file of the shown groups (ignore-empty and the Filter
// dialog's group filter applied) that match accepts and whose mark would flip: selecting skips
// kept and already marked files, unselecting visits only marked ones. It iterates snapshot groups,
// not visible rows, so files inside collapsed nodes count too.
func (h *Handler) eachGroupSelectTarget(match func(abs string) bool, selected bool, fn func(abs string, size int64)) {
	st := h.model.DedupView
	for _, g := range ui.DedupActiveGroups(h.model.DedupSnapshot, st) {
		for _, f := range g.Files {
			abs := f.Abs.String()
			if st.Marked[abs] == selected || (selected && st.Kept[abs]) || !match(abs) {
				continue
			}
			fn(abs, g.Size)
		}
	}
}

// GroupSelectCount is how many files GroupSelect would mark (selected) or unmark.
func (h *Handler) GroupSelectCount(match func(abs string) bool, selected bool) int {
	n := 0
	h.eachGroupSelectTarget(match, selected, func(string, int64) { n++ })
	return n
}

// GroupSelect is panel.select-group / panel.unselect-group in the dedup view: it marks (selected)
// or unmarks every shown file that match accepts and reports how many changed. Kept files are
// never marked.
func (h *Handler) GroupSelect(match func(abs string) bool, selected bool) int {
	n := 0
	h.eachGroupSelectTarget(match, selected, func(abs string, size int64) {
		h.setMark(abs, size, selected)
		n++
	})
	return n
}
