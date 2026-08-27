package menu

import (
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// FunctionKey describes a labeled footer key with an optional hint.
type FunctionKey struct {
	Key             tcell.Key // tcell.KeyF1 etc.
	KeyLabel        string    // "F1"
	Hint            string    // primary footer label suffix (e.g. "Mkdir")
	HintShiftPrefix string    // Shift-alternative suffix shown after Hint (e.g. "Ren" in MovRen)
	// RequiresActiveJob marks jobs-view actions (cancel/pause/resume/reorder) that are
	// no-ops once the selected job has reached a terminal state; FunctionKeysJobsView
	// hides them in that case.
	RequiresActiveJob bool
	// ActionID is the keymap action bound to this key, used to resolve its vi-motion leader letter.
	ActionID string
}

// FullHint returns the combined footer hint text for width layout.
func (fk FunctionKey) FullHint() string {
	return fk.HintShiftPrefix + fk.Hint
}

// FooterEscClose is prepended to dialog and menu footers where Esc dismisses the overlay.
var FooterEscClose = FunctionKey{Key: tcell.KeyEsc, KeyLabel: "Esc", Hint: "Close"}

// FunctionKeyEditConfig opens meta.toml or menu.toml from meta/user-menu dialogs.
var FunctionKeyEditConfig = FunctionKey{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Edit config"}

// FunctionKeyLeaderMenuToggleChords toggles direct keybind hints in the Esc function menu.
var FunctionKeyLeaderMenuToggleChords = FunctionKey{Key: tcell.KeyF3, KeyLabel: "F3", Hint: "Toggle chords"}

// FunctionKeys is the single source of truth for all F-keys shown in the footer
// and used to route quick-filter function-key presses to menu items.
var FunctionKeys = []FunctionKey{
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help", ActionID: keymap.ActionAppShowHelp},
	{Key: tcell.KeyF2, KeyLabel: "F2", HintShiftPrefix: "Edit", Hint: "UserCmd", ActionID: keymap.ActionAppUserMenu},
	{Key: tcell.KeyF3, KeyLabel: "F3", HintShiftPrefix: "Quick", Hint: "View", ActionID: keymap.ActionFileView},
	{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Edit", ActionID: keymap.ActionFileEdit},
	{Key: tcell.KeyF5, KeyLabel: "F5", HintShiftPrefix: "Duplicate", Hint: "Copy", ActionID: keymap.ActionCopy},
	{Key: tcell.KeyF6, KeyLabel: "F6", HintShiftPrefix: "Ren", Hint: "Mov", ActionID: keymap.ActionMove},
	{Key: tcell.KeyF7, KeyLabel: "F7", HintShiftPrefix: "Open", Hint: "Mkdir", ActionID: keymap.ActionFileMkdir},
	{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Delete", ActionID: keymap.ActionFileDelete},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu", ActionID: keymap.ActionAppOpenMenu},
	{Key: tcell.KeyF10, KeyLabel: "F10", HintShiftPrefix: "Now", Hint: "Quit", ActionID: keymap.ActionAppQuit},
}

// FunctionKeyLabelByKey returns the F-key label for a tcell.Key, e.g. tcell.KeyF5 → "F5".
func FunctionKeyLabelByKey(k tcell.Key) (string, bool) {
	for _, fk := range FunctionKeys {
		if fk.Key == k {
			return fk.KeyLabel, true
		}
	}
	return "", false
}

// FunctionKeysJobs is the footer legend while the jobs view is active.
var FunctionKeysJobs = []FunctionKey{
	FooterEscClose,
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help", ActionID: keymap.ActionAppShowHelp},
	{Key: tcell.KeyCtrlC, KeyLabel: "C-c", Hint: "Cancel", RequiresActiveJob: true, ActionID: keymap.ActionJobsCancel},
	{Key: tcell.KeyCtrlP, KeyLabel: "C-p", Hint: "Pause", RequiresActiveJob: true, ActionID: keymap.ActionJobsPause},
	{Key: tcell.KeyCtrlR, KeyLabel: "C-r", Hint: "Resume", RequiresActiveJob: true, ActionID: keymap.ActionJobsResume},
	{Key: tcell.KeyUp, KeyLabel: "C-up", Hint: "Move up", RequiresActiveJob: true, ActionID: keymap.ActionJobsQueueUp},
	{Key: tcell.KeyDown, KeyLabel: "C-down", Hint: "Move down", RequiresActiveJob: true, ActionID: keymap.ActionJobsQueueDown},
	{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Clear", ActionID: keymap.ActionJobsClearFinished},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu", ActionID: keymap.ActionAppOpenMenu},
	{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit", ActionID: keymap.ActionAppQuit},
}

// FunctionKeysJobsView returns hints for the jobs screen footer. selectedFinished
// hides the cancel/pause/resume/reorder actions, which are no-ops on a completed,
// failed, or canceled job.
func FunctionKeysJobsView(selectedFinished bool) []FunctionKey {
	if !selectedFinished {
		return FunctionKeysJobs
	}
	out := make([]FunctionKey, 0, len(FunctionKeysJobs))
	for _, fk := range FunctionKeysJobs {
		if fk.RequiresActiveJob {
			continue
		}
		out = append(out, fk)
	}
	return out
}

// FunctionKeysCommandsView is the footer legend while the Commands screen is active.
var FunctionKeysCommands = []FunctionKey{
	FooterEscClose,
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help", ActionID: keymap.ActionAppShowHelp},
	{Key: tcell.KeyF8, KeyLabel: "F8", HintShiftPrefix: "Kill", Hint: "Term", ActionID: keymap.ActionCommandsTerminate},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu", ActionID: keymap.ActionAppOpenMenu},
	{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit", ActionID: keymap.ActionAppQuit},
}

// FunctionKeysCommandsView returns hints for the Commands screen footer.
func FunctionKeysCommandsView() []FunctionKey { return FunctionKeysCommands }

// FunctionKeysMessagesView is the footer legend while the Messages screen is active.
var FunctionKeysMessages = []FunctionKey{
	FooterEscClose,
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help", ActionID: keymap.ActionAppShowHelp},
	{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Clear", ActionID: keymap.ActionMessagesClear},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu", ActionID: keymap.ActionAppOpenMenu},
	{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit", ActionID: keymap.ActionAppQuit},
}

// FunctionKeysMessagesView returns hints for the Messages screen footer.
func FunctionKeysMessagesView() []FunctionKey { return FunctionKeysMessages }

// FunctionKeysFilePreviewStylePicker is the footer legend while the F3 style picker is open.
func FunctionKeysFilePreviewStylePicker() []FunctionKey {
	return []FunctionKey{
		FooterEscClose,
		{Key: tcell.KeyEnter, KeyLabel: "Enter", Hint: "Save"},
		{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
	}
}

// FunctionKeysFilePreviewView is the footer legend while the full-screen file view is active.
// Esc is the primary exit key; Left also closes the view. rawMarkdown reflects
// file.view.toggle-raw's current state: the F6 hint reads "Raw" when showing rendered
// markdown (F6 would switch to raw source) and "Render" when already showing raw source.
// launchedAsFileViewer is true when the app was started directly via `pc <file>`, where Esc
// quits the app just like F10 -- the separate "Esc Close" entry is omitted since it would
// duplicate F10 Quit rather than describe a distinct action. showToggleRaw is false for files
// file.view.toggle-raw would no-op on (non-markdown, or a git diff), in which case the F6 entry
// is omitted entirely rather than advertising an action that does nothing. F3 is reserved for
// toggling direct-key chord hints in the `:` leader menu (menu.FunctionKeyLeaderMenuToggleChords),
// matching the file-list view, so it is not listed here.
func FunctionKeysFilePreviewView(rawMarkdown, launchedAsFileViewer, showToggleRaw bool) []FunctionKey {
	toggleHint := "Raw"
	if rawMarkdown {
		toggleHint = "Render"
	}
	out := []FunctionKey{}
	if !launchedAsFileViewer {
		out = append(out, FooterEscClose)
	}
	out = append(out,
		FunctionKey{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
		FunctionKey{Key: tcell.KeyRune, KeyLabel: "/", Hint: "Search"},
		FunctionKey{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Edit"},
		FunctionKey{Key: tcell.KeyF5, KeyLabel: "F5", Hint: "Reload"},
	)
	if showToggleRaw {
		out = append(out, FunctionKey{Key: tcell.KeyF6, KeyLabel: "F6", Hint: toggleHint})
	}
	return append(out,
		FunctionKey{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Delete this"},
		FunctionKey{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Style"},
		FunctionKey{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
	)
}

// FunctionKeysSelectionsStripView returns hints for the footer while the selections strip has keyboard focus.
// F3 View and F4 Edit use the same handlers as the file list, targeting the highlighted strip row.
// F2 selects the unique parent directories of the whole selection (panel.select-parent-dirs), replacing its normal
// global meaning (open the user-command menu) only in this context.
func FunctionKeysSelectionsStripView(clearSelectionLabel string) []FunctionKey {
	out := []FunctionKey{
		{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
		{Key: tcell.KeyF2, KeyLabel: "F2", Hint: "Parent dirs"},
		{Key: tcell.KeyF3, KeyLabel: "F3", Hint: "View"},
		{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Edit"},
	}
	if clearSelectionLabel != "" {
		out = append(out, FunctionKey{KeyLabel: clearSelectionLabel, Hint: "Unselect all"})
	}
	out = append(out, FunctionKey{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"})
	return out
}

var fKeyNum = map[tcell.Key]int{
	tcell.KeyF1: 1, tcell.KeyF2: 2, tcell.KeyF3: 3, tcell.KeyF4: 4,
	tcell.KeyF5: 5, tcell.KeyF6: 6, tcell.KeyF7: 7, tcell.KeyF8: 8,
	tcell.KeyF9: 9, tcell.KeyF10: 10, tcell.KeyF11: 11, tcell.KeyF12: 12,
}

// FKeyNum returns the numeric F-key index (1..12) for a known F-key.
func FKeyNum(k tcell.Key) (int, bool) {
	n, ok := fKeyNum[k]
	return n, ok
}

// KeyLabel returns the F-key label for a numeric key, e.g. 5 → "F5".
func KeyLabel(n int) string {
	return "F" + strconv.Itoa(n)
}

// FindItemByFKeyLabel returns the first non-separator item whose KeyLabel equals label (e.g. "F5").
func FindItemByFKeyLabel(defs []Definition, label string) (def Definition, item Item, ok bool) {
	if label == "" {
		return Definition{}, Item{}, false
	}
	for _, d := range defs {
		for _, it := range d.Items {
			if it.Separator || it.KeyLabel != label {
				continue
			}
			return d, it, true
		}
	}
	return Definition{}, Item{}, false
}
