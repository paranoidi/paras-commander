package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/textutil"
)

// restartStatusCommandTicker (re)starts the status_command background ticker from cfg,
// stopping any ticker already running first. Called at startup and after config.toml is
// reloaded so edits to [status_command] take effect without an app restart.
func (a *App) restartStatusCommandTicker(cfg config.StatusCommandConfig) {
	if a.statusCmdStopCh != nil {
		close(a.statusCmdStopCh)
		a.statusCmdStopCh = nil
	}
	if cfg.Command == "" {
		return
	}
	stop := make(chan struct{})
	a.statusCmdStopCh = stop
	go a.runStatusCommandTicker(cfg.Command, time.Duration(cfg.IntervalMS)*time.Millisecond, cfg.MaxWidth, stop)
}

// statusCommandTimeout bounds a single status_command run so a hung command can't
// accumulate; independent of the configured poll interval.
const statusCommandTimeout = 10 * time.Second

// statusCommandResultPayload delivers async status_command output to the main loop.
type statusCommandResultPayload struct {
	Text string
	OK   bool
}

// runStatusCommandTicker fires command immediately, then on every interval, until stop
// closes. Overlapping runs are skipped (see requestStatusCommandRun's CAS guard) rather
// than queued.
func (a *App) runStatusCommandTicker(command string, interval time.Duration, maxWidth int, stop <-chan struct{}) {
	if interval <= 0 {
		return
	}
	a.requestStatusCommandRun(command, maxWidth)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			a.requestStatusCommandRun(command, maxWidth)
		}
	}
}

func (a *App) requestStatusCommandRun(command string, maxWidth int) {
	if !a.statusCmdRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer a.statusCmdRunning.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), statusCommandTimeout)
		defer cancel()
		// No macro context (no active panel/row) applies to a global status command, so this
		// runs the configured line via sh -c directly rather than cmdrun.BuildInvocation
		// (which requires a panel/row cmdmacro.Context even when the template has no macros).
		result := cmdrun.Run(ctx, cmdrun.ShellArgv(command), "", cmdrun.MaxStreamBytes)
		if result.LaunchErr != nil || result.ExitCode != 0 {
			// Keep displaying the last known-good text rather than blanking on a
			// transient failure.
			return
		}
		line := textutil.TruncateBannerRunes(textutil.FirstLine(string(result.Stdout)), maxWidth)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(statusCommandResultPayload{Text: line, OK: true}))
	}()
}

// applyStatusCommandResult merges an async status_command result into app state. It
// returns true when the displayed text changed (caller may repaint).
func (a *App) applyStatusCommandResult(d statusCommandResultPayload) bool {
	if !d.OK || d.Text == a.statusCmdText {
		return false
	}
	a.statusCmdText = d.Text
	return true
}
