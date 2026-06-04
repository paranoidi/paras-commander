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

func buildRunForEachItem(cmdTemplate string, ent localfs.Entry, active, other *panel.State, forceShell bool) (runForEachBuiltItem, error) {
	if !usermenu.CommandRequiresIteratedF(cmdTemplate) {
		return runForEachBuiltItem{}, errors.New(usermenu.ErrRunForEachRequiresF)
	}
	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template:   cmdTemplate,
		Mode:       cmdrun.ModeAuto,
		ForceShell: forceShell,
		Ctx:        usermenu.MacroContext(active, other, absPathClean(ent.Path), ""),
	})
	if err != nil {
		return runForEachBuiltItem{}, err
	}
	display := cmdTemplate
	if strings.TrimSpace(built.Expanded) != cmdTemplate {
		display = built.Display
	}
	return runForEachBuiltItem{
		Argv:     built.Argv,
		UserLine: display,
	}, nil
}
