package keymap

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Binding pairs a parsed chord with its action ID and original string form.
type Binding struct {
	ActionID string
	KeyStr   string
	Chord    Chord
}

// ActionBindings returns every (actionID, keyStr, chord) tuple in the map,
// preserving the order they were originally inserted.
func (m *Map) ActionBindings() []Binding {
	if m == nil {
		return nil
	}
	// Collect unique action order first.
	seen := make(map[string]bool)
	var actionOrder []string
	for _, action := range m.keyToAction {
		if !seen[action] {
			seen[action] = true
			actionOrder = append(actionOrder, action)
		}
	}
	// Build reverse map: actionID -> []chord
	rev := make(map[string][]Chord)
	keyStrs := make(map[Chord]string)
	// Pre-populate with TOML original forms from every default registry
	// (specs + jobs overlay) so we keep canonical representations for
	// display even when the chord would otherwise round-trip through
	// FormatChord (e.g. "C-j" instead of "Ctrl+J").
	rememberCanonical := func(ks string) {
		ch, err := ParseKey(ks)
		if err != nil {
			return
		}
		if _, seen := keyStrs[ch]; !seen {
			keyStrs[ch] = ks
		}
	}
	for _, spec := range DefaultActionSpecs() {
		for _, ks := range spec.DefaultKeys {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultJobsOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultCommandsOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultMessagesOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultDialogInputOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultRenameDialogOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultMkdirDialogOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for _, chords := range DefaultBookmarkDialogOverlayKeys() {
		for _, ks := range chords {
			rememberCanonical(ks)
		}
	}
	for ch, action := range m.keyToAction {
		rev[action] = append(rev[action], ch)
		if _, ok := keyStrs[ch]; !ok {
			keyStrs[ch] = FormatChord(ch)
		}
	}
	var out []Binding
	for _, action := range actionOrder {
		chords := rev[action]
		for _, ch := range chords {
			ks := keyStrs[ch]
			out = append(out, Binding{
				ActionID: action,
				KeyStr:   ks,
				Chord:    ch,
			})
		}
	}
	return out
}

// BindingsForAction returns all chord strings bound to actionID, or nil.
func (m *Map) BindingsForAction(actionID string) []string {
	if m == nil {
		return nil
	}
	var out []string
	for _, b := range m.ActionBindings() {
		if b.ActionID == actionID {
			out = append(out, b.KeyStr)
		}
	}
	return out
}

func menuBindingLabelPick(spec ActionSpec, hasSpec bool, keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	if hasSpec && spec.PreferredKey != "" {
		for _, k := range keys {
			if k == spec.PreferredKey {
				return k
			}
		}
	}
	return keys[0]
}

// MenuBindingLabel returns one key string for pulldown menus: the PreferredKey from the action
// spec when it appears among bindings, otherwise the first binding from the map, otherwise the
// first default key from the spec. Empty means unbound with no defaults to show.
func (m *Map) MenuBindingLabel(actionID string) string {
	spec, hasSpec := SpecForAction(actionID)
	var keys []string
	if m != nil {
		keys = m.BindingsForAction(actionID)
	}
	if len(keys) == 0 && hasSpec {
		keys = append(keys, spec.DefaultKeys...)
	}
	return menuBindingLabelPick(spec, hasSpec, keys)
}

// MenuBindingLabelPreferCommands resolves a menu hint using the Commands overlay when present,
// otherwise falls back to the global map.
func MenuBindingLabelPreferCommands(global, commands *Map, actionID string) string {
	spec, hasSpec := SpecForAction(actionID)
	if commands != nil {
		if ks := commands.BindingsForAction(actionID); len(ks) > 0 {
			return menuBindingLabelPick(spec, hasSpec, ks)
		}
	}
	if global != nil {
		return global.MenuBindingLabel(actionID)
	}
	return ""
}

// MenuBindingLabelPreferMessages resolves a menu hint using the Messages overlay when present,
// otherwise falls back to the global map.
func MenuBindingLabelPreferMessages(global, messages *Map, actionID string) string {
	spec, hasSpec := SpecForAction(actionID)
	if messages != nil {
		if ks := messages.BindingsForAction(actionID); len(ks) > 0 {
			return menuBindingLabelPick(spec, hasSpec, ks)
		}
	}
	if global != nil {
		return global.MenuBindingLabel(actionID)
	}
	return ""
}

// MenuBindingLabelPreferJobs resolves a menu hint using overlay bindings only when present,
// otherwise falls back to the global map (avoids duplicating global defaults via the overlay map).
func MenuBindingLabelPreferJobs(global, jobs *Map, actionID string) string {
	spec, hasSpec := SpecForAction(actionID)
	if jobs != nil {
		if ks := jobs.BindingsForAction(actionID); len(ks) > 0 {
			return menuBindingLabelPick(spec, hasSpec, ks)
		}
	}
	if global != nil {
		return global.MenuBindingLabel(actionID)
	}
	return ""
}

// FormatChord returns a human-readable string for a parsed chord,
// suitable for display in help screens.
// Examples: "Ctrl+D", "Alt+←", "Shift+Tab", "F5", "Enter", "Ctrl+Alt+D".
func FormatChord(ch Chord) string {
	var parts []string

	// Ctrl may be encoded in the key type (KeyCtrlA..KeyCtrlZ) after
	// CanonicalChord clears the ModCtrl bit. Detect it here.
	isCtrlKey := ch.Key == tcell.KeyCtrlSpace || (ch.Key >= tcell.KeyCtrlA && ch.Key <= tcell.KeyCtrlZ)

	// Build in canonical order: modifiers first, then key name.
	if mod := ch.Mod; mod != 0 {
		if mod&tcell.ModCtrl != 0 {
			parts = append(parts, "Ctrl")
		}
		if mod&tcell.ModAlt != 0 {
			parts = append(parts, "Alt")
		}
		if mod&tcell.ModShift != 0 {
			parts = append(parts, "Shift")
		}
	}
	// Add Ctrl for KeyCtrl* keys whose ModCtrl was cleared.
	if isCtrlKey && !contains(parts, "Ctrl") {
		parts = append(parts, "Ctrl")
	}

	kn := chordKeyName(ch, isCtrlKey)
	if kn != "" {
		parts = append(parts, kn)
	}
	if len(parts) == 0 {
		return "?"
	}
	return strings.Join(parts, "+")
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// chordKeyName returns the display name for a chord's key part.
// isCtrlKey is true when the chord is a KeyCtrl* type — the function
// returns the bare letter in that case (the "Ctrl+" prefix is added by FormatChord).
func chordKeyName(ch Chord, isCtrlKey bool) string {
	if ch.Key == tcell.KeyRune {
		r := ch.Rune
		if ch.Mod&tcell.ModAlt != 0 && unicode.IsLetter(r) {
			r = unicode.ToUpper(r)
		}
		return string(r)
	}
	if ch.Key == tcell.KeyCtrlSpace {
		return "Space"
	}
	if isCtrlKey {
		return string('A' + rune(ch.Key-tcell.KeyCtrlA))
	}
	switch ch.Key {
	case tcell.KeyUp:
		return "↑"
	case tcell.KeyDown:
		return "↓"
	case tcell.KeyLeft:
		return "←"
	case tcell.KeyRight:
		return "→"
	case tcell.KeyPgUp:
		return "PgUp"
	case tcell.KeyPgDn:
		return "PgDn"
	case tcell.KeyHome:
		return "Home"
	case tcell.KeyEnd:
		return "End"
	case tcell.KeyTab:
		return "Tab"
	case tcell.KeyBacktab:
		return "Shift+Tab"
	case tcell.KeyEnter:
		return "Enter"
	case tcell.KeyEsc:
		return "Esc"
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "Backspace"
	case tcell.KeyInsert:
		return "Insert"
	case tcell.KeyDelete:
		return "Delete"
	}
	if ch.Key >= tcell.KeyF1 && ch.Key <= tcell.KeyF12 {
		n := int(ch.Key-tcell.KeyF1) + 1
		return fmt.Sprintf("F%d", n)
	}
	return ""
}
