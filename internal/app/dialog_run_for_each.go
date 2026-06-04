package app

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) recomputeRunForEachCommandValidation() {
	d := &a.model.FileDialog
	if !d.Open || d.DialogType != ui.FileDialogRunForEach || len(d.Fields) == 0 {
		return
	}
	msg := validateRunForEachCommand(
		strings.TrimSpace(d.Fields[0].Value),
		d.RunForEachEntries,
		a.activePanel(),
		a.inactivePanel(),
	)
	d.RunForEachCommandError = msg
	d.Fields[0].InputInvalid = msg != ""
}

func validateRunForEachCommand(cmdLine string, entries []localfs.Entry, active, other *panel.State) string {
	if cmdLine == "" {
		return "Command is empty"
	}
	ent := localfs.Entry{}
	if len(entries) > 0 {
		ent = entries[0]
	} else if active != nil {
		if e, ok := active.CurrentEntry(); ok {
			ent = e
		}
	}
	if ent.Path == "" && active != nil {
		ent.Path = active.PathString()
	}
	if _, err := buildRunForEachItem(cmdLine, ent, active, other, false); err != nil {
		return err.Error()
	}
	return ""
}
