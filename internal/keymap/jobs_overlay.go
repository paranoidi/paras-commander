package keymap

import "strings"

// DefaultJobsOverlayKeys holds built-in chords that apply only while the
// jobs view is focused ([jobs]). jobs.open defaults live in
// DefaultActionSpecs → [main] because opening the jobs screen must
// work from browser/dialog contexts without consulting the overlay.
//
// Most entries deliberately overlap global chords (e.g. F8 clear-finished
// vs F8 delete on panels).
func DefaultJobsOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionJobsClose:         {"left"},
		ActionJobsMaximizePane:  {"enter"},
		ActionJobsCancel:        {"C-c"},
		ActionJobsPause:         {"C-p"},
		ActionJobsResume:        {"C-r"},
		ActionJobsQueueUp:       {"C-up"},
		ActionJobsQueueDown:     {"C-down"},
		ActionJobsClearFinished: {"F8"},

		ActionJobsRateLimitIncrease: {"+"},
		ActionJobsRateLimitDecrease: {"-"},
		ActionJobsRateLimitClear:    {"C-u"},
		ActionJobsUndo:              {"C-z"},
		ActionJobsResultFilter:      {"F3"},
		ActionJobsResetFilter:       {"S-F3"},
		ActionJobsExpand:            {"M-right"},
		ActionJobsCollapse:          {"M-left"},

		ActionJobsToggleOperationsLayout: {"C-space"},
	}
}

// AllowedInJobsOverlay reports whether actionID may appear under [jobs].
func AllowedInJobsOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "jobs.")
}
