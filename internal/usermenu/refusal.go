package usermenu

import (
	"slices"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// LocalOnlyRefusal is the message for a user command that has no local directory or file to
// work on.
const LocalOnlyRefusal = "User commands run on local panels only"

// Launch describes a user-menu or Run-for-each command for CommandRefusal.
type Launch struct {
	Command  string
	CwdPanel bool // the entry runs in the panel directory instead of the cursor row's directory
	// RunForEach is set for a run_for_each entry and the Run-for-each dialog; RunForEachRows are
	// then the rows the command runs on (ops.ResolveSource: the tags, or the cursor row when
	// nothing is tagged). A caller resolves them only once the active panel is known to be local.
	RunForEach     bool
	RunForEachRows []localfs.Entry
}

// CommandRefusal is the message for refusing the launch of l from the active panel, or "" when it
// may run. The only rule: a command must never receive a fileindex:// value (a ghost row, or a
// path of an index panel) as if it were a real file or directory. So a non-local active panel is
// always refused, and otherwise only what the command actually reads counts (for a fallback group
// %{a|b}, the alternative that resolves): %f (cursor row),
// %t (the tags %t expands to, tagged ghost rows included: panel.State.CommandTags), %d or the cursor-row working directory, the rows of a
// run_for_each launch, and %F, %T and %D of the other panel (which may be SFTP, but not an index
// panel). A pure string check, safe on the UI goroutine.
func CommandRefusal(l Launch, active, other *panel.State) string {
	if active == nil {
		return ""
	}
	if !active.Path.IsLocal() {
		return LocalOnlyRefusal
	}
	ctx := MacroContext(active, other, "", "")
	resolved := cmdmacro.ResolvedMacros(l.Command, ctx)
	uses := func(letter byte) bool { return cmdmacro.UsesMacro(resolved, letter) }
	var values []string // every value the command would read; the fileindex:// ones are refused
	localOnly := false  // refused with no row to name
	if l.RunForEach {
		for _, e := range l.RunForEachRows {
			values = append(values, e.Path)
		}
	} else if uses('f') && ctx.Active.HasCurrent {
		values = append(values, ctx.Active.CurrentName)
	}
	if uses('t') {
		values = append(values, ctx.Active.TaggedInDir...)
	}
	if (uses('d') || !l.RunForEach && !l.CwdPanel) && !active.CursorDir().IsLocal() {
		// On a local panel only a ghost row (a fileindex:// path in a tree) sits in another scheme.
		values = append(values, ctx.Active.CurrentName)
	}
	if o := ctx.Other; o != nil {
		if uses('F') && o.HasCurrent {
			values = append(values, o.CurrentName)
		}
		if uses('T') {
			values = append(values, o.TaggedInDir...)
		}
	}
	if uses('D') && other != nil && other.CursorDir().Scheme() == pathloc.SchemeFileIndex {
		// An index panel's own directory is no row's fault; a ghost row in a local tree is.
		if !other.Path.IsLocal() {
			localOnly = true
		} else {
			values = append(values, ctx.Other.CurrentName)
		}
	}

	names := active.GhostSources(values...)
	slices.Sort(names) // the message lists each name once, sorted
	if msg := panel.GhostRefusal(slices.Compact(names)); msg != "" {
		return msg
	}
	if localOnly {
		return LocalOnlyRefusal
	}
	return ""
}
