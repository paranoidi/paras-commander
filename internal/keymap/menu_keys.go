package keymap

import (
	"fmt"
	"strings"
)

// letterMenu is one single-letter menu table of keybindings.toml ([leader_key], [copy_menu], ...):
// its built-in letters and its validation.
type letterMenu struct {
	table    string
	defaults func() map[string]string
	validate func(map[string]string) error
}

// letterMenus is the one list of single-letter menu tables: LoadFromPaths layers each, buildBundle
// validates each, parseKeybindingsFile reads each and the stub writes each.
var letterMenus = []letterMenu{
	{LeaderKeyShortcutsTable, DefaultLeaderKeys, validateLeaderKeysPerScope},
	{CopyMenuShortcutsTable, DefaultCopyMenuKeys, validateCopyMenuKeys},
	{PreviewMenuShortcutsTable, DefaultPreviewMenuKeys, validatePreviewMenuKeys},
	{FileIndexMenuShortcutsTable, DefaultFileIndexMenuKeys, validateFileIndexMenuKeys},
}

// defaultLetterMenuKeys returns every letter-menu table's built-in letters, keyed by table name.
func defaultLetterMenuKeys() map[string]map[string]string {
	out := make(map[string]map[string]string, len(letterMenus))
	for _, m := range letterMenus {
		out[m.table] = m.defaults()
	}
	return out
}

// isLetterMenuTable reports whether table is one of letterMenus.
func isLetterMenuTable(table string) bool {
	for _, m := range letterMenus {
		if m.table == table {
			return true
		}
	}
	return false
}

// mergeMenuKeys overlays user single-letter bindings on defaults; empty user value omits the action.
func mergeMenuKeys(defaults, user map[string]string) map[string]string {
	out := make(map[string]string, len(defaults))
	for action, key := range defaults {
		out[action] = key
	}
	for action, key := range user {
		if key == "" {
			delete(out, action)
			continue
		}
		out[action] = key
	}
	return out
}

func validateMenuKeys(table string, keys map[string]string, valid func(rune) bool, invalidDesc string) error {
	seen := make(map[rune]string)
	for action, keyStr := range keys {
		runes := []rune(strings.TrimSpace(keyStr))
		if len(runes) != 1 {
			return fmt.Errorf("%s action %q: key %q must be a single character", table, action, keyStr)
		}
		r := runes[0]
		if !valid(r) {
			return fmt.Errorf("%s action %q: key %q must be %s", table, action, keyStr, invalidDesc)
		}
		if prev, ok := seen[r]; ok {
			return fmt.Errorf("%s: key %q used by both %q and %q", table, string(r), prev, action)
		}
		seen[r] = action
	}
	return nil
}

func actionSpecTitle(actionID string) string {
	if spec, ok := SpecForAction(actionID); ok {
		return spec.Title
	}
	return actionID
}

func parseMenuKeyRune(keys map[string]string, actionID string) (rune, bool) {
	keyStr, ok := keys[actionID]
	if !ok || strings.TrimSpace(keyStr) == "" {
		return 0, false
	}
	runes := []rune(strings.TrimSpace(keyStr))
	if len(runes) != 1 {
		return 0, false
	}
	return runes[0], true
}
