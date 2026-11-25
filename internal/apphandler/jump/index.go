package jump

import (
	"context"
	"strings"

	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// queryIndex runs the current query against the fileindex on a goroutine (JumpIndex); an
// empty query clears the list.
func (h *Handler) queryIndex() {
	st := &h.model.JumpDialog
	st.Err = ""
	if strings.TrimSpace(st.Query) == "" {
		st.Results, st.Truncated, st.Searching = nil, false, false
		st.Selected, st.ListScroll = 0, 0
		return
	}
	st.Searching = true
	gen, root, limit := h.rankGen, st.Root, h.config.UI.Find.MaxResults
	q := fi.FindQuery{Text: st.Query, Mode: findcrit.IndexPattern, OnlyDirs: true, MaxDepth: st.MaxDepth, IncludeHidden: st.IncludeHidden}
	index := h.index
	go func() {
		found, truncated, err := index.Find(context.Background(), root, q, limit)
		res := make([]dialog.JumpResult, len(found))
		for i, f := range found {
			res[i] = dialog.JumpResult{Path: f.Path, Rel: pathloc.RelOrSelf(root, f.Path)}
		}
		h.post(IndexResultPayload{Gen: gen, Results: res, Truncated: truncated, Err: err})
	}()
}

// HandleIndexResult applies a finished index query unless the dialog closed or the query
// changed since; it reports whether the dialog needs a repaint.
func (h *Handler) HandleIndexResult(p IndexResultPayload) bool {
	st := &h.model.JumpDialog
	if !st.Open || p.Gen != h.rankGen {
		return false
	}
	prev, _ := st.Cursor()
	st.Searching = false
	st.Results, st.Truncated = p.Results, p.Truncated
	if p.Err != nil {
		st.Err = p.Err.Error()
	}
	h.keepCursor(prev.Path)
	return true
}
