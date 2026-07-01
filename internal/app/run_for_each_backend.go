package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type runForEachItemBuilder func(entry localfs.Entry) (item runForEachBuiltItem, err error)

type runForEachBatchSpec struct {
	Kind ui.CommandRunKind

	// Entries to iterate in order.
	Entries []localfs.Entry

	// Selection allowlist. When both are false, all entries are allowed.
	AllowFiles bool
	AllowDirs  bool

	// WorkDir is used as exec.Cmd.Dir.
	WorkDir string

	// PoolName optionally gates each invocation through the work pool registry.
	PoolName string

	// Background keeps the browser visible and emits a single notification on issues.
	Background bool
	// NotifyLabel prefixes issue summaries (e.g. "Run for each", "User menu: Build").
	NotifyLabel string

	BuildItem runForEachItemBuilder
}

func (a *App) startRunForEachBatch(spec runForEachBatchSpec) {
	if len(spec.Entries) == 0 {
		a.setTransientMessage("No paths to run", ui.MessageUrgencyWarn)
		return
	}
	if spec.BuildItem == nil {
		a.setTransientMessage("Run for each: internal error (missing builder)", ui.MessageUrgencyError)
		return
	}

	entries := make([]ui.CommandRunEntry, len(spec.Entries))
	for i := range spec.Entries {
		entries[i] = ui.CommandRunEntry{
			ID:       cmdrun.NewRunID(),
			Kind:     spec.Kind,
			Phase:    ui.CommandRunPending,
			ExitCode: -1,
		}
	}
	a.commandsMu.Lock()
	start := len(a.model.CommandsList)
	a.model.CommandsList = append(a.model.CommandsList, entries...)
	a.commandsMu.Unlock()

	if !spec.Background {
		a.openCommandsView()
		a.model.CommandsView.Selected = start
		a.model.CommandsView.FocusPane = 0
		a.model.CommandsView.ListScroll = 0
		a.model.CommandsView.StdoutScroll = 0
		a.model.CommandsView.StderrScroll = 0
		a.ensureCommandsViewSelectionVisible()
	}

	a.commandsBatchesInflight.Add(1)
	go a.runForEachUnifiedBatch(a.commandsCtx, start, spec)
}

func (a *App) runForEachUnifiedBatch(ctx context.Context, start int, spec runForEachBatchSpec) {
	defer func() {
		a.commandsBatchesInflight.Add(-1)
		var snap []ui.CommandRunEntry
		a.commandsMu.RLock()
		if start >= 0 && start < len(a.model.CommandsList) {
			end := start + len(spec.Entries)
			if end > len(a.model.CommandsList) {
				end = len(a.model.CommandsList)
			}
			snap = append([]ui.CommandRunEntry(nil), a.model.CommandsList[start:end]...)
		}
		a.commandsMu.RUnlock()

		p := commandWakePayload{clearActiveSelection: true}
		if spec.Background {
			p.refreshBrowserPanel = true
		}
		if log, banner, urg, ok := summarizeRunForEachIssues(spec.NotifyLabel, snap); ok {
			p.notifyLog = log
			p.notifyBanner = banner
			p.notifyUrg = urg
		}
		if spec.Background || p.notifyLog != "" {
			a.postCommandWakePayload(p)
			return
		}
		a.postCommandWakePayload(p)
	}()

	allowFilter := spec.AllowFiles || spec.AllowDirs

	for i, ent := range spec.Entries {
		select {
		case <-ctx.Done():
			a.markCommandsCanceled(start+i, len(spec.Entries)-i)
			a.postCommandWake()
			return
		default:
		}

		idx := start + i
		abs := absPathClean(ent.Path)

		built, buildErr := spec.BuildItem(ent)
		userLine := built.UserLine
		if buildErr != nil {
			a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
				e.Phase = ui.CommandRunDone
				e.TargetPath = abs
				e.UserCommandLine = userLine
				e.ExitCode = -1
				e.ErrorMsg = buildErr.Error()
			})
			a.postCommandWake()
			continue
		}
		argvPrefix := built.Argv
		if len(argvPrefix) == 0 {
			a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
				e.Phase = ui.CommandRunDone
				e.TargetPath = abs
				e.UserCommandLine = userLine
				e.ExitCode = -1
				e.ErrorMsg = "Command is empty"
			})
			a.postCommandWake()
			continue
		}

		if allowFilter {
			if ent.Type == localfs.EntryDirectory && !spec.AllowDirs {
				a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
					e.Phase = ui.CommandRunDone
					e.TargetPath = abs
					e.UserCommandLine = userLine
					e.ExitCode = -1
					e.ErrorMsg = "Skipped: directories are not allowed"
				})
				a.postCommandWake()
				continue
			}
			if ent.Type != localfs.EntryDirectory && !spec.AllowFiles {
				a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
					e.Phase = ui.CommandRunDone
					e.TargetPath = abs
					e.UserCommandLine = userLine
					e.ExitCode = -1
					e.ErrorMsg = "Skipped: files are not allowed"
				})
				a.postCommandWake()
				continue
			}
		}

		argv := append([]string(nil), argvPrefix...)

		a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunRunning
			e.TargetPath = abs
			e.UserCommandLine = userLine
		})
		a.postCommandWake()

		var release func()
		if strings.TrimSpace(spec.PoolName) != "" {
			var err error
			release, err = a.workPools.Acquire(ctx, spec.PoolName)
			if err != nil {
				a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
					e.Phase = ui.CommandRunDone
					e.ExitCode = -1
					if ctx.Err() != nil {
						if e.ErrorMsg == "" {
							e.ErrorMsg = "Canceled"
						}
					} else {
						e.ErrorMsg = err.Error()
					}
				})
				a.postCommandWake()
				continue
			}
		}

		res := cmdrun.RunTracked(ctx, argv, spec.WorkDir, cmdrun.MaxStreamBytes, func(p *os.Process) {
			a.setCommandProcess(idx, p)
		})
		a.unregisterCommandProc(idx)
		if release != nil {
			release()
		}

		a.patchCommandEntry(idx, func(e *ui.CommandRunEntry) {
			e.Phase = ui.CommandRunDone
			e.Stdout = string(res.Stdout)
			e.Stderr = string(res.Stderr)
			if res.LaunchErr != nil {
				e.ErrorMsg = res.LaunchErr.Error()
				e.ExitCode = -1
			} else {
				e.ExitCode = res.ExitCode
			}
		})
		a.postCommandWake()
	}
}

