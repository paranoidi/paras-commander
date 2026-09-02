package keymap

import (
	"strings"
	"unicode"
)

// Built-in Esc function-menu group titles (display order).
const (
	LeaderMenuGroupFile       = "File"
	LeaderMenuGroupSelection  = "Selection"
	LeaderMenuGroupView       = "View"
	LeaderMenuGroupTools      = "Tools"
	LeaderMenuGroupNavigation = "Navigation"
	LeaderMenuGroupDisplay    = "Display"
	LeaderMenuGroupApp        = "App"
)

// Per-view leader-menu group titles (used by leaderMenuViewSpecs below), in addition to the
// shared LeaderMenuGroupApp/LeaderMenuGroupDisplay groups every per-view menu also carries.
const (
	LeaderMenuGroupCompare    = "Compare"
	LeaderMenuGroupQueue      = "Queue"
	LeaderMenuGroupCommands   = "Commands"
	LeaderMenuGroupMessages   = "Messages"
	LeaderMenuGroupTree       = "Tree"
	LeaderMenuGroupDuplicates = "Duplicates"
)

// leaderMenuGroupColumn assigns each group to a macro column
// (0=File, 1=Selection+View, 2=Tools, 3=Navigation+Display+App).
var leaderMenuGroupColumn = map[string]int{
	LeaderMenuGroupFile:       0,
	LeaderMenuGroupSelection:  1,
	LeaderMenuGroupView:       1,
	LeaderMenuGroupNavigation: 2,
	LeaderMenuGroupTools:      2,
	LeaderMenuGroupDisplay:    3,
	LeaderMenuGroupApp:        3,
}

var leaderMenuGroupOrder = []string{
	LeaderMenuGroupFile,
	LeaderMenuGroupSelection,
	LeaderMenuGroupView,
	LeaderMenuGroupNavigation,
	LeaderMenuGroupTools,
	LeaderMenuGroupDisplay,
	LeaderMenuGroupApp,
}

// leaderMenuGroupActions is display order within each group (single source for row layout).
var leaderMenuGroupActions = map[string][]string{
	LeaderMenuGroupFile: {
		ActionCopy,
		ActionMove,
		ActionFileRename,
		ActionFileDelete,
		ActionFileEdit,
		ActionFileMkdir,
		ActionFileMkdirOpenInOther,
		ActionFileDuplicate,
		ActionFileView,
		ActionFileQuickView,
	},
	LeaderMenuGroupSelection: {
		ActionPanelSelectGroup,
		ActionPanelUnselectGroup,
		ActionPanelInvertSelection,
		ActionPanelClearSelection,
	},
	LeaderMenuGroupView: {
		ActionPanelToggleHidden,
		ActionPanelMeta,
		ActionPanelFilterDialog,
		ActionPanelDiskUsageScan,
	},
	LeaderMenuGroupTools: {
		ActionPanelFindDialog,
		ActionPanelFindDuplicates,
		ActionPanelComparePanels,
		ActionFileRunForEach,
		ActionFileExtract,
		ActionFileFlatten,
	},
	LeaderMenuGroupNavigation: {
		ActionPanelHistoryDialog,
		ActionPanelRefresh,
		ActionBookmarkOpen,
		ActionPanelPinDialog,
	},
	LeaderMenuGroupDisplay: {
		ActionJobsOpen,
		ActionMessagesOpen,
		ActionCommandsOpen,
		ActionAppDropToShell,
	},
	LeaderMenuGroupApp: {
		ActionAppShowHelp,
		ActionAppQuit,
	},
}

// leaderMenuViewSpec is one view's leader-menu group order/membership table. column assigns
// each of the view's groups to a macro column: the view's own group(s) come first (column 0,
// 1, ...), with the shared Display/App group(s) always last (rightmost), mirroring the
// browser leader menu's Navigation/Display/App column order.
type leaderMenuViewSpec struct {
	order   []string
	actions map[string][]string
	column  map[string]int
}

