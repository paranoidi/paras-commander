package menu

import (
	"strconv"

	"github.com/gdamore/tcell/v2"
)

// FunctionKey describes a labeled footer key with an optional hint.
type FunctionKey struct {
	Key             tcell.Key // tcell.KeyF1 etc.
	KeyLabel        string    // "F1"
	Hint            string    // primary footer label suffix (e.g. "Mkdir")
	HintShiftPrefix string    // Shift-alternative suffix shown after Hint (e.g. "Ren" in MovRen)
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
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
	{Key: tcell.KeyF2, KeyLabel: "F2", HintShiftPrefix: "Edit", Hint: "UserCmd"},
	{Key: tcell.KeyF3, KeyLabel: "F3", HintShiftPrefix: "Quick", Hint: "View"},
	{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Edit"},
	{Key: tcell.KeyF5, KeyLabel: "F5", HintShiftPrefix: "Duplicate", Hint: "Copy"},
	{Key: tcell.KeyF6, KeyLabel: "F6", HintShiftPrefix: "Ren", Hint: "Mov"},
	{Key: tcell.KeyF7, KeyLabel: "F7", HintShiftPrefix: "Open", Hint: "Mkdir"},
	{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Delete"},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
	{Key: tcell.KeyF10, KeyLabel: "F10", HintShiftPrefix: "Now", Hint: "Quit"},
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
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
	{Key: tcell.KeyCtrlC, KeyLabel: "C-c", Hint: "Cancel"},
	{Key: tcell.KeyCtrlP, KeyLabel: "C-p", Hint: "Pause"},
	{Key: tcell.KeyCtrlR, KeyLabel: "C-r", Hint: "Resume"},
	{Key: tcell.KeyUp, KeyLabel: "C-up", Hint: "Move up"},
	{Key: tcell.KeyDown, KeyLabel: "C-down", Hint: "Move down"},
	{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Clear"},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
	{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
}

// FunctionKeysJobsView returns hints for the jobs screen footer.
func FunctionKeysJobsView() []FunctionKey { return FunctionKeysJobs }

// FunctionKeysCommandsView is the footer legend while the Commands screen is active.
var FunctionKeysCommands = []FunctionKey{
	FooterEscClose,
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
	{Key: tcell.KeyF8, KeyLabel: "F8", HintShiftPrefix: "Kill", Hint: "Term"},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
	{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
}

// FunctionKeysCommandsView returns hints for the Commands screen footer.
func FunctionKeysCommandsView() []FunctionKey { return FunctionKeysCommands }

// FunctionKeysMessagesView is the footer legend while the Messages screen is active.
var FunctionKeysMessages = []FunctionKey{
	FooterEscClose,
	{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
	{Key: tcell.KeyF8, KeyLabel: "F8", Hint: "Clear"},
	{Key: tcell.KeyF9, KeyLabel: "F9", Hint: "Menu"},
	{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"},
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
func FunctionKeysSelectionsStripView(clearSelectionLabel string) []FunctionKey {
	out := []FunctionKey{
		{Key: tcell.KeyF1, KeyLabel: "F1", Hint: "Help"},
		{Key: tcell.KeyF3, KeyLabel: "F3", Hint: "View"},
		{Key: tcell.KeyF4, KeyLabel: "F4", Hint: "Edit"},
	}
	if clearSelectionLabel != "" {
		out = append(out, FunctionKey{KeyLabel: clearSelectionLabel, Hint: "Unselect all"})
	}
	out = append(out, FunctionKey{Key: tcell.KeyF10, KeyLabel: "F10", Hint: "Quit"})
	return out
}

// FunctionKeyHints returns a label list suitable for footer rendering.
func FunctionKeyHints() []FunctionKey { return FunctionKeys }

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
