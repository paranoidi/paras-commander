package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type runForEachItemBuilder func(entry localfs.Entry) (item RunForEachBuiltItem, err error)

// RunForEachBatchSpec describes one run-for-each style command batch (Run-for-each dialog,
// user-menu run_for_each entries).
type RunForEachBatchSpec struct {
	Kind ui.CommandRunKind

	// Entries to iterate in order.
	Entries []localfs.Entry

	// Selection allowlist. When both are false, all entries are allowed.
	AllowFiles bool
	AllowDirs  bool

	// WorkDir is used as exec.Cmd.Dir.
	WorkDir string
	// PerEntryWorkDir uses each entry's own absolute path as the working directory instead of
	// WorkDir.
	PerEntryWorkDir bool

	// PoolName optionally gates each invocation through the work pool registry.
	PoolName string

	// Background keeps the browser visible and emits a single notification on issues.
	Background bool
	// NotifyLabel prefixes issue summaries (e.g. "Run for each", "User menu: Build").
	NotifyLabel string

	// PTY runs each entry attached to a live pseudo-TTY, shown interactively in the same
	// bottom terminal-panel strip Alt+P uses, instead of capturing stdout/stderr into memory
	// buffers non-interactively. Lets tools that open /dev/tty for prompts (e.g. chezmoi) work
	// under run-for-each. Falls back silently to the non-PTY path on platforms without PTY
	// support.
	PTY bool

	BuildItem runForEachItemBuilder
}

// StartRunForEachBatch enqueues Commands-view rows for spec.Entries and runs them in the
// background, one at a time in submission order (gated by PoolName's parallelism, if any).
func (h *Handler) StartRunForEachBatch(spec RunForEachBatchSpec) {
	if len(spec.Entries) == 0 {
		h.host.SetTransientMessage("No paths to run", ui.MessageUrgencyWarn)
		return
	}
	if spec.BuildItem == nil {
		h.host.SetTransientMessage("Run for each: internal error (missing builder)", ui.MessageUrgencyError)
		return
	}

	entries := make([]ui.CommandRunEntry, len(spec.Entries))
	for i := range spec.Entries {
		entries[i] = ui.CommandRunEntry{
			ID:       cmdrun.NewRunID(),
			Kind:     spec.Kind,
			Phase:    ui.CommandRunPending,
			ExitCode: -1,
			PTY:      spec.PTY,
		}
	}
	h.mu.Lock()
	start := len(h.model.CommandsList)
	h.model.CommandsList = append(h.model.CommandsList, entries...)
	h.mu.Unlock()

	if !spec.Background && !spec.PTY {
		h.OpenViewAt(start)
	}

	h.BeginBatch()
	go h.runForEachUnifiedBatch(h.ctx, start, spec)
}

func (h *Handler) runForEachUnifiedBatch(ctx context.Context, start int, spec RunForEachBatchSpec) {
	var panelOwner int64
	acquired := false
	defer func() {
		h.EndBatch()
		if acquired {
			h.postPanelWakeAndWait(ctx, WakePayload{TerminalPanelHide: true})
			h.releaseForegroundPanel(panelOwner)
		}
		var snap []ui.CommandRunEntry
		h.mu.RLock()
		if start >= 0 && start < len(h.model.CommandsList) {
			end := start + len(spec.Entries)
			if end > len(h.model.CommandsList) {
				end = len(h.model.CommandsList)
			}
			snap = append([]ui.CommandRunEntry(nil), h.model.CommandsList[start:end]...)
		}
		h.mu.RUnlock()

		p := WakePayload{ClearActiveSelection: true}
		if spec.Background {
			p.RefreshBrowserPanel = true
		}
		if log, banner, urg, ok := summarizeRunForEachIssues(spec.NotifyLabel, snap); ok {
			p.NotifyLog = log
			p.NotifyBanner = banner
			p.NotifyUrg = urg
		}
		h.PostWake(p)
	}()

	allowFilter := spec.AllowFiles || spec.AllowDirs

	if spec.PTY && !spec.Background {
		panelOwner = h.ptyOwnerSeq.Add(1)
		if !h.acquireForegroundPanel(ctx, panelOwner) {
			h.markCommandsCanceled(start, len(spec.Entries))
			h.PostRenderWake()
			return
		}
		acquired = true
		h.postPanelWakeAndWait(ctx, WakePayload{TerminalPanelShow: true})
	}

	for i, ent := range spec.Entries {
		select {
		case <-ctx.Done():
			h.markCommandsCanceled(start+i, len(spec.Entries)-i)
			h.PostRenderWake()
			return
		default:
		}

		idx := start + i
		abs := textutil.AbsPathClean(ent.Path)

		built, buildErr := spec.BuildItem(ent)
		userLine := built.UserLine
		if buildErr != nil {
			h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
				e.Phase = ui.CommandRunDone
				e.TargetPath = abs
				e.UserCommandLine = userLine
				e.ExitCode = -1
				e.ErrorMsg = buildErr.Error()
			})
			h.PostRenderWake()
			continue
		}
		argvPrefix := built.Argv
		if len(argvPrefix) == 0 {
			h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
				e.Phase = ui.CommandRunDone
				e.TargetPath = abs
				e.UserCommandLine = userLine
				e.ExitCode = -1
				e.ErrorMsg = "Command is empty"
			})
			h.PostRenderWake()
			continue
		}

		if allowFilter {
			if ent.Type == localfs.EntryDirectory && !spec.AllowDirs {
				h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
					e.Phase = ui.CommandRunDone
					e.TargetPath = abs
					e.UserCommandLine = userLine
					e.ExitCode = -1
					e.ErrorMsg = "Skipped: directories are not allowed"
				})
				h.PostRenderWake()
				continue
			}
			if ent.Type != localfs.EntryDirectory && !spec.AllowFiles {
				h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
					e.Phase = ui.CommandRunDone
					e.TargetPath = abs
					e.UserCommandLine = userLine
					e.ExitCode = -1
					e.ErrorMsg = "Skipped: files are not allowed"
				})
				h.PostRenderWake()
				continue
			}
		}

		argv := append([]string(nil), argvPrefix...)

		var release func()
		if strings.TrimSpace(spec.PoolName) != "" {
			// Stay Pending through Acquire so terminate/kill can tell "no process
			// yet" from a live subprocess. Target/line are filled so the waiting
			// row still names the command.
			h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
				e.TargetPath = abs
				e.UserCommandLine = userLine
			})
			h.PostRenderWake()
			var err error
			release, err = h.workPools.Acquire(ctx, spec.PoolName)
			if err != nil {
				h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
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
				h.PostRenderWake()
				continue
			}
		}

		markRunning := func() {
			h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
				e.Phase = ui.CommandRunRunning
				e.TargetPath = abs
				e.UserCommandLine = userLine
			})
			h.PostRenderWake()
		}

		workDir := spec.WorkDir
		if spec.PerEntryWorkDir {
			workDir = abs
		}
		var res cmdrun.RunResult
		if spec.PTY {
			// Stays Pending while the PTY starts; name the row now so it isn't blank.
			h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
				e.TargetPath = abs
				e.UserCommandLine = userLine
			})
			res = h.runEntryPTY(ctx, idx, argv, workDir, panelOwner, markRunning)
		} else {
			markRunning()
			res = cmdrun.RunTracked(ctx, argv, workDir, cmdrun.MaxStreamBytes, func(p *os.Process) {
				h.SetProcess(idx, p)
			})
			h.UnregisterProc(idx)
		}
		if release != nil {
			release()
		}

		h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
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
		h.PostRenderWake()
	}
}

