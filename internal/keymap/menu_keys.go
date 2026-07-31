package keymap

import (
	"fmt"
	"strings"
)

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
