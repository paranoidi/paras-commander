package app

import (
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) paneHiddenVisibilityMessage(shown bool) string {
	respect := a.config.RespectGitignore && a.gitignoreCache != nil
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

// toggleHiddenGlobal flips hidden-file visibility for both panels together: it is a
// single global setting rather than per-panel, so any trigger (keybinding or panel menu)
// must keep both panels in sync.
func (a *App) toggleHiddenGlobal() error {
	if err := a.model.Primary.ToggleHidden(a.panelViewportRows(ui.PrimaryPanel)); err != nil {
		return err
	}
	if err := a.model.Secondary.ToggleHidden(a.panelViewportRows(ui.SecondaryPanel)); err != nil {
		return err
	}
	a.setTransientMessage(a.paneHiddenVisibilityMessage(a.model.Primary.ShowHidden), ui.MessageUrgencyInfo)
	return nil
}
