package menu

import "testing"

// allMenuSets returns every menu builder variant (dedup in both tree modes).
func allMenuSets() map[string][]Definition {
	return map[string][]Definition{
		"browser":   Definitions(),
		"compare":   DefinitionsCompare(false),
		"dedup":     DefinitionsDedup(false, false),
		"dedup/dir": DefinitionsDedup(false, true),
		"jobs":      DefinitionsJobs(),
		"commands":  DefinitionsCommands(),
		"messages":  DefinitionsMessages(),
	}
}

// TestMenuShortcutsConsistent: every action keeps one menu letter in every menu,
// and a letter never repeats within a single pulldown.
func TestMenuShortcutsConsistent(t *testing.T) {
	seen := map[string]rune{}
	for name, defs := range allMenuSets() {
		for _, d := range defs {
			dup := map[rune]string{}
			for _, it := range d.Items {
				if it.Separator || it.Action == "" {
					continue
				}
				if prev, ok := dup[it.Shortcut]; ok {
					t.Errorf("%s/%s: %q used by %s and %s", name, d.Label, it.Shortcut, prev, it.Action)
				}
				dup[it.Shortcut] = it.Action
				if r, ok := seen[it.Action]; ok && r != it.Shortcut {
					t.Errorf("%s/%s: %s has %q, elsewhere %q", name, d.Label, it.Action, it.Shortcut, r)
				}
				seen[it.Action] = it.Shortcut
			}
		}
	}
}
