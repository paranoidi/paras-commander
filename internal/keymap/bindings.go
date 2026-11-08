package keymap

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
)

var (
	cachedCompareOverlayKeys     = sync.OnceValue(DefaultCompareOverlayKeys)
	cachedJobsOverlayKeys        = sync.OnceValue(DefaultJobsOverlayKeys)
	cachedCommandsOverlayKeys    = sync.OnceValue(DefaultCommandsOverlayKeys)
	cachedMessagesOverlayKeys    = sync.OnceValue(DefaultMessagesOverlayKeys)
	cachedDialogInputOverlayKeys = sync.OnceValue(DefaultDialogInputOverlayKeys)
	cachedRenameDialogKeys       = sync.OnceValue(DefaultRenameDialogOverlayKeys)
	cachedMkdirDialogKeys        = sync.OnceValue(DefaultMkdirDialogOverlayKeys)
	cachedBookmarkDialogKeys     = sync.OnceValue(DefaultBookmarkDialogOverlayKeys)
	cachedFindDialogKeys         = sync.OnceValue(DefaultFindDialogOverlayKeys)
	cachedHistoryDialogKeys      = sync.OnceValue(DefaultHistoryDialogOverlayKeys)
	cachedFlattenDialogKeys      = sync.OnceValue(DefaultFlattenDialogOverlayKeys)
	cachedColumnsDialogKeys      = sync.OnceValue(DefaultColumnsDialogOverlayKeys)
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
	m.bindingsOnce.Do(func() { m.bindings = m.buildActionBindings() })
	return slices.Clone(m.bindings)
}

func (m *Map) buildActionBindings() []Binding {
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
	// Pre-populate with the TOML strings from every default registry so KeyStr
	// matches ActionSpec.DefaultKeys / PreferredKey for ordering and picking.
	// Display formatting happens later, through DisplayKey.
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

// BindingsForAction returns all chord strings bound to actionID in stable display order.
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
	return sortMenuBindingKeys(actionID, out)
}

func menuBindingLabelPick(actionID string, spec ActionSpec, hasSpec bool, keys []string) string {
	keys = sortMenuBindingKeys(actionID, keys)
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

// sortMenuBindingKeys orders chord labels deterministically for menus and footers.
func sortMenuBindingKeys(actionID string, keys []string) []string {
	if len(keys) <= 1 {
		return keys
	}
	spec, hasSpec := SpecForAction(actionID)
	order := make(map[string]int, len(keys))
	if hasSpec {
		for i, k := range spec.DefaultKeys {
			if _, ok := order[k]; !ok {
				order[k] = i
			}
		}
	}
	for _, reg := range []map[string][]string{
		cachedCompareOverlayKeys(),
		cachedJobsOverlayKeys(),
		cachedCommandsOverlayKeys(),
		cachedMessagesOverlayKeys(),
		cachedDialogInputOverlayKeys(),
		cachedRenameDialogKeys(),
		cachedMkdirDialogKeys(),
		cachedBookmarkDialogKeys(),
		cachedFindDialogKeys(),
		cachedHistoryDialogKeys(),
		cachedFlattenDialogKeys(),
		cachedColumnsDialogKeys(),
	} {
		chords, ok := reg[actionID]
		if !ok {
			continue
		}
		for i, k := range chords {
			if _, seen := order[k]; !seen {
				order[k] = i
			}
		}
	}
	out := append([]string(nil), keys...)
	slices.SortStableFunc(out, func(a, b string) int {
		pa, oka := order[a]
		pb, okb := order[b]
		switch {
		case oka && okb && pa != pb:
			return pa - pb
		case oka != okb:
			if oka {
				return -1
			}
			return 1
		}
		return menuBindingKeyPriority(a) - menuBindingKeyPriority(b)
	})
	return out
}

func menuBindingKeyPriority(key string) int {
	ch, err := ParseKey(key)
	if err != nil {
		return 100
	}
	switch ch.Key {
	case tcell.KeyEsc:
		return 0
	case tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5, tcell.KeyF6,
		tcell.KeyF7, tcell.KeyF8, tcell.KeyF9, tcell.KeyF10, tcell.KeyF11, tcell.KeyF12:
		return 10 + int(ch.Key-tcell.KeyF1)
	case tcell.KeyEnter, tcell.KeyInsert, tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		return 30
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight,
		tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		return 90
	}
	if ch.Key == tcell.KeyRune {
		return 50
	}
	return 60
}

// MenuBindingLabel returns one key hint (DisplayKey form) for pulldown menus: the PreferredKey from the action
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
	return DisplayKey(menuBindingLabelPick(actionID, spec, hasSpec, keys))
}

