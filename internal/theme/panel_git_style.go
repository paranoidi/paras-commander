package theme

import "github.com/gdamore/tcell/v2"

// PanelGitStyle returns foreground style for one eza-style Git status glyph.
func (t Theme) PanelGitStyle(statusKey string) tcell.Style {
	switch statusKey {
	case "panel.git.new":
		return t.PanelGitNew
	case "panel.git.modified":
		return t.PanelGitModified
	case "panel.git.deleted":
		return t.PanelGitDeleted
	case "panel.git.renamed":
		return t.PanelGitRenamed
	case "panel.git.typechange":
		return t.PanelGitTypechange
	case "panel.git.ignored":
		return t.PanelGitIgnored
	case "panel.git.conflicted":
		return t.PanelGitConflicted
	default:
		return t.PanelGitNotModified
	}
}
