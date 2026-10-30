package ui

// LeaderMenuItem is one row in the bottom leader menu (group title or action shortcut).
type LeaderMenuItem struct {
	GroupTitle  string // non-empty = group header row (built-in Esc menu)
	GroupColumn int    // macro column 0..2 for grouped menu; ignored for F2 user menu
	Key         rune   // pinned activation letter, or 0 for auto-assignment
	Label       string
	Disabled    bool   // drawn dimmed (still activatable)
	DirectKey   string // compact global binding, e.g. "C-s"; built-in Esc menu only
}

// LeaderMenuKind says which menu the leader-menu strip is showing.
type LeaderMenuKind int

const (
	LeaderMenuBuiltin     LeaderMenuKind = iota // app.leader-menu, per-view and find menus
	LeaderMenuUser                              // app.user-menu (menu.toml)
	LeaderMenuCopy                              // app.copy-menu
	LeaderMenuPreview                           // preview.menu (ui.ViewFilePreview only)
	LeaderMenuGitFilter                         // panel.git-filter-menu
	LeaderMenuFolderColor                       // panel.folder-color-menu
)

// LeaderMenuState is the renderable bottom function menu strip.
type LeaderMenuState struct {
	Open    bool
	Kind    LeaderMenuKind
	CopyGit bool // with LeaderMenuCopy: the "Relative to git root" submenu
	Items   []LeaderMenuItem
}