// MenuBindingLabelPreferOverlay resolves a menu hint using the overlay's chords when it
// binds the action, otherwise falls back to the global map. A nil overlay means "global
// only" (avoids duplicating global defaults via the overlay map).
func MenuBindingLabelPreferOverlay(global, overlay *Map, actionID string) string {
	spec, hasSpec := SpecForAction(actionID)
	if overlay != nil {
		if ks := overlay.BindingsForAction(actionID); len(ks) > 0 {
			return DisplayKey(menuBindingLabelPick(actionID, spec, hasSpec, ks))
		}
	}
	if global != nil {
		return global.MenuBindingLabel(actionID)
	}
	return ""
}

// FormatChord returns the display form of a parsed chord, in the same notation
// keybindings.toml uses: "C-" (Ctrl), "M-" (Alt) and "S-" (Shift) prefixes in that
// order, then the key. Letters keep their case, so "M-I" is Alt+Shift+I while "C-f"
// and "M-i" need no Shift. Every help, menu and footer key hint goes through it.
// Examples: "C-d", "M-←", "S-Tab", "F5", "Enter", "C-M-d".
func FormatChord(ch Chord) string {
	var b strings.Builder
	// Ctrl may be encoded in the key type (KeyCtrlA..KeyCtrlZ, KeyCtrlSpace) after
	// CanonicalChord clears the ModCtrl bit.
	isCtrlKey := ch.Key == tcell.KeyCtrlSpace || (ch.Key >= tcell.KeyCtrlA && ch.Key <= tcell.KeyCtrlZ)
	if ch.Mod&tcell.ModCtrl != 0 || isCtrlKey {
		b.WriteString("C-")
	}
	if ch.Mod&tcell.ModAlt != 0 {
		b.WriteString("M-")
	}
	if ch.Mod&tcell.ModShift != 0 {
		b.WriteString("S-")
	}
	kn := chordKeyName(ch, isCtrlKey)
	if kn == "" {
		return "?"
	}
	b.WriteString(kn)
	return b.String()
}

// DisplayEvent formats a live key press the way FormatChord formats a binding. A rune already
// carries its Shift, so the modifier is dropped as in Map.Lookup.
func DisplayEvent(ev *tcell.EventKey) string {
	ch := CanonicalChord(EventChord(ev))
	if ch.Key == tcell.KeyRune {
		ch.Mod &^= tcell.ModShift
	}
	return FormatChord(ch)
}

// DisplayKey converts a keybindings.toml key string to its FormatChord display form.
// Strings that do not parse are returned unchanged.
func DisplayKey(s string) string {
	ch, err := ParseKey(s)
	if err != nil {
		return s
	}
	return FormatChord(ch)
}

// chordKeyName returns the display name for a chord's key part.
// isCtrlKey is true when the chord is a KeyCtrl* type — the function
// returns the bare lowercase letter in that case (the "C-" prefix is added by FormatChord).
func chordKeyName(ch Chord, isCtrlKey bool) string {
	if ch.Key == tcell.KeyRune {
		r := ch.Rune
		if r == ' ' {
			return "Space"
		}
		return string(r)
	}
	if ch.Key == tcell.KeyCtrlSpace {
		return "Space"
	}
	if isCtrlKey {
		return string('a' + rune(ch.Key-tcell.KeyCtrlA))
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
		return "S-Tab"
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
