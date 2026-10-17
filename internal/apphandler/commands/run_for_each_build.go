package commands

import (
	"errors"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

// RunForEachBuiltItem is one resolved run-for-each invocation: the argv to execute and the
// display line shown in the Commands view.
type RunForEachBuiltItem struct {
	Argv     []string
	UserLine string
	Macros   []cmdmacro.MacroUse // display values of the template's macros
}

// BuildRunForEachItem expands cmdTemplate's macros for ent and returns the argv to run.
// When requireF is true, returns an error if the template omits the required %f (iterated
// item) macro; when false (run-in-each-directory mode), %f is optional since the shell is
// already positioned inside ent's directory.
func BuildRunForEachItem(cmdTemplate string, ent localfs.Entry, active, other *panel.State, forceShell bool, requireF bool) (RunForEachBuiltItem, error) {
	if requireF && !usermenu.CommandRequiresIteratedF(cmdTemplate) {
		return RunForEachBuiltItem{}, errors.New(usermenu.ErrRunForEachRequiresF)
	}
	ctx := usermenu.MacroContext(active, other, textutil.AbsPathClean(ent.Path), "")
	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template:   cmdTemplate,
		Mode:       cmdrun.ModeAuto,
		ForceShell: forceShell,
		Ctx:        ctx,
	})
	if err != nil {
		return RunForEachBuiltItem{}, err
	}
	display := cmdTemplate
	if strings.TrimSpace(built.Expanded) != cmdTemplate {
		display = built.Display
	}
	return RunForEachBuiltItem{
		Argv:     built.Argv,
		UserLine: display,
		Macros:   cmdmacro.MacroDisplayValues(cmdTemplate, ctx),
	}, nil
}
