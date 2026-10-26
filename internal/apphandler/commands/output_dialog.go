package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// CloseOutputDialog closes the command-output dialog.
func (h *Handler) CloseOutputDialog() {
	if cancel := h.model.CommandOutputDialog.Cancel; cancel != nil {
		cancel()
	}
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{}
}

// OpenRunningOutputDialog opens the output dialog in its running state for the Commands row
// rowID. cancel kills the run; queued shows it as waiting for a pool slot until
// WakePayload.OutputDialogStartedRunID arrives. Main goroutine only.
func (h *Handler) OpenRunningOutputDialog(rowID, title string, queued bool, cancel context.CancelFunc, prefW, prefH string) {
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{
		Open:       true,
		Running:    true,
		Queued:     queued,
		Follow:     true,
		RunID:      rowID,
		Cancel:     cancel,
		Title:      strings.TrimSpace(title),
		PrefWidth:  prefW,
		PrefHeight: prefH,
	}
}

// closeOutputDialogKeepRunning closes the dialog without canceling the run (Background).
func (h *Handler) closeOutputDialogKeepRunning() {
	h.model.CommandOutputDialog = dialog.CommandOutputDialogState{}
}

// HandleOutputDialogKey handles key events while the command-output dialog is open.
func (h *Handler) HandleOutputDialogKey(event *tcell.EventKey) {
	st := &h.model.CommandOutputDialog
	scrollEv := event
	if !st.Running {
		scrollEv = keymap.RemapViMotionKey(event) // hjkl: no text input here
		if len(st.Paths) > 0 && h.handlePathKey(st, event, scrollEv.Key()) {
			return
		}
	}
	if h.scrollOutputDialog(st, scrollEv.Key()) {
		return
	}
	if ev, ok := dlg.HandleKey(dialog.CommandOutputDialogKeys(*st), &st.Focus, event); ok {
		h.ApplyOutputDialogEvent(ev)
	}
}

// ApplyOutputDialogEvent acts on a widget event of the command-output dialog (key or click).
func (h *Handler) ApplyOutputDialogEvent(ev dlg.Event) {
	switch ev.ID {
	case dialog.OutputBackground:
		h.closeOutputDialogKeepRunning()
	case dialog.OutputCancel, dlg.OK:
		h.CloseOutputDialog()
	}
}

// OutputDialogButtons returns what OK, Cancel and Esc do in the open command-output dialog.
// While the command runs, OK and Esc keep it running in the background and Cancel stops it;
// once finished all three just close the dialog.
func (h *Handler) OutputDialogButtons() (ok, cancel, esc func()) {
	if h.model.CommandOutputDialog.Running {
		return h.closeOutputDialogKeepRunning, h.CloseOutputDialog, h.closeOutputDialogKeepRunning
	}
	return h.CloseOutputDialog, h.CloseOutputDialog, nil
}

// OutputDialogResult builds the finished-state output dialog for a completed run.
func OutputDialogResult(runID, title string, res cmdrun.RunResult, prefW, prefH, workDir string) dialog.CommandOutputDialogState {
	dialogTitle := strings.TrimSpace(title)
	stderr := string(res.Stderr)
	if res.LaunchErr != nil {
		stderr = res.LaunchErr.Error()
	} else if res.ExitCode != 0 {
		if dialogTitle != "" {
			dialogTitle += fmt.Sprintf(" (exit %d)", res.ExitCode)
		} else {
			dialogTitle = fmt.Sprintf("exit %d", res.ExitCode)
		}
	}
	lines := outputDialogLines(string(res.Stdout), stderr)
	return dialog.CommandOutputDialogState{
		Open:       true,
		RunID:      runID,
		Title:      dialogTitle,
		Lines:      lines,
		PrefWidth:  prefW,
		PrefHeight: prefH,
		Paths:      detectOutputPaths(lines, workDir),
		WorkDir:    workDir,
	}
}

// outputDialogLines is the single builder for the dialog body: stdout lines, then a
// "--- stderr ---" separator and stderr lines when stderr is non-blank.
func outputDialogLines(stdout, stderr string) []string {
	var lines []string
	if stdout = strings.TrimRight(stdout, "\n"); stdout != "" {
		lines = strings.Split(stdout, "\n")
	}
	if stderr = strings.TrimSpace(stderr); stderr != "" {
		lines = append(lines, "--- stderr ---")
		lines = append(lines, strings.Split(stderr, "\n")...)
	}
	return lines
}

