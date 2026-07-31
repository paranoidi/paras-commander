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

// leaderMenuGroupColumn assigns each group to a macro column (0=File, 1=Selection+Tools, 2=rest).
var leaderMenuGroupColumn = map[string]int{
	LeaderMenuGroupFile:       0,
	LeaderMenuGroupSelection:  1,
	LeaderMenuGroupView:       1,
	LeaderMenuGroupTools:      1,
	LeaderMenuGroupNavigation: 2,
	LeaderMenuGroupDisplay:    2,
	LeaderMenuGroupApp:        2,
}

var leaderMenuGroupOrder = []string{
	LeaderMenuGroupFile,
	LeaderMenuGroupSelection,
	LeaderMenuGroupView,
	LeaderMenuGroupTools,
	LeaderMenuGroupNavigation,
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
		ActionFileExtract,
		ActionFileFlatten,
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
	},
	LeaderMenuGroupTools: {
		ActionPanelFindDialog,
		ActionPanelFindDuplicates,
		ActionPanelComparePanels,
		ActionFileRunForEach,
	},
	LeaderMenuGroupNavigation: {
		ActionPanelHistoryDialog,
		ActionPanelRefresh,
		ActionBookmarkOpen,
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

// BuildLeaderMenuEntries returns grouped built-in menu rows in display order.
func BuildLeaderMenuEntries(keys map[string]string) []LeaderMenuEntry {
	if len(keys) == 0 {
		return nil
	}
	var out []LeaderMenuEntry
	for _, group := range leaderMenuGroupOrder {
		var groupItems []LeaderMenuEntry
		for _, actionID := range leaderMenuGroupActions[group] {
			if ent, ok := leaderMenuEntryForAction(keys, actionID); ok {
				groupItems = append(groupItems, ent)
			}
		}
		if len(groupItems) == 0 {
			continue
		}
		col := leaderMenuGroupColumn[group]
		out = append(out, LeaderMenuEntry{GroupTitle: group, GroupColumn: col})
		for i := range groupItems {
			groupItems[i].GroupColumn = col
			out = append(out, groupItems[i])
		}
	}
	return out
}

// LeaderMenuEntries returns built-in Esc function-menu rows from the bundle's merged keys.
func (b *Bundle) LeaderMenuEntries() []LeaderMenuEntry {
	if b == nil || len(b.LeaderKey) == 0 {
		return nil
	}
	return BuildLeaderMenuEntries(b.LeaderKey)
}
