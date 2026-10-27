package app

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/statuscmds"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// statusCmdRunner is one shown status command: its definition, the overlap guard shared
// with its ticker goroutine, and its latest output (main goroutine only).
type statusCmdRunner struct {
	def     statuscmds.Def
	running atomic.Bool
	text    string
}

// statusCommandsPath is status.toml under the config directory ("" when unknown).
func (a *App) statusCommandsPath() string {
	return statuscmds.Path(a.paths.ConfigDir)
}

// restartStatusCommands reloads status.toml and (re)starts one background ticker per shown
// status command, stopping the previous tickers first. Called at startup and after the
// status-commands dialog saves a new selection.
func (a *App) restartStatusCommands() {
	set, err := statuscmds.Load(a.statusCommandsPath())
	if err != nil {
		a.setErrorMessage("Status commands", err)
	}
	shown := set.Shown()
	for _, d := range shown {
		if d.Color == "" {
			continue
		}
		if _, err := a.styles.ResolveColor(d.Color); err != nil {
			a.setErrorMessage("Status commands", fmt.Errorf("status.toml: command %q color: %w", d.Name, err))
		}
	}
	a.startStatusCommands(shown)
}

// startStatusCommands replaces the running status commands with defs.
func (a *App) startStatusCommands(defs []statuscmds.Def) {
	a.stopStatusCommands()
	a.statusCmdGen++
	a.statusCmds = nil
	if len(defs) == 0 {
		return
	}
	stop := make(chan struct{})
	a.statusCmdStopCh = stop
	for i, d := range defs {
		r := &statusCmdRunner{def: d}
		a.statusCmds = append(a.statusCmds, r)
		go a.runStatusCommandTicker(r, a.statusCmdGen, i, stop)
	}
}

// stopStatusCommands stops every status-command ticker (results already in flight are
// dropped by their stale generation).
func (a *App) stopStatusCommands() {
	if a.statusCmdStopCh != nil {
		close(a.statusCmdStopCh)
		a.statusCmdStopCh = nil
	}
}

// statusCommandTimeout bounds a single status-command run so a hung command can't
// accumulate; independent of the configured poll interval.
const statusCommandTimeout = 10 * time.Second

// statusCommandResultPayload delivers async status-command output to the main loop. Gen
// identifies the startStatusCommands call and Index the runner within it.
type statusCommandResultPayload struct {
	Gen   int
	Index int
	Text  string
}

// runStatusCommandTicker fires r immediately, then on every interval, until stop closes.
// Overlapping runs are skipped (see requestStatusCommandRun's CAS guard) rather than queued.
func (a *App) runStatusCommandTicker(r *statusCmdRunner, gen, index int, stop <-chan struct{}) {
	interval := time.Duration(r.def.IntervalMS) * time.Millisecond
	if interval <= 0 {
		return
	}
	a.requestStatusCommandRun(r, gen, index)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			a.requestStatusCommandRun(r, gen, index)
		}
	}
}

func (a *App) requestStatusCommandRun(r *statusCmdRunner, gen, index int) {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	command, maxWidth := r.def.Command, r.def.MaxWidth
	go func() {
		defer r.running.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), statusCommandTimeout)
		defer cancel()
		// No macro context (no active panel/row) applies to a global status command, so this
		// runs the configured line via sh -c directly rather than cmdrun.BuildInvocation
		// (which requires a panel/row cmdmacro.Context even when the template has no macros).
		result := cmdrun.Run(ctx, cmdrun.ShellArgv(command), "", cmdrun.MaxStreamBytes)
		line := ""
		// A failing (or timed-out) command clears the text so stale output never lingers.
		if result.LaunchErr == nil && result.ExitCode == 0 {
			line = textutil.TruncateBannerRunes(textutil.FirstLine(string(result.Stdout)), maxWidth)
		}
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(statusCommandResultPayload{Gen: gen, Index: index, Text: line}))
	}()
}

// applyStatusCommandResult merges an async status-command result into app state. It
// returns true when a displayed text changed (caller may repaint).
func (a *App) applyStatusCommandResult(d statusCommandResultPayload) bool {
	if d.Gen != a.statusCmdGen || d.Index < 0 || d.Index >= len(a.statusCmds) {
		return false
	}
	r := a.statusCmds[d.Index]
	if d.Text == r.text {
		return false
	}
	r.text = d.Text
	return true
}

// statusCommandSegments returns the shown status commands' non-empty outputs in show order,
// each styled menu.status with its color (resolved against the current theme) as foreground.
// A color the theme can't resolve keeps menu.status; restartStatusCommands reports it.
func (a *App) statusCommandSegments() []ui.StatusCommandSegment {
	var out []ui.StatusCommandSegment
	for _, r := range a.statusCmds {
		if r.text == "" {
			continue
		}
		style := a.styles.MenuStatus
		if r.def.Color != "" {
			if c, err := a.styles.ResolveColor(r.def.Color); err == nil {
				style = style.Foreground(c)
			}
		}
		out = append(out, ui.StatusCommandSegment{Text: r.text, Style: style})
	}
	return out
}
