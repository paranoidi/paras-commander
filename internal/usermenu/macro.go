package usermenu

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/entrymatch"
	"github.com/paranoidi/paras-commander/internal/panel"
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
	root := filepath.Clean(ps.PathString())
	snap := &cmdmacro.PanelSnapshot{Dir: root}
	if ent, ok := ps.CurrentEntry(); ok {
		snap.HasCurrent = true
		snap.CurrentName = ent.Path
		// Tree rows can sit in expanded subdirectories: %d is the directory holding the caret row.
		if ps.ListLayout == panel.ListLayoutTree && ent.Name != ".." {
			snap.Dir = filepath.Clean(filepath.Dir(ent.Path))
		}
	}
	tree := ps.ListLayout == panel.ListLayoutTree
	for p, on := range ps.SelectedPaths {
		// Tree view tags rows at any depth, so %t takes every tag under the root.
		if on && (filepath.Clean(filepath.Dir(p)) == root || tree && underDir(root, p)) {
			snap.TaggedInDir = append(snap.TaggedInDir, p)
		}
	}
	return snap
}

func underDir(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, "../")
}
