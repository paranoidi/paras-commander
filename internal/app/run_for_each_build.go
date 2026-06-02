package app

import (
	"errors"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

type runForEachBuiltItem struct {
	Argv     []string
	UserLine string
}

func buildRunForEachItem(cmdTemplate string, ent localfs.Entry, active, other *panel.State) (runForEachBuiltItem, error) {
	if !usermenu.CommandRequiresIteratedF(cmdTemplate) {
		return runForEachBuiltItem{}, errors.New(usermenu.ErrRunForEachRequiresF)
	}
	expanded, err := usermenu.ExpandCommandWithFOverride(cmdTemplate, active, other, absPathClean(ent.Path))
	if err != nil {
		return runForEachBuiltItem{}, err
	}
	display := cmdTemplate
	if strings.TrimSpace(expanded) != cmdTemplate {
		display = cmdTemplate + " → " + expanded
	}
	if cmdrun.NeedsShellFromLine(expanded) {
		return runForEachBuiltItem{
			Argv:     cmdrun.ShellArgv(expanded),
			UserLine: display,
		}, nil
	}
	argv, err := cmdrun.ParseCommandArgv(expanded)
	if err != nil {
		return runForEachBuiltItem{}, err
	}
	return runForEachBuiltItem{
		Argv:     argv,
		UserLine: display,
	}, nil
}
