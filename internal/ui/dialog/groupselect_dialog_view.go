package dialog

import (
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// GroupSelectView is the Select / Unselect group dialog's widget spec; the layout and the shared
// body come from patternSpec (the Filter dialog's twin, FilterDialogView). Match full path is
// shown in the find and output contexts only, Invert never.
func GroupSelectView(s GroupSelectState, layout Layout, styles theme.Theme) dlg.Spec {
	title := "Unselect group"
	if s.Mode == "select" {
		title = "Select group"
	}
	return patternSpec(patternView{
		Title: title, LayoutWidth: layout.Width,
		Mode: s.PatternMode, Types: s.Types, FilesOnly: s.FilesOnly, DirsOnly: s.DirsOnly, CaseSensitive: s.CaseSensitive,
		ShowFilesDirs: true, ShowFullPath: GroupSelectShowsFullPath(s), FullPath: s.FullPath,
		MetaCount: s.MetaColumnCount, IncludeMeta: s.IncludeMetaColumns, OnlyMeta: s.OnlyMetaColumns,
		Text: s.Text, Cursor: s.TextCursor, Scroll: s.TextScroll,
		Invalid: groupSelectPatternInvalid(s), Hint: groupSelectPatternHintText(s),
		Preview: patternPreviewText(s.PreviewShow, s.PreviewFiles, s.PreviewFolders, styles),
	})
}

// toggles exposes the shared option fields to patternToggles.
func (s *GroupSelectState) toggles() patternToggles {
	return patternToggles{
		Mode: &s.PatternMode, Types: &s.Types, FilesOnly: &s.FilesOnly, DirsOnly: &s.DirsOnly, Case: &s.CaseSensitive,
		FullPath: &s.FullPath, IncludeMeta: &s.IncludeMetaColumns, OnlyMeta: &s.OnlyMetaColumns,
	}
}

// Apply folds an activation from dlg.HandleKey / dlg.Click into the state: a radio selects its
// pattern mode, a checkbox toggles. It reports whether anything changed. OK, Cancel and the
// pattern input are the caller's.
func (s *GroupSelectState) Apply(ev dlg.Event) bool { return s.toggles().apply(ev.ID) }
