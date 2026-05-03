package keymap

import "strings"

// DefaultJobsOverlayKeys holds built-in chords that apply only while the
// jobs view is focused ([jobs_action_keys]). jobs.open defaults live in
// DefaultActionSpecs → [action_keys] because opening the jobs screen must
// work from browser/dialog contexts without consulting the overlay.
//
// Most entries deliberately overlap global chords (e.g. F8 clear vs F8
// delete on panels).
func DefaultJobsOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionJobsCancel:        {"C-c"},
		ActionJobsPause:         {"C-p"},
		ActionJobsResume:        {"C-r"},
		ActionJobsQueueUp:       {"C-up"},
		ActionJobsQueueDown:     {"C-down"},
		ActionJobsClearFinished: {"F8"},
	}
}

// AllowedInJobsOverlay reports whether actionID may appear under [jobs_action_keys].
func AllowedInJobsOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "jobs.")
}
