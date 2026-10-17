package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	if h.scrollOutputDialog(st, event.Key()) {
		return
	}
	if st.Running {
		switch {
		case event.Key() == tcell.KeyEsc || dialog.AltLetter(event, 'b'):
			h.closeOutputDialogKeepRunning()
		case dialog.AltDialogCancel(event):
			h.CloseOutputDialog()
		case event.Key() == tcell.KeyLeft:
			st.Focus = 0
		case event.Key() == tcell.KeyRight:
			st.Focus = 1
		case event.Key() == tcell.KeyEnter:
			if st.Focus == 0 {
				h.closeOutputDialogKeepRunning()
			} else {
				h.CloseOutputDialog()
			}
		}
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		h.CloseOutputDialog()
	case tcell.KeyRune:
		if dialog.AltDialogOK(event) || dialog.AltDialogCancel(event) {
			h.CloseOutputDialog()
		}
	}
}

// OutputDialogResult builds the finished-state output dialog for a completed run.
func OutputDialogResult(runID, title string, res cmdrun.RunResult, prefW, prefH string) dialog.CommandOutputDialogState {
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
	return dialog.CommandOutputDialogState{
		Open:       true,
		RunID:      runID,
		Title:      dialogTitle,
		Lines:      outputDialogLines(string(res.Stdout), stderr),
		PrefWidth:  prefW,
		PrefHeight: prefH,
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
