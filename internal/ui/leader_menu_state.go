package ui

// LeaderMenuItem is one row in the bottom leader menu (group title or action shortcut).
type LeaderMenuItem struct {
	GroupTitle  string // non-empty = group header row (built-in Esc menu)
	GroupColumn int    // macro column 0..2 for grouped menu; ignored for F2 user menu
	Key         rune   // pinned activation letter, or 0 for auto-assignment
	Label       string
	DirectKey   string // compact global binding, e.g. "C-s"; built-in Esc menu only
}

// LeaderMenuState is the renderable bottom function menu (Esc built-in / F2 user menu / `"` copy menu).
type LeaderMenuState struct {
	Open     bool
	UserMenu bool // true = F2/menu.toml
	CopyMenu bool // true = `"` copy menu
	Items    []LeaderMenuItem
}