// leaderMenuViewSpecs is the per-view source of truth for `:` leader-menu content in the
// Compare, Dedup, Jobs, Commands, and Messages views — parallel to leaderMenuGroupActions
// for the browser's leader menu. Every view carries the same App/Display actions (Quit and
// links to the other auxiliary views) plus its own view-specific group(s). The F2 user menu
// (ActionAppUserMenu) only opens from the browser view, so it is omitted from every per-view
// menu rather than listed as a dead entry.
var leaderMenuViewSpecs = map[HelpViews]leaderMenuViewSpec{
	HelpCompare: {
		order: []string{LeaderMenuGroupCompare, LeaderMenuGroupDisplay, LeaderMenuGroupApp},
		actions: map[string][]string{
			LeaderMenuGroupApp:     {ActionAppQuit},
			LeaderMenuGroupDisplay: {ActionJobsOpen, ActionMessagesOpen, ActionCommandsOpen},
			LeaderMenuGroupCompare: {
				ActionCompareClose,
				ActionCompareCycleFilter,
				ActionCompareResetFilter,
				ActionCompareRefresh,
				ActionCompareMerge,
				ActionCompareToggleEmpty,
				ActionPanelSelectToggle,
				ActionPanelExternalBrowser,
			},
		},
		column: map[string]int{
			LeaderMenuGroupCompare: 0,
			LeaderMenuGroupDisplay: 1,
			LeaderMenuGroupApp:     1,
		},
	},
	HelpDedup: {
		order: []string{LeaderMenuGroupTree, LeaderMenuGroupDuplicates, LeaderMenuGroupDisplay, LeaderMenuGroupApp},
		actions: map[string][]string{
			LeaderMenuGroupApp:     {ActionAppQuit},
			LeaderMenuGroupDisplay: {ActionJobsOpen, ActionMessagesOpen, ActionCommandsOpen},
			LeaderMenuGroupTree: {
				ActionDedupToggleNode,
				ActionDedupCollapse,
				ActionDedupToggleTree,
				ActionDedupCollapseAll,
				ActionDedupExpandAll,
				ActionDedupPrevDir,
				ActionDedupNextDir,
				ActionDedupToggleSort,
				ActionDedupToggleEmpty,
			},
			LeaderMenuGroupDuplicates: {
				ActionPanelRefresh,
				ActionPanelInvertSelection,
				ActionPanelClearSelection,
				ActionFileDelete,
				ActionDedupMarkKeep,
				ActionDedupCompare,
				ActionPanelSelectToggle,
				ActionDedupClose,
			},
		},
		column: map[string]int{
			LeaderMenuGroupTree:       0,
			LeaderMenuGroupDuplicates: 1,
			LeaderMenuGroupDisplay:    2,
			LeaderMenuGroupApp:        2,
		},
	},
	HelpJobs: {
		order: []string{LeaderMenuGroupQueue, LeaderMenuGroupDisplay, LeaderMenuGroupApp},
		actions: map[string][]string{
			LeaderMenuGroupApp:     {ActionAppQuit},
			LeaderMenuGroupDisplay: {ActionMessagesOpen, ActionCommandsOpen},
			LeaderMenuGroupQueue: {
				ActionPanelExternalBrowser,
				ActionJobsAnswerBlocker,
				ActionJobsCancel,
				ActionJobsPause,
				ActionJobsResume,
				ActionJobsQueueUp,
				ActionJobsQueueDown,
				ActionJobsClose,
				ActionJobsClearFinished,
			},
		},
		column: map[string]int{
			LeaderMenuGroupQueue:   0,
			LeaderMenuGroupDisplay: 1,
			LeaderMenuGroupApp:     1,
		},
	},
	HelpCommands: {
		order: []string{LeaderMenuGroupCommands, LeaderMenuGroupDisplay, LeaderMenuGroupApp},
		actions: map[string][]string{
			LeaderMenuGroupApp:     {ActionAppQuit},
			LeaderMenuGroupDisplay: {ActionJobsOpen, ActionMessagesOpen},
			LeaderMenuGroupCommands: {
				ActionPanelExternalBrowser,
				ActionCommandsClose,
				ActionCommandsTerminate,
				ActionCommandsKill,
			},
		},
		column: map[string]int{
			LeaderMenuGroupCommands: 0,
			LeaderMenuGroupDisplay:  1,
			LeaderMenuGroupApp:      1,
		},
	},
	HelpMessages: {
		order: []string{LeaderMenuGroupMessages, LeaderMenuGroupDisplay, LeaderMenuGroupApp},
		actions: map[string][]string{
			LeaderMenuGroupApp:     {ActionAppQuit},
			LeaderMenuGroupDisplay: {ActionJobsOpen, ActionCommandsOpen},
			LeaderMenuGroupMessages: {
				ActionMessagesClose,
				ActionMessagesClear,
			},
		},
		column: map[string]int{
			LeaderMenuGroupMessages: 0,
			LeaderMenuGroupDisplay:  1,
			LeaderMenuGroupApp:      1,
		},
	},
}

// LeaderMenuEntry is one built-in Esc function-menu row (action or group title).
type LeaderMenuEntry struct {
	GroupTitle  string // non-empty = cyan group header row
	GroupColumn int    // macro column 0..2 when GroupTitle set or action row
	ActionID    string
	Key         rune
	Label       string
}

// DefaultLeaderKeys returns built-in default leader-menu letters per action.
func DefaultLeaderKeys() map[string]string {
	out := make(map[string]string)
	for _, spec := range DefaultActionSpecs() {
		if k := strings.TrimSpace(spec.LeaderKey); k != "" {
			out[spec.ID] = k
		}
	}
	return out
}

