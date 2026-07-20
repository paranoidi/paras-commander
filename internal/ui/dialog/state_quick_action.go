package dialog

// QuickActionItem is one row of a QuickActionState list.
type QuickActionItem struct {
	Key   rune // pinned activation letter; 0 = auto-assign from Label
	Label string
}

// QuickActionState is a buttonless, letter-activated list modal: Up/Down move the
// selection bar, Enter activates it, a plain letter key jumps straight to and
// activates the matching row, Esc closes. No OK/Cancel buttons or Alt mnemonics.
type QuickActionState struct {
	Open             bool
	Title            string // "" = untitled frame
	Items            []QuickActionItem
	Selected         int
	ScrollOffset     int
	AnchorX, AnchorY int
	Anchored         bool // false = centered
}