func summarizeRunForEachIssues(notifyLabel string, entries []ui.CommandRunEntry) (log, banner string, urg ui.MessageUrgency, ok bool) {
	if len(entries) == 0 {
		return "", "", ui.MessageUrgencyInfo, false
	}
	prefix := strings.TrimSpace(notifyLabel)
	if prefix == "" {
		prefix = "command"
	}

	var skipped, failed, stderr int
	var firstDetail string
	worstUrg := ui.MessageUrgencyInfo

	for _, e := range entries {
		if strings.HasPrefix(strings.TrimSpace(e.ErrorMsg), "Skipped:") {
			skipped++
			if firstDetail == "" {
				firstDetail = e.ErrorMsg
			}
			if worstUrg < ui.MessageUrgencyWarn {
				worstUrg = ui.MessageUrgencyWarn
			}
			continue
		}
		if e.ExitCode != 0 || strings.TrimSpace(e.ErrorMsg) != "" {
			failed++
			if firstDetail == "" {
				firstDetail = strings.TrimSpace(e.ErrorMsg)
				if firstDetail == "" {
					firstDetail = fmt.Sprintf("exit %d", e.ExitCode)
				}
			}
			worstUrg = ui.MessageUrgencyError
			continue
		}
		if strings.TrimSpace(e.Stderr) != "" {
			stderr++
			if firstDetail == "" {
				firstDetail = firstMessageLine(strings.TrimSpace(e.Stderr))
			}
			if worstUrg < ui.MessageUrgencyWarn {
				worstUrg = ui.MessageUrgencyWarn
			}
		}
	}

	if skipped == 0 && failed == 0 && stderr == 0 {
		return "", "", ui.MessageUrgencyInfo, false
	}

	var parts []string
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if stderr > 0 {
		parts = append(parts, fmt.Sprintf("%d stderr", stderr))
	}
	summary := strings.Join(parts, ", ")
	log = prefix + ": " + summary
	if strings.TrimSpace(firstDetail) != "" {
		log += " (" + strings.TrimSpace(firstDetail) + ")"
	}
	banner = prefix + ": " + truncateStatusBannerRunes(summary, jobFailureBannerMaxRunes)
	return log, banner, worstUrg, true
}