// handlePathKey moves the path cursor (Up/Down by line, Left/Right by match; key is the
// vi-remapped key) or toggles the highlighted path's selection in the active panel. Returns
// false for any other key.
func (h *Handler) handlePathKey(st *dialog.CommandOutputDialogState, event *tcell.EventKey, key tcell.Key) bool {
	n := len(st.Paths)
	cur := min(st.PathCursor, n-1)
	switch key {
	case tcell.KeyDown:
		cur = nextPathLine(st.Paths, cur)
	case tcell.KeyUp:
		cur = prevPathLine(st.Paths, cur)
	case tcell.KeyRight:
		cur = min(cur+1, n-1)
	case tcell.KeyLeft:
		cur = max(cur-1, 0)
	default:
		if h.handleGroupSelectionKey(st, event) {
			return true
		}
		id, ok := h.keysCommandOutputDialog.Lookup(event)
		if !ok || id != keymap.ActionCommandOutputToggleSelection {
			return false
		}
		if _, conflicts := h.host.ActivePanel().TogglePathSelection(st.Paths[cur].Path); conflicts {
			h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
		cur = min(cur+1, n-1)
	}
	st.PathCursor = cur
	h.ensurePathVisible(st)
	return true
}

// handleGroupSelectionKey runs the global select-group, unselect-group and invert-selection
// actions over the detected paths. Returns false for any other key.
func (h *Handler) handleGroupSelectionKey(st *dialog.CommandOutputDialogState, event *tcell.EventKey) bool {
	id, _ := h.keys.Lookup(event)
	switch id {
	case keymap.ActionPanelSelectGroup:
		h.host.OpenOutputGroupSelect(true)
	case keymap.ActionPanelUnselectGroup:
		h.host.OpenOutputGroupSelect(false)
	case keymap.ActionPanelInvertSelection:
		uniq := st.UniquePaths()
		p := h.host.ActivePanel()
		conflicts := false
		for _, m := range uniq {
			_, c := p.TogglePathSelection(m.Path)
			conflicts = conflicts || c
		}
		h.host.SetTransientMessage(fmt.Sprintf("Inverted %d", len(uniq)), ui.MessageUrgencyInfo)
		if conflicts {
			h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
	default:
		return false
	}
	return true
}

// ensurePathVisible scrolls so the cursor match's line is on screen and clears Follow.
func (h *Handler) ensurePathVisible(st *dialog.CommandOutputDialogState) {
	visH := h.outputDialogVisH(*st)
	if st.Follow {
		st.Scroll = max(0, len(st.Lines)-visH)
		st.Follow = false
	}
	line := st.Paths[st.PathCursor].Line
	if line < st.Scroll {
		st.Scroll = line
	} else if line >= st.Scroll+visH {
		st.Scroll = line - visH + 1
	}
}

// outputDialogVisH returns the dialog's visible row count (at least 1).
func (h *Handler) outputDialogVisH(st dialog.CommandOutputDialogState) int {
	w, ht := h.screen.Size()
	return max(1, dialog.CommandOutputDialogListH(h.host.LayoutForTerminalSize(w, ht), st))
}

// scrollOutputDialog applies a scroll key to the dialog's Lines (valid in both Running and
// finished states) and keeps Follow in sync: reaching the bottom follows, leaving it unfollows.
// Returns false when key is not a scroll key.
func (h *Handler) scrollOutputDialog(st *dialog.CommandOutputDialogState, key tcell.Key) bool {
	switch key {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
	default:
		return false
	}
	visH := h.outputDialogVisH(*st)
	maxScroll := max(0, len(st.Lines)-visH)
	if st.Follow {
		st.Scroll = maxScroll
	}
	switch key {
	case tcell.KeyUp:
		st.Scroll = max(0, st.Scroll-1)
	case tcell.KeyDown:
		st.Scroll = min(maxScroll, st.Scroll+1)
	case tcell.KeyPgUp:
		st.Scroll = max(0, st.Scroll-visH)
	case tcell.KeyPgDn:
		st.Scroll = min(maxScroll, st.Scroll+visH)
	case tcell.KeyHome:
		st.Scroll = 0
	case tcell.KeyEnd:
		st.Scroll = maxScroll
	}
	st.Follow = st.Scroll >= maxScroll
	return true
}