// mergeLeaderKeys overlays user letters on defaults.
func mergeLeaderKeys(defaults, user map[string]string) map[string]string {
	return mergeMenuKeys(defaults, user)
}

func validLeaderKeyRune(r rune) bool {
	return unicode.IsLetter(r) || r == '?' || r == ',' || r == '.'
}

func validateLeaderKeys(keys map[string]string) error {
	return validateMenuKeys("leader_key", keys, validLeaderKeyRune, "a letter, ?, comma, or period")
}

func leaderMenuEntryForAction(keys map[string]string, actionID string) (LeaderMenuEntry, bool) {
	key, ok := parseMenuKeyRune(keys, actionID)
	if !ok {
		return LeaderMenuEntry{}, false
	}
	return LeaderMenuEntry{
		ActionID: actionID,
		Key:      key,
		Label:    actionSpecTitle(actionID),
	}, true
}

// buildLeaderMenuEntries returns grouped menu rows in display order, shared by
// BuildLeaderMenuEntries (browser) and BuildLeaderMenuEntriesForView (per-view menus).
func buildLeaderMenuEntries(keys map[string]string, order []string, actions map[string][]string, column map[string]int) []LeaderMenuEntry {
	if len(keys) == 0 {
		return nil
	}
	var out []LeaderMenuEntry
	for _, group := range order {
		var groupItems []LeaderMenuEntry
		for _, actionID := range actions[group] {
			if ent, ok := leaderMenuEntryForAction(keys, actionID); ok {
				groupItems = append(groupItems, ent)
			}
		}
		if len(groupItems) == 0 {
			continue
		}
		col := column[group]
		out = append(out, LeaderMenuEntry{GroupTitle: group, GroupColumn: col})
		for i := range groupItems {
			groupItems[i].GroupColumn = col
			out = append(out, groupItems[i])
		}
	}
	return out
}

// BuildLeaderMenuEntries returns grouped built-in menu rows in display order.
func BuildLeaderMenuEntries(keys map[string]string) []LeaderMenuEntry {
	return buildLeaderMenuEntries(keys, leaderMenuGroupOrder, leaderMenuGroupActions, leaderMenuGroupColumn)
}

// BuildLeaderMenuEntriesForView returns grouped `:` leader-menu rows for one auxiliary view
// (Compare, Dedup, Jobs, Commands, or Messages), or nil when vm has no per-view menu.
func BuildLeaderMenuEntriesForView(keys map[string]string, vm HelpViews) []LeaderMenuEntry {
	spec, ok := leaderMenuViewSpecs[vm]
	if !ok {
		return nil
	}
	return buildLeaderMenuEntries(keys, spec.order, spec.actions, spec.column)
}

// LeaderMenuEntries returns built-in Esc function-menu rows from the bundle's merged keys.
func (b *Bundle) LeaderMenuEntries() []LeaderMenuEntry {
	if b == nil || len(b.LeaderKey) == 0 {
		return nil
	}
	return BuildLeaderMenuEntries(b.LeaderKey)
}

// LeaderMenuEntriesForView returns per-view `:` leader-menu rows from the bundle's merged keys.
func (b *Bundle) LeaderMenuEntriesForView(vm HelpViews) []LeaderMenuEntry {
	if b == nil || len(b.LeaderKey) == 0 {
		return nil
	}
	return BuildLeaderMenuEntriesForView(b.LeaderKey, vm)
}

// flattenGroupActions flattens a group-title -> action-ID-list table into the set of action IDs.
func flattenGroupActions(actions map[string][]string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, ids := range actions {
		for _, id := range ids {
			out[id] = struct{}{}
		}
	}
	return out
}

// scopedLeaderKeys filters keys down to the action IDs present in scope.
func scopedLeaderKeys(keys map[string]string, scope map[string]struct{}) map[string]string {
	out := make(map[string]string, len(scope))
	for actionID, key := range keys {
		if _, ok := scope[actionID]; ok {
			out[actionID] = key
		}
	}
	return out
}

// validateLeaderKeysPerScope validates leader-menu key uniqueness independently within each
// rendered menu (the browser menu plus every per-view menu) rather than across the whole flat
// action set. Each rendered menu is a closed set — a keypress only resolves against that menu's
// own item list — so the same letter may legally activate different actions in different views
// (e.g. Compare's `c` = Close vs. Dedup's `c` = Collapse) as long as it is unique within any one
// menu.
func validateLeaderKeysPerScope(keys map[string]string) error {
	if err := validateLeaderKeys(scopedLeaderKeys(keys, flattenGroupActions(leaderMenuGroupActions))); err != nil {
		return err
	}
	for _, spec := range leaderMenuViewSpecs {
		if err := validateLeaderKeys(scopedLeaderKeys(keys, flattenGroupActions(spec.actions))); err != nil {
			return err
		}
	}
	return nil
}
