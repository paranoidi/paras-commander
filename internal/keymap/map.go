package keymap

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
)

// Map resolves key events to action IDs.
type Map struct {
	keyToAction map[Chord]string
}

// Lookup returns the bound action ID for event, if any.
func (m *Map) Lookup(ev *tcell.EventKey) (actionID string, ok bool) {
	if m == nil || ev == nil {
		return "", false
	}
	ch := CanonicalChord(EventChord(ev))
	if id, ok := m.keyToAction[ch]; ok {
		return id, true
	}
	// Terminals often deliver shifted punctuation as Rune + ModShift (e.g. Shift+8 → '*'),
	// while TOML binds the symbol without Shift. Try again without redundant Shift.
	if ch.Key == tcell.KeyRune && ch.Mod&tcell.ModShift != 0 {
		ch2 := CanonicalChord(Chord{Key: ch.Key, Rune: ch.Rune, Mod: ch.Mod &^ tcell.ModShift})
		id, ok := m.keyToAction[ch2]
		return id, ok
	}
	return "", false
}

// FirstEventKeyForAction returns a stable *tcell.EventKey that Lookup resolves to actionID.
// Intended for tests; the chord is chosen deterministically when multiple keys bind the same action.
func (m *Map) FirstEventKeyForAction(actionID string) (*tcell.EventKey, bool) {
	if m == nil {
		return nil, false
	}
	var chords []Chord
	for ch, id := range m.keyToAction {
		if id == actionID {
			chords = append(chords, ch)
		}
	}
	if len(chords) == 0 {
		return nil, false
	}
	sort.Slice(chords, func(i, j int) bool {
		a, b := chords[i], chords[j]
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		if a.Mod != b.Mod {
			return a.Mod < b.Mod
		}
		return a.Rune < b.Rune
	})
	for _, ch := range chords {
		ev := chordToEventKey(ch)
		if id, ok := m.Lookup(ev); ok && id == actionID {
			return ev, true
		}
	}
	return nil, false
}

func chordToEventKey(ch Chord) *tcell.EventKey {
	if ch.Key == tcell.KeyRune {
		return tcell.NewEventKey(tcell.KeyRune, ch.Rune, ch.Mod)
	}
	return tcell.NewEventKey(ch.Key, ch.Rune, ch.Mod)
}

// Build parses bindings and validates uniqueness of every chord.
func Build(bindings map[string][]string) (*Map, error) {
	keyToAction := make(map[Chord]string)
	for action, keys := range bindings {
		if _, known := KnownActions[action]; !known {
			return nil, fmt.Errorf("unknown action %q", action)
		}
		for _, ks := range keys {
			ch, err := ParseKey(ks)
			if err != nil {
				return nil, fmt.Errorf("action %q: %w", action, err)
			}
			if existing, taken := keyToAction[ch]; taken {
				if existing != action {
					return nil, fmt.Errorf("key %q bound to both %q and %q", ks, existing, action)
				}
				return nil, fmt.Errorf("action %q: duplicate key %q", action, ks)
			}
			keyToAction[ch] = action
		}
	}
	return &Map{keyToAction: keyToAction}, nil
}

// Default returns the built-in global key map (no config files).
func Default() (*Map, error) {
	return Build(DefaultActionKeys())
}
