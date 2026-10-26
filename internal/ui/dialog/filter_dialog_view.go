package dialog

import (
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// FilterDialogView is the Filter dialog's widget spec; the layout and the shared body come from
// patternSpec (the Select group dialog's twin, GroupSelectView). The layout picks the width at
// which the live preview still fits beside the Pattern label, the theme supplies its icons.
func FilterDialogView(s FilterDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	hint := filterPatternHintText(s)
	return patternSpec(patternView{
		Title: "Filter", LayoutWidth: layout.Width,
		Mode: s.PatternMode, Types: s.Types, FilesOnly: s.FilesOnly, DirsOnly: s.DirsOnly, CaseSensitive: s.CaseSensitive,
		ShowFilesDirs: FilterShowsFilesDirs(s), ShowInvert: true, Invert: s.Invert,
		MetaCount: s.MetaColumnCount, IncludeMeta: s.IncludeMetaColumns, OnlyMeta: s.OnlyMetaColumns,
		Text: s.Text, Cursor: s.TextCursor, Scroll: s.TextScroll,
		Invalid: filterPatternInvalid(s), Hint: hint,
		Preview: patternPreviewText(s.PreviewShow, s.PreviewFiles, s.PreviewFolders, styles),
	})
}

// toggles exposes the shared option fields to patternToggles.
func (s *FilterDialogState) toggles() patternToggles {
	return patternToggles{
		Mode: &s.PatternMode, Types: &s.Types, FilesOnly: &s.FilesOnly, DirsOnly: &s.DirsOnly, Case: &s.CaseSensitive,
		Invert: &s.Invert, IncludeMeta: &s.IncludeMetaColumns, OnlyMeta: &s.OnlyMetaColumns,
	}
}

// Apply folds an activation from dlg.HandleKey into the state: a radio selects its
// pattern mode, a checkbox toggles. It reports whether anything changed (the caller then
// recomputes the live preview). OK, Cancel and the pattern input are the caller's.
func (s *FilterDialogState) Apply(ev dlg.Event) bool { return s.toggles().apply(ev.ID) }
