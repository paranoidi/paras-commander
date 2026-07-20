package app

import (
	"errors"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) paneHiddenVisibilityMessage(shown bool) string {
	respect := a.config.Panels.RespectGitignore && a.gitignoreCache != nil
	if shown {
		if respect {
			return "Hidden and ignored files shown"
		}
		return "Hidden files shown"
	}
	if respect {
		return "Hidden and ignored files hidden"
	}
	return "Hidden files hidden"
}

// toggleHiddenGlobal flips hidden-file visibility as a single global setting: the new
// value is derived from the active panel and assigned to both panels, so they converge
// even if a previous reload error left them diverged. Both panels are always updated;
// a reload error on one does not skip the other.
func (a *App) toggleHiddenGlobal() error {
	shown := !a.activePanel().ShowHidden
	errPrimary := a.model.Primary.SetShowHidden(shown, a.panelViewportRows(ui.PrimaryPanel))
	errSecondary := a.model.Secondary.SetShowHidden(shown, a.panelViewportRows(ui.SecondaryPanel))
	if err := errors.Join(errPrimary, errSecondary); err != nil {
		return err
	}
	a.setTransientMessage(a.paneHiddenVisibilityMessage(shown), ui.MessageUrgencyInfo)
	return nil
}
