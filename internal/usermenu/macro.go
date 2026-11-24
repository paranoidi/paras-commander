package usermenu

import (
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// EvalContext carries panel state for condition evaluation.
type EvalContext = entrymatch.Context

// ExpandCommand substitutes % macros in a single command line for argv parsing.
func ExpandCommand(cmd string, active, other *panel.State) (string, error) {
	return ExpandCommandWithFOverride(cmd, active, other, "")
}

// CommandRequiresIteratedF reports whether cmd contains a %f macro (not %%).
func CommandRequiresIteratedF(cmd string) bool {
	return cmdmacro.CommandRequiresMacro(cmd, 'f')
}

// ErrRunForEachRequiresF is returned when a run-for-each command omits %f.
const ErrRunForEachRequiresF = cmdmacro.ErrRunForEachRequiresF

// ExpandCommandWithFOverride behaves like ExpandCommand, but when fOverride is non-empty,
// %f expands to that value (shell-quoted) instead of the active panel cursor entry.
func ExpandCommandWithFOverride(cmd string, active, other *panel.State, fOverride string) (string, error) {
	return cmdmacro.ExpandCommandLine(cmd, MacroContext(active, other, fOverride, ""))
}

// MacroContext builds cmdmacro.Context from panel state.
func MacroContext(active, other *panel.State, fOverride, rowPath string) cmdmacro.Context {
	var a, o *cmdmacro.PanelSnapshot
	if active != nil {
		a = panelSnapshot(active)
	}
	if other != nil {
		o = panelSnapshot(other)
	}
	return cmdmacro.Context{
		Active:    a,
		Other:     o,
		FOverride: fOverride,
		RowPath:   rowPath,
	}
}

func panelSnapshot(ps *panel.State) *cmdmacro.PanelSnapshot {
	if ps == nil {
		return nil
	}
	root := pathloc.Clean(ps.PathString())
	snap := &cmdmacro.PanelSnapshot{Dir: root}
	if ent, ok := ps.CurrentEntry(); ok {
		snap.HasCurrent = true
		snap.CurrentName = ent.Path
		// Tree rows can sit in expanded subdirectories, and rows of a flat tag view in many: %d is
		// the directory holding the caret row.
		snap.Dir = pathloc.Clean(ps.EntryDir(ent).String())
	}
	snap.TaggedInDir = ps.CommandTags()
	return snap
}
