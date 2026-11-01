package find

import (
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// The advanced section (match mode and criteria) is parsed into h.criteria / h.exclude, which
// the match requests and walk options read. Checkbox and radio changes apply at once; the text
// inputs apply on Enter and when focus leaves them (commitAdvanced), since an owner lookup or new
// exclude patterns cost more than a keystroke should.

// matchMode is the query match mode in effect (jump mode is always fuzzy).
func (h *Handler) matchMode(st *dialog.FindDialogState) findcrit.Mode {
	if st.JumpMode {
		return findcrit.ModeFuzzy
	}
	return st.Advanced.Mode
}

// caseInsensitive reports whether the query ignores case: [filter] case_insensitive unless the
// advanced Case sensitive box is checked.
func (h *Handler) caseInsensitive(st *dialog.FindDialogState) bool {
	return h.config.Filter.CaseInsensitive && (st.JumpMode || !st.Advanced.CaseSensitive)
}

// findFilterEmpty reports whether the dialog filters nothing: an empty query and no criteria. The
// result list then shows the index in walk order from the UI mirror, without a match pass.
func (h *Handler) findFilterEmpty(st *dialog.FindDialogState) bool {
	return findcrit.QueryEmpty(st.Query, h.matchMode(st)) && !h.criteria.Active()
}

// validateQuery flags a glob or regex query that does not compile; the match pass returns
// nothing for it.
func (h *Handler) validateQuery(st *dialog.FindDialogState) {
	_, err := findcrit.Compile(st.Query, h.matchMode(st), h.caseInsensitive(st))
	st.QueryInvalid = err != nil
}

// parseAdvanced turns the advanced section into criteria and exclude patterns, flagging the
// fields that do not parse (they are left out). Jump mode has no criteria.
func (h *Handler) parseAdvanced(st *dialog.FindDialogState) (findcrit.Criteria, findcrit.Excludes) {
	if st.JumpMode {
		return findcrit.Criteria{}, nil
	}
	a := &st.Advanced
	now := time.Now()
	age := func(f dialog.FindAdvField) int64 {
		v, err := findcrit.ParseAge(a.Fields[f].Value, now)
		a.Fields[f].Invalid = err != nil
		return v
	}
	size := func(f dialog.FindAdvField) (int64, bool) {
		v, ok, err := findcrit.ParseSize(a.Fields[f].Value)
		a.Fields[f].Invalid = err != nil
		return v, ok
	}
	c := findcrit.Criteria{
		Modified:       findcrit.TimeBound{Newer: age(dialog.FindAdvModifiedNewer), Older: age(dialog.FindAdvModifiedOlder)},
		Changed:        findcrit.TimeBound{Newer: age(dialog.FindAdvChangedNewer), Older: age(dialog.FindAdvChangedOlder)},
		Accessed:       findcrit.TimeBound{Newer: age(dialog.FindAdvAccessedNewer), Older: age(dialog.FindAdvAccessedOlder)},
		Empty:          a.Empty,
		Symlinks:       a.Symlinks,
		BrokenSymlinks: a.Broken,
		Executables:    a.Executables,
		NotMine:        a.NotMine,
		MyUID:          uint32(os.Getuid()),
	}
	c.MinSize, _ = size(dialog.FindAdvSizeMin)
	c.MaxSize, c.HasMaxSize = size(dialog.FindAdvSizeMax)
	owner := &a.Fields[dialog.FindAdvOwner]
	uid, err := localfs.LookupUser(strings.TrimSpace(owner.Value))
	owner.Invalid = err != nil
	if err == nil && uid >= 0 {
		c.Owner, c.HasOwner = uint32(uid), true
	}
	c.Types, _ = dialog.FilterTypesRegexp(false, a.Types, h.config.Filter.Types)
	excl, err := findcrit.ParseExcludes(a.Fields[dialog.FindAdvExclude].Value)
	a.Fields[dialog.FindAdvExclude].Invalid = err != nil
	return c, excl
}

// validateAdvField re-checks field f while it is edited (the owner is checked on commit).
func validateAdvField(a *dialog.FindAdvancedState, f dialog.FindAdvField) {
	in := &a.Fields[f]
	switch {
	case f.IsAge():
		in.Invalid = !findcrit.ValidAge(in.Value)
	case f.IsSize():
		in.Invalid = !findcrit.ValidSize(in.Value)
	case f == dialog.FindAdvExclude:
		_, err := findcrit.ParseExcludes(in.Value)
		in.Invalid = err != nil
	default:
		in.Invalid = false
	}
}

// loadAdvanced parses the advanced section without touching the index (dialog open).
func (h *Handler) loadAdvanced(st *dialog.FindDialogState) {
	h.criteria, h.exclude = h.parseAdvanced(st)
	h.validateQuery(st)
}

// commitAdvanced applies the advanced section: criteria needing metadata the index lacks, or
// changed exclude patterns, restart the index; anything else re-runs the match.
func (h *Handler) commitAdvanced() {
	st := &h.model.FindDialog
	h.advEditing = ""
	if !st.Open {
		return
	}
	h.loadAdvanced(st)
	if h.criteria.NeedsStat() > st.IndexStat || strings.Join(h.exclude, ",") != st.IndexExclude {
		h.restartFindIndexer()
		return
	}
	h.clearFindNavIdle()
	h.syncFindDialogRanks()
}

// commitLeftAdvField commits the advanced text input being edited once focus has left it.
func (h *Handler) commitLeftAdvField() {
	if h.advEditing != "" && h.model.FindDialog.Focus != h.advEditing {
		h.commitAdvanced()
	}
}

// editAdvField hands a key the engine declined to the focused advanced text input.
func (h *Handler) editAdvField(event *tcell.EventKey, f dialog.FindAdvField) {
	st := &h.model.FindDialog
	in := &st.Advanced.Fields[f]
	id := dialog.FindAdvFieldID(f)
	termW, termH := h.screen.Size()
	width := max(dlg.InputTextWidth(termW, termH, h.DialogSpec(), id), 1)
	edit := scrollquery.NewEdit(&in.Value, &in.Cursor, &in.Scroll, width, func() {
		validateAdvField(&st.Advanced, f)
		h.advEditing = id
	})
	h.host.HandleScrollingQueryKey(event, true, NewScrollingQueryEdit(edit))
}

// ToggleAdvanced shows or hides the advanced section. Hiding it moves focus that was inside
// the section back to the query.
func (h *Handler) ToggleAdvanced() {
	st := &h.model.FindDialog
	if st.JumpMode {
		return
	}
	st.Advanced.Expanded = !st.Advanced.Expanded
	if !st.Advanced.Expanded && advancedSectionID(st.Focus) {
		h.commitLeftAdvField()
		st.Focus = dialog.FindQuery
	}
	dialog.EnsureFindListScroll(st, h.findDialogListRows())
}

// CycleMatchMode switches the query to the next match mode (fuzzy, glob, regex).
func (h *Handler) CycleMatchMode() {
	st := &h.model.FindDialog
	if st.JumpMode {
		return
	}
	h.setMatchMode(st.Advanced.Mode.Next())
}

func (h *Handler) setMatchMode(m findcrit.Mode) {
	st := &h.model.FindDialog
	st.Advanced.Mode = m
	st.Selected = 0
	h.commitAdvanced()
}

// ResetAdvanced clears the match mode and every criterion.
func (h *Handler) ResetAdvanced() {
	st := &h.model.FindDialog
	if st.JumpMode {
		return
	}
	st.Advanced.Clear()
	h.commitAdvanced()
}

// advancedSectionID reports whether id is a widget of the advanced section.
func advancedSectionID(id dlg.ID) bool {
	if _, ok := dialog.FindAdvFieldOf(id); ok {
		return true
	}
	if _, ok := dialog.FindAdvModeOf(id); ok {
		return true
	}
	if _, ok := dialog.FindAdvTypeOf(id); ok {
		return true
	}
	switch id {
	case dialog.FindAdvEmpty, dialog.FindAdvSymlinks, dialog.FindAdvBroken, dialog.FindAdvExecutable,
		dialog.FindAdvNotMine, dialog.FindAdvCase:
		return true
	}
	return false
}

// applyAdvancedEvent acts on an advanced-section widget event; it reports false for any other
// widget.
func (h *Handler) applyAdvancedEvent(ev dlg.Event) bool {
	st := &h.model.FindDialog
	a := &st.Advanced
	if ev.ID == dialog.FindAdvanced {
		h.ToggleAdvanced()
		return true
	}
	if f, ok := dialog.FindAdvFieldOf(ev.ID); ok {
		if ev.Kind == dlg.EventCaret {
			a.Fields[f].Cursor = ev.Index
			return true
		}
		h.commitAdvanced() // Enter in the input
		return true
	}
	if m, ok := dialog.FindAdvModeOf(ev.ID); ok {
		h.setMatchMode(m)
		return true
	}
	if i, ok := dialog.FindAdvTypeOf(ev.ID); ok {
		a.Types[i] = !a.Types[i]
		h.commitAdvanced()
		return true
	}
	flag := map[dlg.ID]*bool{
		dialog.FindAdvEmpty: &a.Empty, dialog.FindAdvSymlinks: &a.Symlinks, dialog.FindAdvBroken: &a.Broken,
		dialog.FindAdvExecutable: &a.Executables, dialog.FindAdvNotMine: &a.NotMine, dialog.FindAdvCase: &a.CaseSensitive,
	}[ev.ID]
	if flag == nil {
		return false
	}
	*flag = !*flag
	h.commitAdvanced()
	return true
}
