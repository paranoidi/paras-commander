package app

import "fmt"

func (a *App) panelHiddenVisibilityMessage(label string, shown bool) string {
	respect := a.config.RespectGitignore && a.gitignoreCache != nil
	if shown {
		if respect {
			return fmt.Sprintf("%s hidden and ignored files shown", label)
		}
		return fmt.Sprintf("%s hidden files shown", label)
	}
	if respect {
		return fmt.Sprintf("%s hidden and ignored files hidden", label)
	}
	return fmt.Sprintf("%s hidden files hidden", label)
}