// runEntryPTY runs argv attached to a live pseudo-TTY shown in the bottom terminal panel
// (the same strip Alt+P uses), blocking until it exits. Falls back silently to the ordinary
// cmdrun.RunTracked path when PTY sessions aren't supported on this platform. panelOwner is
// the foreground batch lease id (0 for background): only the owning batch installs a drawer.
// markRunning flips the row to Running only once the session is registered, so terminate/kill
// on a Running row always finds the PTY to close.
func (h *Handler) runEntryPTY(ctx context.Context, idx int, argv []string, workDir string, panelOwner int64, markRunning func()) cmdrun.RunResult {
	sub, err := subshell.StartArgv(argv, workDir)
	if err != nil {
		if errors.Is(err, subshell.ErrUnsupportedPlatform) {
			markRunning()
			res := cmdrun.RunTracked(ctx, argv, workDir, cmdrun.MaxStreamBytes, func(p *os.Process) {
				h.SetProcess(idx, p)
			})
			h.UnregisterProc(idx)
			return res
		}
		return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
	}

	cols, rows, ok := h.terminalPanelDims()
	if !ok {
		cols, rows = 80, 24 // ponytail: fallback size when the layout can't be measured; still runs, just at a default size.
	}
	feed, err := sub.StartPanelFeed(cols, rows, h.PostRenderWake)
	if err != nil {
		_ = sub.Close()
		return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
	}

	sess := &entryPTYSession{idx: idx, sub: sub, feed: feed}
	h.registerEntryPTY(panelOwner, sess)
	markRunning()
	if panelOwner != 0 {
		drawer := &subshell.PanelDrawer{Sub: sub, Feed: feed, Style: h.host.Styles().TerminalTextStyle()}
		h.postPanelWakeAndWait(ctx, WakePayload{TerminalPanelDrawer: drawer})
	}

	// Unlike cmdrun.RunTracked (exec.CommandContext, killed automatically on ctx cancel),
	// StartArgv's child has no ctx wiring — close the session ourselves on quit instead of
	// leaking a PTY child that outlives the app.
	select {
	case <-sub.Done():
	case <-ctx.Done():
		_ = sub.Close()
	}

	exitCode := sub.ExitCode()
	stdout := feed.SnapshotText()
	_ = sub.Close()
	h.unregisterEntryPTY(idx)
	if panelOwner != 0 {
		h.PostWake(WakePayload{TerminalPanelClearDrawer: true})
	}

	return cmdrun.RunResult{ExitCode: exitCode, Stdout: []byte(stdout)}
}

// terminalPanelDims returns the content cols/rows for the bottom terminal panel strip, the same
// rect drawTerminalPanel paints into. Only returns real dims once TerminalPanel.Visible is true
// and ViewMode == ui.ViewBrowser (that's how the layout code reserves the strip — see
// internal/app/terminal_panel.go's terminalLayoutRows) — runEntryPTY calls this only after the
// batch-level panel setup in runForEachUnifiedBatch has already flipped those on. ok is false
// when the layout has no room for the strip.
func (h *Handler) terminalPanelDims() (cols, rows int, ok bool) {
	w, ht := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(w, ht)
	if layout.Terminal.Width <= 0 || layout.Terminal.Height <= 0 {
		return 0, 0, false
	}
	return layout.Terminal.Width, layout.Terminal.Height, true
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
				firstDetail = textutil.FirstLine(strings.TrimSpace(e.Stderr))
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
	banner = prefix + ": " + textutil.TruncateBannerRunes(summary, textutil.BannerMaxRunes)
	return log, banner, worstUrg, true
}
